/*
     Copyright 2024 Felix Weinmann

     Licensed under the Apache License, Version 2.0 (the "License");
     you may not use this file except in compliance with the License.
     You may obtain a copy of the License at

       http://www.apache.org/licenses/LICENSE-2.0

     Unless required by applicable law or agreed to in writing, software
     distributed under the License is distributed on an "AS IS" BASIS,
     WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
     See the License for the specific language governing permissions and
     limitations under the License.
 */
#ifdef SPINNAKER_AVAILABLE

#include "spinnakerdriver.h"

#include <chrono>
#include <thread>

#define CATCH_SPINNAKER(f) try { f; } catch (Spinnaker::Exception &e) { WARN("Could not set parameter: " << e.GetFullErrorMessage()); }


class SpinnakerImage : public RawImage {
public:
	SpinnakerImage(SpinnakerDriver& source, const Spinnaker::ImagePtr& pImage): RawImage(*source.borrow(pImage)), source(source), pImage(pImage) {
		timestamp = (double)pImage->GetTimeStamp() / 1e9;
	}

	~SpinnakerImage() override {
		// borrow() already releases images it had to copy out of a non-pooled buffer
		try {
			if(pImage->IsInUse())
				pImage->Release();
		} catch (Spinnaker::Exception &e) {
			WARN("Could not release image: " << e.GetFullErrorMessage());
		}
		source.restore(*this);
	}

private:
	SpinnakerDriver& source;
	const Spinnaker::ImagePtr pImage;
};

SpinnakerDriver::SpinnakerDriver(const CameraConfig& config) {
	pSystem = Spinnaker::System::GetInstance();

	while(true) {
		Spinnaker::CameraList camList = pSystem->GetCameras();
		const unsigned int camCount = camList.GetSize();
		// Enumeration order is not stable (especially across GigE interfaces), prefer the serial number
		if(!config.serial.empty())
			pCam = camList.GetBySerial(config.serial);
		else if(config.hardwareId >= 0 && camCount > (unsigned int)config.hardwareId)
			pCam = camList.GetByIndex(config.hardwareId);
		camList.Clear();

		if(pCam.IsValid()) {
			pCam->Init();
			initTime = getRealTime();
			VP_LOG("Opened " << pCam->DeviceModelName.GetValue() << " - " << pCam->DeviceSerialNumber.GetValue().c_str());
			break;
		}

		if(!config.serial.empty())
			WARN("Waiting for cam with serial " << config.serial);
		else
			WARN("Waiting for cam: " << camCount << "/" << (config.hardwareId+1));

		sleep(1);
	}

	loadDefaultUserSet();

	CATCH_SPINNAKER(pCam->TriggerMode.SetValue(Spinnaker::TriggerMode_Off))
	CATCH_SPINNAKER(pCam->AcquisitionMode.SetValue(Spinnaker::AcquisitionMode_Continuous))
	selectPixelFormat();
	// Free running at the maximum frame rate
	setBool(SpinnakerFeatures::FrameRateControl, false);

	pCam->TLStream.StreamBufferHandlingMode.SetValue(Spinnaker::StreamBufferHandlingMode_NewestOnly);
	pCam->TLStream.StreamBufferCountManual.SetValue(pCam->TLStream.StreamBufferCountManual.GetMin());

	configurePacketSize();
}

void SpinnakerDriver::reportOnce(const std::string& message) {
	if(reported.insert(message).second)
		WARN(message);
}

Spinnaker::GenApi::INode* SpinnakerDriver::findNode(const SpinnakerFeature& feature) {
	Spinnaker::GenApi::INodeMap& nodeMap = pCam->GetNodeMap();

	Spinnaker::GenApi::CNodePtr unavailable = nullptr;
	for(const char* name : feature.nodes) {
		Spinnaker::GenApi::CNodePtr node = nodeMap.GetNode(name);
		if(SpinnakerFeatures::isAvailable(node))
			return node;
		if(node.IsValid() && !unavailable.IsValid())
			unavailable = node;
	}

	if(unavailable.IsValid()) {
		reportOnce(std::string("Camera feature ") + feature.label + " (" + unavailable->GetName().c_str() + ") is not available in the current camera mode");
	} else {
		std::string list;
		for(const char* name : feature.nodes)
			list += (list.empty() ? "" : ", ") + std::string(name);
		reportOnce(std::string("Camera does not support ") + feature.label + " (" + list + ")");
	}
	return nullptr;
}

bool SpinnakerDriver::setBool(const SpinnakerFeature& feature, bool value) {
	Spinnaker::GenApi::CBooleanPtr node = findNode(feature);
	if(!node.IsValid())
		return false;
	if(!IsWritable(node)) {
		reportOnce(std::string("Camera feature ") + feature.label + " is not writable");
		return false;
	}

	try {
		node->SetValue(value);
		return true;
	} catch (Spinnaker::Exception &e) {
		WARN("Could not set " << feature.label << ": " << e.GetFullErrorMessage());
		return false;
	}
}

bool SpinnakerDriver::setFloat(const SpinnakerFeature& feature, double value) {
	Spinnaker::GenApi::CFloatPtr node = findNode(feature);
	if(!node.IsValid())
		return false;
	if(!IsWritable(node)) {
		reportOnce(std::string("Camera feature ") + feature.label + " is not writable");
		return false;
	}

	try {
		node->SetValue(value);
		return true;
	} catch (Spinnaker::Exception &e) {
		WARN("Could not set " << feature.label << ": " << e.GetFullErrorMessage());
		return false;
	}
}

bool SpinnakerDriver::setEnum(const SpinnakerFeature& feature, const char* entry) {
	Spinnaker::GenApi::CEnumerationPtr node = findNode(feature);
	if(!node.IsValid())
		return false;
	if(!IsWritable(node)) {
		reportOnce(std::string("Camera feature ") + feature.label + " is not writable");
		return false;
	}

	Spinnaker::GenApi::CEnumEntryPtr entryNode = node->GetEntryByName(entry);
	if(!IsAvailable(entryNode)) {
		reportOnce(std::string("Camera feature ") + feature.label + " does not offer " + entry);
		return false;
	}

	try {
		node->SetIntValue(entryNode->GetValue());
		return true;
	} catch (Spinnaker::Exception &e) {
		WARN("Could not set " << feature.label << ": " << e.GetFullErrorMessage());
		return false;
	}
}

std::string SpinnakerDriver::getEnum(const SpinnakerFeature& feature) {
	Spinnaker::GenApi::CEnumerationPtr node = findNode(feature);
	if(!IsReadable(node))
		return "";

	try {
		return node->GetCurrentEntry()->GetSymbolic().c_str();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not read " << feature.label << ": " << e.GetFullErrorMessage());
		return "";
	}
}

void SpinnakerDriver::selectPixelFormat() {
	static const std::pair<std::string, const PixelFormat*> bayerFormats[] = {
			{"BayerRG", &PixelFormat::RGGB8},
			{"BayerBG", &PixelFormat::BGGR8},
			{"BayerGR", &PixelFormat::GRBG8},
	};

	// A color sensor offers the 8 bit Bayer format matching its color filter
	const std::string entry = SpinnakerFeatures::findBayerFormat(pCam->GetNodeMap());
	if(!entry.empty() && setEnum(SpinnakerFeatures::PixelFormatNode, entry.c_str())) {
		// Raw8 cameras report the Bayer pattern separately, "BayerRG8" style entries name it
		const std::string pattern = entry == "Raw8" ? getEnum(SpinnakerFeatures::PixelColorFilter) : entry.substr(0, entry.size() - 1);
		for(const auto& [name, format] : bayerFormats) {
			if(pattern == name) {
				pixelFormat = format;
				VP_LOG("Pixel format: " << entry << " (" << pattern << ")");
				return;
			}
		}
	}

	std::string offered;
	Spinnaker::GenApi::CEnumerationPtr formatNode = pCam->GetNodeMap().GetNode("PixelFormat");
	if(IsReadable(formatNode)) {
		Spinnaker::GenApi::NodeList_t entries;
		formatNode->GetEntries(entries);
		for(Spinnaker::GenApi::CEnumEntryPtr formatEntry : entries) {
			if(IsAvailable(formatEntry))
				offered += std::string(offered.empty() ? "" : ", ") + formatEntry->GetSymbolic().c_str();
		}
	}
	FATAL("Camera offers no supported Bayer pixel format (BayerRG8, BayerBG8, BayerGR8 or Raw8 with one of these patterns), available: " << offered);
}

/** Camera access and streaming state, for diagnosing failed commands. Each read may itself fail while the camera is unresponsive. */
static std::string describeAccessState(const Spinnaker::CameraPtr& cam) {
	std::string state;
	auto add = [&](Spinnaker::GenApi::INodeMap& nodeMap, const char* map, const char* name) {
		std::string value;
		try {
			Spinnaker::GenApi::CValuePtr node = nodeMap.GetNode(name);
			value = Spinnaker::GenApi::IsReadable(node) ? node->ToString().c_str() : "n/a";
		} catch (Spinnaker::Exception &e) {
			value = "read failed";
		}
		state += std::string(state.empty() ? "" : ", ") + map + name + "=" + value;
	};

	add(cam->GetNodeMap(), "", "TLParamsLocked");
	add(cam->GetNodeMap(), "", "GevCCP");
	add(cam->GetTLDeviceNodeMap(), "TL ", "DeviceAccessStatus");
	
	return state;
}

void SpinnakerDriver::loadDefaultUserSet() {
	// Reset camera parameters.
	//
	// Only UserSetLoad changes the active settings; UserSetDefault just
	// selects the power-on set and persists that choice on the camera.
    //
	// First generation GigE cameras sometimes fail the first load after opening with an IO error and
	// stay unresponsive briefly. Loading the factory set again is harmless, so retry after a short wait;
	// the selector write at the start of each attempt also waits for the camera to respond again.
	const int attempts = 3;
	const double start = getRealTime();
	for(int attempt = 1; attempt <= attempts; attempt++) {
		try {
			pCam->UserSetSelector.SetValue(Spinnaker::UserSetSelector_Default);
			pCam->UserSetLoad.Execute();
			if(attempt > 1)
				VP_LOG("Loaded default user set on attempt " << attempt << " after " << (getRealTime() - start) * 1000.0 << " ms");
			return;
		} catch (Spinnaker::Exception &e) {
			WARN("Could not load default user set (attempt " << attempt << "/" << attempts << ", " << (getRealTime() - start) * 1000.0 << " ms, "
					<< (getRealTime() - initTime) * 1000.0 << " ms after Init; " << describeAccessState(pCam) << "): " << e.GetFullErrorMessage());
		}

		if(attempt < attempts)
			std::this_thread::sleep_for(std::chrono::milliseconds(250 * attempt));
	}

	WARN("Camera keeps its previous settings, except those configured explicitly");
}

void SpinnakerDriver::configurePacketSize() {
	if(!IsWritable(pCam->GevSCPSPacketSize))
		return; // Not a GigE camera

	try {
		// The largest packet size the whole path (NIC MTU, switches) supports; 9000 only works with jumbo frames everywhere
		int64_t packetSize = pCam->DiscoverMaxPacketSize();
		const int64_t min = pCam->GevSCPSPacketSize.GetMin();
		const int64_t inc = pCam->GevSCPSPacketSize.GetInc();
		packetSize = std::min(packetSize, pCam->GevSCPSPacketSize.GetMax());
		packetSize -= (packetSize - min) % inc;
		pCam->GevSCPSPacketSize.SetValue(packetSize);
		VP_LOG("GigE packet size: " << packetSize);
	} catch (Spinnaker::Exception &e) {
		WARN("Could not configure GigE packet size: " << e.GetFullErrorMessage());
	}
}

void SpinnakerDriver::allocateBuffers() {
	buffers.clear();

	// Provide image buffers to achieve faster mapping with OpenCL
	const int width = (int)pCam->Width.GetValue();
	const int height = (int)pCam->Height.GetValue();
	// Spinnaker buffer sizes need to be 1024 byte aligned
	const size_t bufferSize = ((size_t)width * height + 1023)/1024 * 1024;
	for(int64_t i = 0; i < pCam->TLStream.StreamBufferCountManual.GetValue(); i++) {
		std::shared_ptr<RawImage> buffer = std::make_shared<RawImage>(CLArray(bufferSize), pixelFormat, width/2, height/2, "spinnaker");
		buffers[buffer] = std::make_unique<CLMap<uint8_t>>(buffer->write<uint8_t>());
	}

	bufferPtrs.clear();
	for (auto& item: buffers)
		bufferPtrs.push_back(**item.second);

	pCam->SetBufferOwnership(Spinnaker::SPINNAKER_BUFFER_OWNERSHIP_USER);
	pCam->SetUserBuffers(bufferPtrs.data(), buffers.size(), bufferSize);
}

bool SpinnakerDriver::buffersIdle() const {
	for (auto& item : buffers) {
		if(item.second == nullptr)
			return false;
	}
	return true;
}

void SpinnakerDriver::startAcquisition() {
	// Buffers are sized for the current resolution, so they are (re)created on every start
	allocateBuffers();
	pCam->BeginAcquisition();
	acquiring = true;
	refreshFrametime();
}

void SpinnakerDriver::stopAcquisition() {
	pCam->EndAcquisition();
	acquiring = false;
}

void SpinnakerDriver::refreshFrametime() {
	frametimeUpdated = getRealTime();

	// First generation cameras (e.g. Blackfly GigE) lack AcquisitionResultingFrameRate
	try {
		if(IsReadable(pCam->AcquisitionResultingFrameRate)) {
			frametime = 1 / pCam->AcquisitionResultingFrameRate.GetValue();
			return;
		}

		if(IsReadable(pCam->AcquisitionFrameRate)) {
			frametime = 1 / pCam->AcquisitionFrameRate.GetValue();
			return;
		}
	} catch (Spinnaker::Exception &e) {
		WARN("Could not read frame rate: " << e.GetFullErrorMessage());
	}

	if(frametime == 0.0)
		WARN("Camera reports no frame rate, frame time overruns will not be detected");
	frametime = INFINITY;
}

std::shared_ptr<RawImage> SpinnakerDriver::readImage() {
	if(!acquiring)
		startAcquisition();

	// The resulting frame rate changes with (auto) exposure, so poll it occasionally
	if(getRealTime() - frametimeUpdated > 1.0)
		refreshFrametime();

	return std::make_shared<SpinnakerImage>(*this, pCam->GetNextImage());
}

const PixelFormat SpinnakerDriver::format() {
	return *pixelFormat;
}

double SpinnakerDriver::expectedFrametime() {
	return frametime;
}

SpinnakerDriver::~SpinnakerDriver() {
	// Exceptions must not escape a destructor. DeInit also releases the GigE control channel,
	// otherwise the camera stays claimed until its heartbeat times out.
	try {
		if(acquiring)
			stopAcquisition();
		pCam->DeInit();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not close camera: " << e.GetFullErrorMessage());
	}

	pCam = nullptr;
	try {
		pSystem->ReleaseInstance();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not release Spinnaker system: " << e.GetFullErrorMessage());
	}
}

std::shared_ptr<RawImage> SpinnakerDriver::borrow(const Spinnaker::ImagePtr& pImage) {
	void* data = pImage->GetData();
	for (auto& item : buffers) {
		if(item.second != nullptr && **item.second == data) {
			item.second = nullptr;
			return item.first;
		}
	}

	WARN("Did not get image with given buffer, creating new buffer; expect OpenCL performance degradation");
	std::shared_ptr<RawImage> image = std::make_shared<RawImage>(pixelFormat, (int)pImage->GetWidth() / 2, (int)pImage->GetHeight() / 2, (unsigned char*)pImage->GetData());
	pImage->Release();
	return image;
}

void SpinnakerDriver::restore(const RawImage& image) {
	for (auto& item : buffers) {
		if(item.first->buffer == image.buffer) {
			item.second = std::make_unique<CLMap<uint8_t>>(item.first->write<uint8_t>());
			return;
		}
	}
}

uint32_t SpinnakerDriver::getWidth() {
    return pCam->Width.GetValue();
}

uint32_t SpinnakerDriver::getHeight() {
    return pCam->Height.GetValue();
}

void SpinnakerDriver::setResolution(uint32_t width, uint32_t height) {
	if(width == 0 || height == 0) {
		width = pCam->WidthMax.GetValue();
		height = pCam->HeightMax.GetValue();
	}

	if(width == getWidth() && height == getHeight())
		return;

	// The camera locks the image size while streaming and the buffers are sized for it
	const bool restart = acquiring;
	if(restart) {
		if(!buffersIdle()) {
			WARN("Cannot change resolution while images are in use");
			return;
		}
		stopAcquisition();
	}

	CATCH_SPINNAKER(pCam->Width.SetValue(width));
	CATCH_SPINNAKER(pCam->Height.SetValue(height));

	if(restart)
		startAcquisition();
}

float SpinnakerDriver::getExposure() {
    return pCam->ExposureTime.GetValue() / 1000.0f;
}

void SpinnakerDriver::setExposure(float exposure) {
	if(exposure == 0.0f) {
		CATCH_SPINNAKER(pCam->ExposureAuto.SetValue(Spinnaker::ExposureAuto_Continuous));
		setEnum(SpinnakerFeatures::AutoExposureMetering, "Average");

		if(pCam->GainAuto.GetValue() == Spinnaker::GainAuto_Continuous) {
			setEnum(SpinnakerFeatures::AutoExposurePriority, "Gain");
		}
	} else {
		CATCH_SPINNAKER(pCam->ExposureAuto.SetValue(Spinnaker::ExposureAuto_Off))
		CATCH_SPINNAKER(pCam->ExposureTime.SetValue(exposure * 1000.0f))
	}

	// Exposure can limit the frame rate
	frametimeUpdated = 0.0;
}

float SpinnakerDriver::getGain() {
    return pCam->Gain.GetValue();
}

void SpinnakerDriver::setGain(float gain) {
	if(gain == 0.0f) {
		CATCH_SPINNAKER(pCam->GainAuto.SetValue(Spinnaker::GainAuto_Continuous));

		if(pCam->ExposureAuto.GetValue() == Spinnaker::ExposureAuto_Continuous) {
			setEnum(SpinnakerFeatures::AutoExposurePriority, "Gain");
		}
	} else {
		CATCH_SPINNAKER(pCam->GainAuto.SetValue(Spinnaker::GainAuto_Off));
		CATCH_SPINNAKER(pCam->Gain.SetValue(gain));
	}
}

float SpinnakerDriver::getGamma() {
	// First generation cameras apply no gamma to Raw/Bayer formats, the node is unavailable then
	Spinnaker::GenApi::CFloatPtr gamma = pCam->GetNodeMap().GetNode("Gamma");
	return IsReadable(gamma) ? (float)gamma->GetValue() : 1.0f;
}

void SpinnakerDriver::setGamma(float gamma) {
	if(gamma == 1.0f) {
		setBool(SpinnakerFeatures::GammaEnable, false);
	} else if(setBool(SpinnakerFeatures::GammaEnable, true)) {
		setFloat(SpinnakerFeatures::Gamma, gamma);
	}
}

WhiteBalanceType SpinnakerDriver::getWhiteBalanceType() {
	if(pCam->BalanceWhiteAuto.GetValue() == Spinnaker::BalanceWhiteAuto_Off) {
		return WhiteBalanceType::WhiteBalanceType_Manual;
	} else {
		const std::string profile = getEnum(SpinnakerFeatures::WhiteBalanceProfile);
		if(profile == "Outdoor")
			return WhiteBalanceType::WhiteBalanceType_AutoOutdoor;
		if(profile == "Indoor")
			return WhiteBalanceType::WhiteBalanceType_AutoIndoor;
		return whiteBalanceType;
	}
}

float SpinnakerDriver::getWhiteBalanceBlue() {
	CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Blue))
	return pCam->BalanceRatio.GetValue();
}

float SpinnakerDriver::getWhiteBalanceRed() {
    CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Red))
	return pCam->BalanceRatio.GetValue();
}

void SpinnakerDriver::setWhiteBalance(WhiteBalanceType type, float blue, float red) {
	if(type != WhiteBalanceType_Manual) {
		whiteBalanceType = type;
		CATCH_SPINNAKER(pCam->BalanceWhiteAuto.SetValue(Spinnaker::BalanceWhiteAuto_Continuous));
		setEnum(SpinnakerFeatures::WhiteBalanceProfile, type == WhiteBalanceType_AutoOutdoor ? "Outdoor" : "Indoor");
	} else {
		CATCH_SPINNAKER(pCam->BalanceWhiteAuto.SetValue(Spinnaker::BalanceWhiteAuto_Off));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Blue));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(blue));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Red));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(red));
	}
}

#endif

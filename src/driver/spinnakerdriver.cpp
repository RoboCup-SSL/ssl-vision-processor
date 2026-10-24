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

#include <algorithm>
#include <chrono>
#include <thread>

using namespace std::chrono_literals;

#define CATCH_SPINNAKER(f) try { f; } catch (Spinnaker::Exception &e) { WARN("Camera call failed: " #f ": " << e.GetErrorMessage()); }

/** Sets a feature the camera may lack (first generation cameras) or not allow in its current mode, warning once if so. */
template<typename Node, typename Value>
static bool setOptional(Node& node, const Value& value, const char* feature) {
	if(!IsWritable(node)) {
		WARN_ONCE("Camera does not support " << feature << (IsImplemented(node) ? " in its current mode" : ""));
		return false;
	}

	try {
		node.SetValue(value);
		return true;
	} catch (Spinnaker::Exception &e) {
		WARN("Could not set " << feature << ": " << e.GetErrorMessage());
		return false;
	}
}

static std::string join(const std::vector<std::string>& items) {
	std::string joined;
	for(const std::string& item : items)
		joined += (joined.empty() ? "" : ", ") + item;
	return joined;
}

/** The index-th camera by ascending serial number, as the enumeration order itself is not guaranteed (e.g. across GigE interfaces). */
static Spinnaker::CameraPtr getBySerialOrder(const Spinnaker::CameraList& camList, int index) {
	std::vector<std::string> serials;
	for(unsigned int i = 0; i < camList.GetSize(); i++)
		serials.emplace_back(camList.GetByIndex(i)->TLDevice.DeviceSerialNumber.GetValue().c_str());

	// Numeric order for numeric serials: shorter first, then alphabetical
	std::ranges::sort(serials, {}, [](const std::string& serial) { return std::pair(serial.size(), serial); });
	if(index < 0 || static_cast<size_t>(index) >= serials.size())
		return Spinnaker::CameraPtr();
	return camList.GetBySerial(serials[static_cast<size_t>(index)]);
}

/** Camera access and streaming state, for diagnosing failed commands. Each read may itself fail while the camera is unresponsive. */
static std::string describeAccessState(const Spinnaker::CameraPtr& cam) {
	std::vector<std::string> state;
	auto add = [&](Spinnaker::GenApi::INodeMap& nodeMap, const char* map, const char* name) {
		std::string value;
		try {
			Spinnaker::GenApi::CValuePtr node = nodeMap.GetNode(name);
			value = Spinnaker::GenApi::IsReadable(node) ? node->ToString().c_str() : "n/a";
		} catch (Spinnaker::Exception &e) {
			value = "read failed";
		}
		state.push_back(std::string(map) + name + "=" + value);
	};

	add(cam->GetNodeMap(), "", "TLParamsLocked");
	add(cam->GetNodeMap(), "", "GevCCP");
	add(cam->GetTLDeviceNodeMap(), "TL ", "DeviceAccessStatus");
	return join(state);
}


class SpinnakerImage : public RawImage {
public:
	SpinnakerImage(SpinnakerDriver& source, const Spinnaker::ImagePtr& pImage): RawImage(*source.borrow(pImage)), source(source), pImage(pImage) {
		timestamp = static_cast<double>(pImage->GetTimeStamp()) / 1e9;
	}

	~SpinnakerImage() override {
		// borrow() already releases images it had to copy out of a non-pooled buffer
		try {
			if(pImage->IsInUse())
				pImage->Release();
		} catch (Spinnaker::Exception &e) {
			WARN("Could not release image: " << e.GetErrorMessage());
		}
		source.restore(*this);
	}

private:
	SpinnakerDriver& source;
	const Spinnaker::ImagePtr pImage;
};

// Function try block: unexpected camera errors during setup exit with a clear message
SpinnakerDriver::SpinnakerDriver(const CameraConfig& config) try {
	pSystem = Spinnaker::System::GetInstance();

	const std::string camera = config.device.empty() ? "id " + std::to_string(config.hardwareId) : "with serial " + config.device;
	while(true) {
		Spinnaker::CameraList camList = pSystem->GetCameras();
		pCam = config.device.empty() ? getBySerialOrder(camList, config.hardwareId) : camList.GetBySerial(config.device);
		if (!pCam.IsValid()) {
			WARN("Waiting for camera " << camera << ", found " << camList.GetSize() << " camera(s)");
		} else {
			try {
				pCam->Init();
				initTime = getRealTime();
				VP_LOG("Opened " << pCam->DeviceModelName.GetValue() << " - " << pCam->DeviceSerialNumber.GetValue().c_str());
				camList.Clear();
				break;
			} catch (Spinnaker::Exception &e) {
				// E.g. opened by another program, or by a crashed one until its GigE heartbeat times out (seconds)
				WARN("Could not open camera " << camera << ", retrying: " << e.GetErrorMessage());
				pCam = nullptr;
			}
		}

		camList.Clear();
		std::this_thread::sleep_for(1s);
	}

	loadDefaultUserSet();

	CATCH_SPINNAKER(pCam->TriggerMode.SetValue(Spinnaker::TriggerMode_Off))
	CATCH_SPINNAKER(pCam->AcquisitionMode.SetValue(Spinnaker::AcquisitionMode_Continuous))
	selectPixelFormat();
	// Free running, so auto exposure can change the frame rate (see readImage)
	setOptional(pCam->AcquisitionFrameRateEnable, false, "frame rate control");

	pCam->TLStream.StreamBufferHandlingMode.SetValue(Spinnaker::StreamBufferHandlingMode_NewestOnly);
	pCam->TLStream.StreamBufferCountManual.SetValue(pCam->TLStream.StreamBufferCountManual.GetMin());

	// Provide image buffers to achieve faster mapping with OpenCL. Allocated once for the maximum
	// resolution, borrow() wraps each image with its actual size.
	int width = pCam->WidthMax.GetValue();
	int height = pCam->HeightMax.GetValue();
	// Spinnaker buffer sizes need to be 1024 byte aligned
	const int bufferSize = (width * height + 1023)/1024 * 1024;
	for(int i = 0; i < pCam->TLStream.StreamBufferCountManual.GetMin(); i++) {
		std::shared_ptr<RawImage> buffer = std::make_shared<RawImage>(CLArray(bufferSize), pixelFormat, width/2, height/2, "spinnaker");
		buffers[buffer] = std::make_unique<CLMap<uint8_t>>(buffer->write<uint8_t>());
	}

	for (auto& item: buffers)
		bufferPtrs.push_back(**item.second);

	pCam->SetBufferOwnership(Spinnaker::SPINNAKER_BUFFER_OWNERSHIP_USER);
	pCam->SetUserBuffers(bufferPtrs.data(), buffers.size(), static_cast<uint64_t>(bufferSize));
	//pCam->Timestamp.SetValue();

	configurePacketSize();

	pCam->BeginAcquisition();
	refreshFrametime();
} catch (Spinnaker::Exception &e) {
	FATAL("Could not set up the camera: " << e.GetErrorMessage());
}

void SpinnakerDriver::loadDefaultUserSet() {
	// Reset camera parameters. Only UserSetLoad changes the active settings (UserSetDefault just picks the
	// persisted power-on set). The load sometimes fails with an IO error right after opening a GigE camera;
	// loading the factory set again is harmless, so retry.
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
			std::this_thread::sleep_for(250ms * attempt);
	}

	WARN("Camera keeps its previous settings, except those configured explicitly");
}

void SpinnakerDriver::selectPixelFormat() {
	// Kernel formats in the order of SpinnakerFeatures::BayerPatterns
	static constexpr auto bayerFormats = std::to_array({&PixelFormat::RGGB8, &PixelFormat::BGGR8, &PixelFormat::GRBG8});
	static_assert(bayerFormats.size() == SpinnakerFeatures::BayerPatterns.size());
	const auto& patterns = SpinnakerFeatures::BayerPatterns;

	// A color sensor offers the 8 bit Bayer format matching its color filter
	const std::string entry = SpinnakerFeatures::findBayerFormat(pCam->GetNodeMap());
	if(!entry.empty()) {
		try {
			pCam->PixelFormat.FromString(entry.c_str());
		} catch (Spinnaker::Exception &e) {
			FATAL("Could not select pixel format " << entry << ": " << e.GetErrorMessage());
		}
		// Raw8 cameras report the Bayer pattern separately, "BayerRG8" style entries name it
		const std::string pattern = entry != "Raw8" ? entry.substr(0, entry.size() - 1)
				: IsReadable(pCam->PixelColorFilter) ? pCam->PixelColorFilter.ToString().c_str() : "";
		const auto match = std::find(patterns.begin(), patterns.end(), pattern);
		if(match != patterns.end()) {
			pixelFormat = bayerFormats[static_cast<size_t>(match - patterns.begin())];
			VP_LOG("Pixel format: " << entry << " (" << pattern << ")");
			return;
		}
	}

	std::vector<std::string> supported;
	for(std::string_view pattern : patterns)
		supported.push_back(std::string(pattern) + "8");
	Spinnaker::GenApi::StringList_t symbolics;
	pCam->PixelFormat.GetSymbolics(symbolics);
	std::vector<std::string> offered;
	for(const auto& symbolic : symbolics)
		offered.emplace_back(symbolic.c_str());
	FATAL("Camera offers no supported Bayer pixel format (" << join(supported) << ", or Raw8 with one of these patterns), available: " << join(offered));
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
		WARN("Could not configure GigE packet size: " << e.GetErrorMessage());
	}
}

void SpinnakerDriver::refreshFrametime() {
	frametimeUpdated = getRealTime();

	// First generation cameras (e.g. Blackfly GigE) lack AcquisitionResultingFrameRate
	double frameRate = 0.0;
	try {
		if(IsReadable(pCam->AcquisitionResultingFrameRate))
			frameRate = pCam->AcquisitionResultingFrameRate.GetValue();
		else if(IsReadable(pCam->AcquisitionFrameRate))
			frameRate = pCam->AcquisitionFrameRate.GetValue();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not read frame rate: " << e.GetErrorMessage());
	}

	if(frameRate <= 0.0) {
		WARN_ONCE("Camera reports no frame rate, assuming 30 fps for frame time overrun detection");
		frameRate = 30.0;
	}
	frametime = 1 / frameRate;
}

std::shared_ptr<RawImage> SpinnakerDriver::readImage() {
	// With frame rate control off, auto exposure changes the frame rate, so poll it occasionally
	if(autoExposure && getRealTime() - frametimeUpdated > 1.0)
		refreshFrametime();

	try {
		Spinnaker::ImagePtr pImage = pCam->GetNextImage();
		// Incomplete images (e.g. lost GigE packets) contain partly stale data
		while(pImage->IsIncomplete()) {
			if(droppedImages++ % 100 == 0)
				WARN("Dropped " << droppedImages << " incomplete camera image(s) so far (" << Spinnaker::Image::GetImageStatusDescription(pImage->GetImageStatus())
						<< "); on GigE this usually means lost network packets");
			pImage->Release();
			pImage = pCam->GetNextImage();
		}
		return std::make_shared<SpinnakerImage>(*this, pImage);
	} catch (Spinnaker::Exception &e) {
		FATAL("Camera stopped delivering images: " << e.GetErrorMessage());
	}
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
		if(pCam->IsStreaming())
			pCam->EndAcquisition();
		pCam->DeInit();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not close camera: " << e.GetErrorMessage());
	}

	pCam = nullptr;
	try {
		pSystem->ReleaseInstance();
	} catch (Spinnaker::Exception &e) {
		WARN("Could not release Spinnaker system: " << e.GetErrorMessage());
	}
}

std::shared_ptr<RawImage> SpinnakerDriver::borrow(const Spinnaker::ImagePtr& pImage) {
	void* data = pImage->GetData();
	for (auto& item : buffers) {
		if(item.second != nullptr && **item.second == data) {
			item.second = nullptr;
			// Same memory as the maximum size buffer, with the actual image size
			return std::make_shared<RawImage>(CLArray(*item.first), pixelFormat, static_cast<int>(pImage->GetWidth()) / 2, static_cast<int>(pImage->GetHeight()) / 2, "spinnaker");
		}
	}

	WARN_ONCE("Camera images arrive outside the driver's buffers and are copied, expect lower performance");
	std::shared_ptr<RawImage> image = std::make_shared<RawImage>(pixelFormat, static_cast<int>(pImage->GetWidth()) / 2, static_cast<int>(pImage->GetHeight()) / 2, static_cast<unsigned char*>(pImage->GetData()));
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

	// The camera locks the image size while streaming
	for (auto& item : buffers) {
		if(item.second == nullptr) {
			WARN("Cannot change resolution while images are in use");
			return;
		}
	}

	CATCH_SPINNAKER(pCam->EndAcquisition());
	CATCH_SPINNAKER(pCam->Width.SetValue(width));
	CATCH_SPINNAKER(pCam->Height.SetValue(height));
	try {
		pCam->BeginAcquisition();
	} catch (Spinnaker::Exception &e) {
		FATAL("Could not restart the camera after changing the resolution: " << e.GetErrorMessage());
	}

	if(getWidth() != width || getHeight() != height)
		WARN("Camera uses " << getWidth() << "x" << getHeight() << " instead of the configured " << width << "x" << height);
	refreshFrametime();
}

float SpinnakerDriver::getExposure() {
    return pCam->ExposureTime.GetValue() / 1000.0f;
}

void SpinnakerDriver::setExposure(float exposure) {
	autoExposure = exposure == 0.0f;
	if(autoExposure) {
		CATCH_SPINNAKER(pCam->ExposureAuto.SetValue(Spinnaker::ExposureAuto_Continuous));
		setOptional(pCam->AutoExposureMeteringMode, Spinnaker::AutoExposureMeteringMode_Average, "auto exposure metering");

		if(pCam->GainAuto.GetValue() == Spinnaker::GainAuto_Continuous) {
			setOptional(pCam->AutoExposureControlPriority, Spinnaker::AutoExposureControlPriority_Gain, "auto exposure priority");
		}
	} else {
		CATCH_SPINNAKER(pCam->ExposureAuto.SetValue(Spinnaker::ExposureAuto_Off))
		CATCH_SPINNAKER(pCam->ExposureTime.SetValue(exposure * 1000.0f))
	}

	refreshFrametime();
}

float SpinnakerDriver::getGain() {
    return pCam->Gain.GetValue();
}

void SpinnakerDriver::setGain(float gain) {
	if(gain == 0.0f) {
		CATCH_SPINNAKER(pCam->GainAuto.SetValue(Spinnaker::GainAuto_Continuous));

		if(pCam->ExposureAuto.GetValue() == Spinnaker::ExposureAuto_Continuous) {
			setOptional(pCam->AutoExposureControlPriority, Spinnaker::AutoExposureControlPriority_Gain, "auto exposure priority");
		}
	} else {
		CATCH_SPINNAKER(pCam->GainAuto.SetValue(Spinnaker::GainAuto_Off));
		CATCH_SPINNAKER(pCam->Gain.SetValue(gain));
	}
}

float SpinnakerDriver::getGamma() {
	// First generation cameras apply no gamma to Raw/Bayer formats, the node is unavailable then
	return IsReadable(pCam->Gamma) ? static_cast<float>(pCam->Gamma.GetValue()) : 1.0f;
}

void SpinnakerDriver::setGamma(float gamma) {
	if(gamma == 1.0f) {
		setOptional(pCam->GammaEnable, false, "gamma");
	} else if(setOptional(pCam->GammaEnable, true, "gamma")) {
		setOptional(pCam->Gamma, gamma, "gamma");
	}
}

WhiteBalanceType SpinnakerDriver::getWhiteBalanceType() {
	if(pCam->BalanceWhiteAuto.GetValue() == Spinnaker::BalanceWhiteAuto_Off) {
		return WhiteBalanceType::WhiteBalanceType_Manual;
	} else if(!IsReadable(pCam->BalanceWhiteAutoProfile)) {
		return whiteBalanceType; // First generation cameras have a single auto profile
	} else {
		return (pCam->BalanceWhiteAutoProfile.GetValue() == Spinnaker::BalanceWhiteAutoProfile_Outdoor)
			? WhiteBalanceType::WhiteBalanceType_AutoOutdoor
			: WhiteBalanceType::WhiteBalanceType_AutoIndoor;
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
		setOptional(pCam->BalanceWhiteAutoProfile,
				type == WhiteBalanceType_AutoOutdoor
				? Spinnaker::BalanceWhiteAutoProfile_Outdoor
				: Spinnaker::BalanceWhiteAutoProfile_Indoor,
				"auto white balance profile");
	} else {
		CATCH_SPINNAKER(pCam->BalanceWhiteAuto.SetValue(Spinnaker::BalanceWhiteAuto_Off));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Blue));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(blue));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Red));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(red));
	}
}

#endif

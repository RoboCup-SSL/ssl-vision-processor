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
			VP_LOG("Opened " << pCam->DeviceModelName.GetValue() << " - " << pCam->DeviceSerialNumber.GetValue().c_str());
			break;
		}

		if(!config.serial.empty())
			WARN("Waiting for cam with serial " << config.serial);
		else
			WARN("Waiting for cam: " << camCount << "/" << (config.hardwareId+1));

		sleep(1);
	}

	// Reset camera parameters. Only UserSetLoad changes the active settings; UserSetDefault just
	// selects the power-on set and persists that choice on the camera, so leave it alone.
	CATCH_SPINNAKER(pCam->UserSetSelector.SetValue(Spinnaker::UserSetSelector_Default))
	CATCH_SPINNAKER(pCam->UserSetLoad.Execute())

	CATCH_SPINNAKER(pCam->TriggerMode.SetValue(Spinnaker::TriggerMode_Off))
	CATCH_SPINNAKER(pCam->AcquisitionMode.SetValue(Spinnaker::AcquisitionMode_Continuous))
	CATCH_SPINNAKER(pCam->PixelFormat.SetValue(Spinnaker::PixelFormat_BayerRG8))
	CATCH_SPINNAKER(pCam->AcquisitionFrameRateEnable.SetValue(false))

	pCam->TLStream.StreamBufferHandlingMode.SetValue(Spinnaker::StreamBufferHandlingMode_NewestOnly);
	pCam->TLStream.StreamBufferCountManual.SetValue(pCam->TLStream.StreamBufferCountManual.GetMin());

	configurePacketSize();
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
		std::shared_ptr<RawImage> buffer = std::make_shared<RawImage>(CLArray(bufferSize), &PixelFormat::RGGB8, width/2, height/2, "spinnaker");
		buffers[buffer] = std::make_unique<CLMap<uint8_t>>(buffer->write<uint8_t>());
	}

	std::vector<void*> bufferPtrs;
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
	return PixelFormat::RGGB8;
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
	std::shared_ptr<RawImage> image = std::make_shared<RawImage>(&PixelFormat::RGGB8, (int)pImage->GetWidth() / 2, (int)pImage->GetHeight() / 2, (unsigned char*)pImage->GetData());
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
		CATCH_SPINNAKER(pCam->AutoExposureMeteringMode.SetValue(Spinnaker::AutoExposureMeteringMode_Average));

		if(pCam->GainAuto.GetValue() == Spinnaker::GainAuto_Continuous) {
			CATCH_SPINNAKER(pCam->AutoExposureControlPriority.SetValue(Spinnaker::AutoExposureControlPriority_Gain));
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
			CATCH_SPINNAKER(pCam->AutoExposureControlPriority.SetValue(Spinnaker::AutoExposureControlPriority_Gain));
		}
	} else {
		CATCH_SPINNAKER(pCam->GainAuto.SetValue(Spinnaker::GainAuto_Off));
		CATCH_SPINNAKER(pCam->Gain.SetValue(gain));
	}
}

float SpinnakerDriver::getGamma() {
    return pCam->Gamma.GetValue();
}

void SpinnakerDriver::setGamma(float gamma) {
	if(gamma == 1.0f) {
		CATCH_SPINNAKER(pCam->GammaEnable.SetValue(false));
	} else {
		CATCH_SPINNAKER(pCam->GammaEnable.SetValue(true));
		CATCH_SPINNAKER(pCam->Gamma.SetValue(gamma));
	}
}

WhiteBalanceType SpinnakerDriver::getWhiteBalanceType() {
	if(pCam->BalanceWhiteAuto.GetValue() == Spinnaker::BalanceWhiteAuto_Off) {
		return WhiteBalanceType::WhiteBalanceType_Manual;
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
		CATCH_SPINNAKER(pCam->BalanceWhiteAuto.SetValue(Spinnaker::BalanceWhiteAuto_Continuous));
		CATCH_SPINNAKER(pCam->BalanceWhiteAutoProfile.SetValue(
				type == WhiteBalanceType_AutoOutdoor
				? Spinnaker::BalanceWhiteAutoProfile_Outdoor
				: Spinnaker::BalanceWhiteAutoProfile_Indoor
		));
	} else {
		CATCH_SPINNAKER(pCam->BalanceWhiteAuto.SetValue(Spinnaker::BalanceWhiteAuto_Off));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Blue));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(blue));
		CATCH_SPINNAKER(pCam->BalanceRatioSelector.SetValue(Spinnaker::BalanceRatioSelector_Red));
		CATCH_SPINNAKER(pCam->BalanceRatio.SetValue(red));
	}
}

#endif

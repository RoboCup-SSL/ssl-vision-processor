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
#ifdef MVIMPACT_AVAILABLE

#include "mvimpactdriver.h"
#include <mvIMPACT_CPP/mvIMPACT_acquire_GenICam.h>


class MVImpactImage : public RawImage {
public:
	explicit MVImpactImage(const std::shared_ptr<Request>& request): RawImage(&PixelFormat::GRBG8, request->imageWidth.read() / 2, request->imageHeight.read() / 2, (double)request->infoTimeStamp_us.read() / 1e6, (uint8_t*)request->imageData.read()), request(request) {}

private:
	std::shared_ptr<Request> request;
};


static mvIMPACT::acquire::Device* getDevice(int hardwareId) {
	mvIMPACT::acquire::DeviceManager devMgr;
	while(devMgr.deviceCount() <= hardwareId) {
		WARN("Waiting for cam: " << devMgr.deviceCount() << "/" << (hardwareId+1));
		sleep(1);
		devMgr.updateDeviceList();
	}
	auto* device = devMgr[hardwareId];
	device->userControlledImageProcessingEnable.write(TBoolean::bFalse);

	try {
		device->open();
	} catch(mvIMPACT::acquire::ImpactAcquireException& e) {
		FATAL("Error while opening the camera: " << e.getErrorCodeAsString() << " " << e.getErrorString());
	}

	return device;
}

MVImpactDriver::MVImpactDriver(const CameraConfig& config): device(getDevice(config.hardwareId)), settings(device) {
	settings.cameraSetting.restoreDefault();
	settings.imageProcessing.restoreDefault();
	settings.imageDestination.restoreDefault();
	settings.cameraSetting.pixelFormat.write(TImageBufferPixelFormat::ibpfMono8);
	settings.imageDestination.pixelFormat.write(TImageDestinationPixelFormat::idpfRaw);

	//TODO user supplied memory functionality to prevent copies https://wassets.balluff.com/documents/DRF_957352_AA_000/CaptureToUserMemory_8cpp-example.html

	provider = std::make_unique<mvIMPACT::acquire::helper::RequestProvider>(device);
	provider->acquisitionStart();
}

MVImpactDriver::~MVImpactDriver() {
	//delete fi;
	provider->acquisitionStop();
	device->close();
}

std::shared_ptr<RawImage> MVImpactDriver::readImage() {
	std::shared_ptr<Request> request = provider->waitForNextRequest();

	//Get only newest frame
	std::shared_ptr<Request> newerRequest;
	while((newerRequest = provider->waitForNextRequest(0)) != nullptr)
		request = newerRequest;

	if(!request->isOK()) {
		WARN("Error while acquiring image: " << request->requestResult.readS());
		return nullptr;
	}

	return std::make_shared<MVImpactImage>(request);
}

const PixelFormat MVImpactDriver::format() {
	return PixelFormat::GRBG8;
}

double MVImpactDriver::expectedFrametime() {
	return 1 / GenICam::AcquisitionControl(device).mvResultingFrameRate.read();
}


uint32_t MVImpactDriver::getWidth() {
	return settings.imageDestination.imageWidth.read();
}

uint32_t MVImpactDriver::getHeight() {
	return settings.imageDestination.imageHeight.read();
}

void MVImpactDriver::setResolution(uint32_t width, uint32_t height) {
	if(width == 0 || height == 0) {
		width = settings.imageDestination.imageWidth.getMaxValue();
		height = settings.imageDestination.imageHeight.getMaxValue();
	}

	settings.imageDestination.imageWidth.write(width);
	settings.imageDestination.imageHeight.write(height);
}

float MVImpactDriver::getExposure() {
	return (float)settings.cameraSetting.expose_us.read() / 1000.0f;
}

void MVImpactDriver::setExposure(float exposure) {
	if(exposure == 0.0f) {
		settings.cameraSetting.autoExposeControl.write(TAutoExposureControl::aecOn);
	} else {
		settings.cameraSetting.autoExposeControl.write(TAutoExposureControl::aecOff);
		settings.cameraSetting.expose_us.write((int)(exposure * 1000));
	}
}

float MVImpactDriver::getGain() {
    return settings.cameraSetting.gain_dB.read();
}

void MVImpactDriver::setGain(float gain) {
	if(gain == 0.0f) {
		settings.cameraSetting.autoGainControl.write(TAutoGainControl::agcOn);
	} else {
		settings.cameraSetting.autoGainControl.write(TAutoGainControl::agcOff);
		settings.cameraSetting.gain_dB.write(gain);
	}
}

float MVImpactDriver::getGamma() {
    return 1.0f; // Unsupported? TODO
}

void MVImpactDriver::setGamma(float gamma) {
	// Unsupported? TODO
}

WhiteBalanceType MVImpactDriver::getWhiteBalanceType() {
	if(settings.imageProcessing.whiteBalanceCalibration.read() != TWhiteBalanceCalibrationMode::wbcmOff) {
		return WhiteBalanceType::WhiteBalanceType_AutoOutdoor;
	} else {
    	return WhiteBalanceType::WhiteBalanceType_Manual;
	}
}

float MVImpactDriver::getWhiteBalanceBlue() {
    return settings.imageProcessing.getWBUserSetting(0).blueGain.read();
}

float MVImpactDriver::getWhiteBalanceRed() {
    return settings.imageProcessing.getWBUserSetting(0).redGain.read();
}

void MVImpactDriver::setWhiteBalance(WhiteBalanceType type, float blue, float red) {
	if(type != WhiteBalanceType_Manual) {
		settings.imageProcessing.whiteBalanceCalibration.write(TWhiteBalanceCalibrationMode::wbcmContinuous);
	} else {
		settings.imageProcessing.whiteBalanceCalibration.write(TWhiteBalanceCalibrationMode::wbcmOff);
		WhiteBalanceSettings& wb = settings.imageProcessing.getWBUserSetting(0);
		wb.restoreDefault();
		wb.blueGain.write(blue);
		wb.redGain.write(red);
		settings.imageProcessing.whiteBalance.write(TWhiteBalanceParameter::wbpUser1);
	}
}

#endif


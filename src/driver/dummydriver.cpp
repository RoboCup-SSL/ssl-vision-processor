#include "dummydriver.h"
#include "driver/cameradriver.h"
#include "opencl.h"

DummyDriver::DummyDriver(const CameraConfig& config): image(std::make_shared<RawImage>(&PixelFormat::BGR8, 640, 480, "dummy")) {}

std::shared_ptr<RawImage> DummyDriver::readImage() {
    return image;
}

const PixelFormat DummyDriver::format() {
    return PixelFormat::BGR8;
}

double DummyDriver::expectedFrametime() {
    return 1.0 / 30.0;
}

uint32_t DummyDriver::getWidth() {
    return 640;
}

uint32_t DummyDriver::getHeight() {
    return 480;
}

void DummyDriver::setResolution(uint32_t width, uint32_t height) {}

float DummyDriver::getExposure() {
    return 1.0f;
}

void DummyDriver::setExposure(float exposure) {}

float DummyDriver::getGain() {
    return 1.0f;
}

void DummyDriver::setGain(float gain) {}

float DummyDriver::getGamma() {
    return 1.0f;
}

void DummyDriver::setGamma(float gamma) {}

WhiteBalanceType DummyDriver::getWhiteBalanceType() {
    return WhiteBalanceType::WhiteBalanceType_Manual;
}

float DummyDriver::getWhiteBalanceBlue() {
    return 1.0f;
}

float DummyDriver::getWhiteBalanceRed() {
    return 1.0f;
}

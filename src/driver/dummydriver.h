#pragma once
#include "cameradriver.h"

class DummyDriver : public CameraDriver {
public:
	explicit DummyDriver(const CameraConfig& config);
	~DummyDriver() override = default;

	std::shared_ptr<RawImage> readImage() override;

	const PixelFormat format() override;

	double expectedFrametime() override;

    uint32_t getWidth() override;
	uint32_t getHeight() override;
	void setResolution(uint32_t width, uint32_t height) override;

	float getExposure() override;
	void setExposure(float exposure) override;

	float getGain() override;
	void setGain(float gain) override;

	float getGamma() override;
	void setGamma(float gamma) override;

	WhiteBalanceType getWhiteBalanceType() override;
	float getWhiteBalanceBlue() override;
	float getWhiteBalanceRed() override;
	void setWhiteBalance(WhiteBalanceType type, float blue, float red) override;

private:
	std::shared_ptr<RawImage> image;
};

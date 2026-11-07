#pragma once
#ifdef DC1394_AVAILABLE

#include "cameradriver.h"
#include <dc1394/dc1394.h>

/** IIDC (FireWire) cameras via libdc1394, e.g. AVT Stingray. Captures Format7 mode 0 in YUV422 and converts to BGR8. */
class DC1394Driver : public CameraDriver {
public:
	explicit DC1394Driver(const CameraConfig& config);
	~DC1394Driver() override;

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
	void startCapture();
	void stopCapture();

	bool setAuto(dc1394feature_t feature, bool automatic);
	// absScale/rawScale convert from config units to the camera's absolute/raw register units
	float getFeature(dc1394feature_t feature, float absScale, float rawScale);
	void setFeature(dc1394feature_t feature, float value, float absScale, float rawScale);

	dc1394_t* dc1394;
	dc1394camera_t* camera = nullptr;
	const dc1394video_mode_t mode = DC1394_VIDEO_MODE_FORMAT7_0;
	bool capturing = false;

	std::shared_ptr<RawImage> image = nullptr;
	std::string name;
};

#endif

#ifdef DC1394_AVAILABLE

#include "dc1394driver.h"

#include <algorithm>
#include <cmath>
#include <unistd.h>
#include <opencv2/imgproc.hpp>

// Raw IIDC shutter values are vendor specific. AVT cameras (e.g. Stingray) default to a 20 µs timebase.
static constexpr float SHUTTER_RAW_PER_MS = 1000.0f / 20.0f;

static dc1394camera_t* getCamera(dc1394_t* dc1394, int hardwareId) {
	dc1394camera_list_t* list = nullptr;
	while(true) {
		if(dc1394_camera_enumerate(dc1394, &list) != DC1394_SUCCESS || list == nullptr)
			FATAL("[DC1394] Could not enumerate cameras (is firewire_ohci loaded and /dev/fw* accessible?)");

		if((int)list->num > hardwareId)
			break;

		WARN("[DC1394] Waiting for cam: " << list->num << "/" << (hardwareId+1));
		dc1394_camera_free_list(list);
		sleep(1);
	}

	// Sort by GUID for a stable id assignment independent of bus enumeration order
	std::sort(list->ids, list->ids + list->num, [](const dc1394camera_id_t& a, const dc1394camera_id_t& b) { return a.guid < b.guid; });
	uint64_t guid = list->ids[hardwareId].guid;
	dc1394_camera_free_list(list);

	dc1394camera_t* camera = dc1394_camera_new(dc1394, guid);
	if(camera == nullptr)
		FATAL("[DC1394] Could not open camera " << hardwareId);

	VP_LOG("[DC1394] Opened " << camera->vendor << " " << camera->model << " (GUID " << std::hex << guid << std::dec << ")");
	return camera;
}

DC1394Driver::DC1394Driver(const CameraConfig& config): dc1394(dc1394_new()) {
	if(dc1394 == nullptr)
		FATAL("[DC1394] Could not initialize libdc1394");

	camera = getCamera(dc1394, config.hardwareId);
	name = std::string(camera->vendor) + "_" + camera->model;
	std::replace(name.begin(), name.end(), ' ', '_');

	dc1394switch_t transmission = DC1394_OFF;
	if(dc1394_video_get_transmission(camera, &transmission) == DC1394_SUCCESS && transmission == DC1394_ON)
		WARN("[DC1394] Camera is already transmitting, it might be in use by another program (e.g. ssl-vision)");

	if(camera->bmode_capable) {
		dc1394_video_set_operation_mode(camera, DC1394_OPERATION_MODE_1394B);
		dc1394_video_set_iso_speed(camera, DC1394_ISO_SPEED_800);
	} else {
		dc1394_video_set_operation_mode(camera, DC1394_OPERATION_MODE_LEGACY);
		dc1394_video_set_iso_speed(camera, DC1394_ISO_SPEED_400);
	}

	if(dc1394_video_set_mode(camera, mode) != DC1394_SUCCESS)
		FATAL("[DC1394] Camera does not support Format7 mode 0");
}

DC1394Driver::~DC1394Driver() {
	stopCapture();
	dc1394_camera_free(camera);
	dc1394_free(dc1394);
}

void DC1394Driver::startCapture() {
	if(dc1394_capture_setup(camera, 4, DC1394_CAPTURE_FLAGS_DEFAULT) != DC1394_SUCCESS)
		FATAL("[DC1394] Could not set up capture, ISO channel or bandwidth unavailable. Is the camera in use by another program (e.g. ssl-vision)?");

	if(dc1394_video_set_transmission(camera, DC1394_ON) != DC1394_SUCCESS) {
		dc1394_capture_stop(camera);
		FATAL("[DC1394] Could not start camera transmission");
	}

	capturing = true;
}

void DC1394Driver::stopCapture() {
	if(!capturing)
		return;

	dc1394_video_set_transmission(camera, DC1394_OFF);
	dc1394_capture_stop(camera);
	capturing = false;
}

std::shared_ptr<RawImage> DC1394Driver::readImage() {
	if(!capturing)
		startCapture();

	dc1394video_frame_t* frame = nullptr;
	while(true) {
		if(dc1394_capture_dequeue(camera, DC1394_CAPTURE_POLICY_WAIT, &frame) != DC1394_SUCCESS || frame == nullptr) {
			WARN("[DC1394] Error while acquiring image");
			return nullptr;
		}

		//Get only newest frame
		dc1394video_frame_t* newerFrame = nullptr;
		while(dc1394_capture_dequeue(camera, DC1394_CAPTURE_POLICY_POLL, &newerFrame) == DC1394_SUCCESS && newerFrame != nullptr) {
			dc1394_capture_enqueue(camera, frame);
			frame = newerFrame;
		}

		if(!dc1394_capture_is_frame_corrupt(camera, frame))
			break;

		WARN("[DC1394] Dropping corrupt frame");
		dc1394_capture_enqueue(camera, frame);
	}

	const int width = (int)frame->size[0];
	const int height = (int)frame->size[1];
	if(image == nullptr || !image.unique() || image->width != width || image->height != height)
		image = std::make_shared<RawImage>(&PixelFormat::BGR8, width, height, name);

	image->timestamp = (double)frame->timestamp / 1e6;
	{
		CLMap<uint8_t> map = image->write<uint8_t>();
		cv::Mat bgr(cv::Size(width, height), CV_8UC3, (void*)*map);
		cv::cvtColor(cv::Mat(cv::Size(width, height), CV_8UC2, frame->image), bgr, cv::COLOR_YUV2BGR_UYVY);
	}

	dc1394_capture_enqueue(camera, frame);
	return image;
}

const PixelFormat DC1394Driver::format() {
	return PixelFormat::BGR8;
}

double DC1394Driver::expectedFrametime() {
	// Format7 frame rate is defined by the packet size, one isochronous packet is sent every 125 µs
	uint32_t packetsPerFrame = 0;
	if(dc1394_format7_get_packets_per_frame(camera, mode, &packetsPerFrame) != DC1394_SUCCESS || packetsPerFrame == 0)
		return 1 / 60.0;

	return packetsPerFrame * 125e-6;
}


uint32_t DC1394Driver::getWidth() {
	uint32_t width = 0, height = 0;
	dc1394_format7_get_image_size(camera, mode, &width, &height);
	return width;
}

uint32_t DC1394Driver::getHeight() {
	uint32_t width = 0, height = 0;
	dc1394_format7_get_image_size(camera, mode, &width, &height);
	return height;
}

void DC1394Driver::setResolution(uint32_t width, uint32_t height) {
	const bool wasCapturing = capturing;
	stopCapture();

	uint32_t maxWidth, maxHeight, unitWidth, unitHeight, unitLeft, unitTop;
	dc1394_format7_get_max_image_size(camera, mode, &maxWidth, &maxHeight);
	dc1394_format7_get_unit_size(camera, mode, &unitWidth, &unitHeight);
	dc1394_format7_get_unit_position(camera, mode, &unitLeft, &unitTop);
	unitLeft = std::max(unitLeft, 1u);
	unitTop = std::max(unitTop, 1u);

	if(width == 0 || height == 0 || width > maxWidth || height > maxHeight) {
		width = maxWidth;
		height = maxHeight;
	}
	width -= width % unitWidth;
	height -= height % unitHeight;

	// Center the region of interest on the sensor
	uint32_t left = (maxWidth - width) / 2;
	uint32_t top = (maxHeight - height) / 2;
	left -= left % unitLeft;
	top -= top % unitTop;

	if(dc1394_format7_set_roi(camera, mode, DC1394_COLOR_CODING_YUV422, DC1394_USE_MAX_AVAIL, (int32_t)left, (int32_t)top, (int32_t)width, (int32_t)height) != DC1394_SUCCESS)
		FATAL("[DC1394] Could not set resolution " << width << "x" << height << " in YUV422");

	VP_LOG("[DC1394] Resolution " << width << "x" << height << ", " << std::round(1 / expectedFrametime()) << " fps");

	if(wasCapturing)
		startCapture();
}


bool DC1394Driver::setAuto(dc1394feature_t feature, bool automatic) {
	dc1394bool_t present = DC1394_FALSE;
	if(dc1394_feature_is_present(camera, feature, &present) != DC1394_SUCCESS || !present)
		return false;

	dc1394feature_modes_t modes;
	dc1394_feature_get_modes(camera, feature, &modes);
	bool autoAvailable = std::find(modes.modes, modes.modes + modes.num, DC1394_FEATURE_MODE_AUTO) != modes.modes + modes.num;
	if(automatic && !autoAvailable)
		WARN("[DC1394] Automatic " << dc1394_feature_get_string(feature) << " unsupported, keeping current value");

	dc1394_feature_set_power(camera, feature, DC1394_ON);
	dc1394_feature_set_mode(camera, feature, automatic && autoAvailable ? DC1394_FEATURE_MODE_AUTO : DC1394_FEATURE_MODE_MANUAL);
	return !automatic;
}

float DC1394Driver::getFeature(dc1394feature_t feature, float absScale, float rawScale) {
	dc1394bool_t absolute = DC1394_FALSE;
	dc1394_feature_has_absolute_control(camera, feature, &absolute);
	if(absolute) {
		float value = 0.0f;
		dc1394_feature_get_absolute_value(camera, feature, &value);
		return value / absScale;
	}

	uint32_t value = 0;
	dc1394_feature_get_value(camera, feature, &value);
	return (float)value / rawScale;
}

void DC1394Driver::setFeature(dc1394feature_t feature, float value, float absScale, float rawScale) {
	dc1394bool_t absolute = DC1394_FALSE;
	dc1394_feature_has_absolute_control(camera, feature, &absolute);
	if(absolute) {
		float min, max;
		dc1394_feature_get_absolute_boundaries(camera, feature, &min, &max);
		dc1394_feature_set_absolute_control(camera, feature, DC1394_ON);
		dc1394_feature_set_absolute_value(camera, feature, std::clamp(value * absScale, min, max));
	} else {
		uint32_t min, max;
		dc1394_feature_get_boundaries(camera, feature, &min, &max);
		dc1394_feature_set_absolute_control(camera, feature, DC1394_OFF);
		dc1394_feature_set_value(camera, feature, std::clamp((uint32_t)std::lround(value * rawScale), min, max));
	}
}


float DC1394Driver::getExposure() {
	return getFeature(DC1394_FEATURE_SHUTTER, 1e-3f, SHUTTER_RAW_PER_MS);
}

void DC1394Driver::setExposure(float exposure) {
	if(setAuto(DC1394_FEATURE_SHUTTER, exposure == 0.0f))
		setFeature(DC1394_FEATURE_SHUTTER, exposure, 1e-3f, SHUTTER_RAW_PER_MS);
}

float DC1394Driver::getGain() {
	return getFeature(DC1394_FEATURE_GAIN, 1.0f, 1.0f);
}

void DC1394Driver::setGain(float gain) {
	if(setAuto(DC1394_FEATURE_GAIN, gain == 0.0f))
		setFeature(DC1394_FEATURE_GAIN, gain, 1.0f, 1.0f);
}

float DC1394Driver::getGamma() {
	uint32_t value = 0, min = 0, max = 0;
	dc1394_feature_get_value(camera, DC1394_FEATURE_GAMMA, &value);
	dc1394_feature_get_boundaries(camera, DC1394_FEATURE_GAMMA, &min, &max);
	return value == min ? 1.0f : 0.45f;
}

void DC1394Driver::setGamma(float gamma) {
	// IIDC gamma is usually just a switch between linear and a camera specific (e.g. 0.45 on AVT) lookup table
	if(!setAuto(DC1394_FEATURE_GAMMA, false))
		return;

	uint32_t min, max;
	dc1394_feature_get_boundaries(camera, DC1394_FEATURE_GAMMA, &min, &max);
	dc1394_feature_set_value(camera, DC1394_FEATURE_GAMMA, gamma < 1.0f ? max : min);
}

WhiteBalanceType DC1394Driver::getWhiteBalanceType() {
	dc1394feature_mode_t featureMode = DC1394_FEATURE_MODE_MANUAL;
	dc1394_feature_get_mode(camera, DC1394_FEATURE_WHITE_BALANCE, &featureMode);
	return featureMode == DC1394_FEATURE_MODE_MANUAL ? WhiteBalanceType_Manual : WhiteBalanceType_AutoOutdoor;
}

float DC1394Driver::getWhiteBalanceBlue() {
	uint32_t blue = 0, red = 0;
	dc1394_feature_whitebalance_get_value(camera, &blue, &red);
	return (float)blue;
}

float DC1394Driver::getWhiteBalanceRed() {
	uint32_t blue = 0, red = 0;
	dc1394_feature_whitebalance_get_value(camera, &blue, &red);
	return (float)red;
}

void DC1394Driver::setWhiteBalance(WhiteBalanceType type, float blue, float red) {
	if(!setAuto(DC1394_FEATURE_WHITE_BALANCE, type != WhiteBalanceType_Manual))
		return;

	// IIDC white balance has no standardized unit, the values are the raw U/B and V/R camera registers
	uint32_t min, max;
	dc1394_feature_get_boundaries(camera, DC1394_FEATURE_WHITE_BALANCE, &min, &max);
	dc1394_feature_whitebalance_set_value(camera, std::clamp((uint32_t)std::lround(blue), min, max), std::clamp((uint32_t)std::lround(red), min, max));
}

#endif

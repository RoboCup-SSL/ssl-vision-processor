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
#include <yaml-cpp/yaml.h>
#include <sys/stat.h>
#include "log.h"

#include "cl_kernels.h"
#include "Resources.h"
#include "driver/cameradriver.h"

template<>
struct YAML::convert<Eigen::Vector2f> {
	static YAML::Node encode(const Eigen::Vector2f& rhs) {
		YAML::Node node;
		node.push_back(rhs.x());
		node.push_back(rhs.y());
		return node;
	}

	static bool decode(const YAML::Node& node, Eigen::Vector2f& rhs) {
		if(!node.IsSequence() || node.size() != 2) {
			return false;
		}

		rhs.x() = node[0].as<float>();
		rhs.y() = node[1].as<float>();
		return true;
	}
};

template<>
struct YAML::convert<Eigen::Vector3i> {
	static YAML::Node encode(const Eigen::Vector3i& rhs) {
		YAML::Node node;
		node.push_back(rhs.x());
		node.push_back(rhs.y());
		node.push_back(rhs.z());
		return node;
	}

	static bool decode(const YAML::Node& node, Eigen::Vector3i& rhs) {
		if(!node.IsSequence() || node.size() != 3) {
			return false;
		}

		rhs.x() = node[0].as<int>();
		rhs.y() = node[1].as<int>();
		rhs.z() = node[2].as<int>();
		return true;
	}
};

static YAML::Node getOptional(const YAML::Node& node) {
	return node.IsDefined() ? node : YAML::Node();
}

Resources::Resources(const std::string& configPath) : configPath(configPath) {
	YAML::Node config = YAML::LoadFile(configPath);
	struct stat st{};
	if(stat(configPath.c_str(), &st) == 0)
		configMtime = (int64_t)st.st_mtim.tv_sec * 1000000000 + st.st_mtim.tv_nsec;

	openCl = std::make_shared<OpenCL>();

	applyTunables(config);

	YAML::Node geometry = getOptional(config["geometry"]);
	cameraAmount = geometry["camera_amount"].as<int>(1);
	cameraHeight = geometry["camera_height"].as<double>(0.0);
	lineCorners = geometry["line_corners"].as<std::vector<Eigen::Vector2f>>(std::vector<Eigen::Vector2f>());
	geometryRefinement = geometry["refinement"].as<bool>(false);
	fieldLineThreshold = geometry["field_line_threshold"].as<int>(5);
	minLineSegmentLength = geometry["min_line_segment_length"].as<double>(10.0);
	maxLineSegmentOffset = geometry["max_line_segment_offset"].as<double>(10.0);
	maxLineSegmentAngle = geometry["max_line_segment_angle"].as<double>(3.0) * M_PI/180.0;

	YAML::Node debug = getOptional(config["debug"]);
	groundTruth = debug["ground_truth"].as<std::string>("gt.yml");
	bool waitForGeometry = debug["wait_for_geometry"].as<bool>(false);

	YAML::Node network = getOptional(config["network"]);
	gcSocket = std::make_shared<GCSocket>(network["gc_ip"].as<std::string>("224.5.23.1"), network["gc_port"].as<int>(10003), YAML::LoadFile(config["bot_heights_file"].as<std::string>("robot-heights.yml")).as<std::map<std::string, double>>());
	socket = std::make_shared<VisionSocket>(network["vision_ip"].as<std::string>("224.5.23.2"), network["vision_port"].as<int>(10006), gcSocket->defaultBotHeight);
	perspective = std::make_shared<Perspective>(socket);

	YAML::Node stream = getOptional(config["stream"]);
	snapshotWriter = std::make_shared<SnapshotWriter>();

	raw2quadKernel = openCl->compile(kernel_raw2quad_cl, camera->format().kernelOptions);
	resampling = openCl->compile(kernel_resampling_cl, camera->format().kernelOptions);
	gradientDot = openCl->compile(kernel_gradientDot_cl);
	satHorizontal = openCl->compile(kernel_satHorizontal_cl);
	satVertical = openCl->compile(kernel_satVertical_cl);
	satBlobCenter = openCl->compile(kernel_satBlobCenter_cl);
	quad2rgbaKernel = openCl->compile(kernel_quad2rgba_cl, camera->format().kernelOptions);
	quad2nv12 = openCl->compile(kernel_quad2nv12_cl, camera->format().kernelOptions);
	rgba2nv12 = openCl->compile(kernel_rgba2nv12_cl);
	f2nv12 = openCl->compile(kernel_f2nv12_cl);

	while(waitForGeometry && !socket->getGeometryVersion()) {
		std::this_thread::sleep_for(std::chrono::milliseconds(1));
		socket->geometryCheck();
	}
}

void Resources::raw2quad(const RawImage& img, std::shared_ptr<CLImage>* channels) {
	for(int i = 0; i < 4; i++)
		channels[i] = openCl->acquire(&PixelFormat::U8, img.width, img.height, img.name);

	openCl->await(raw2quadKernel, cl::EnqueueArgs(cl::NDRange(img.width, img.height)), img.buffer, channels[0]->image, channels[1]->image, channels[2]->image, channels[3]->image);
}

std::shared_ptr<CLImage> Resources::quad2rgba(std::shared_ptr<CLImage>* channels) {
	std::shared_ptr<CLImage> rgba = openCl->acquire(&PixelFormat::RGBA8, channels[0]->width, channels[0]->height, channels[0]->name);
	openCl->await(quad2rgbaKernel, cl::EnqueueArgs(cl::NDRange(channels[0]->width, channels[0]->height)), channels[0]->image, channels[1]->image, channels[2]->image, channels[3]->image, rgba->image);
	return rgba;
}

void Resources::rgba2blobCenter(const std::shared_ptr<CLImage>* channels, std::shared_ptr<CLImage>& flat, std::shared_ptr<CLImage>& gradDot, std::shared_ptr<CLImage>& blobCenter) {
	cl::NDRange visibleFieldRange(perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1]);
	flat = openCl->acquire(&PixelFormat::RGBA8, perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1], channels[0]->name);
	gradDot = openCl->acquire(&PixelFormat::F32, perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1], channels[0]->name);
	std::shared_ptr<CLImage> gradDotHor = openCl->acquire(&PixelFormat::F32, perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1], channels[0]->name);
	std::shared_ptr<CLImage> gradDotSat = openCl->acquire(&PixelFormat::F32, perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1], channels[0]->name);
	blobCenter = openCl->acquire(&PixelFormat::F32, perspective->reprojectedFieldSize[0], perspective->reprojectedFieldSize[1], channels[0]->name);

	cl::Event e1 = openCl->run(resampling, cl::EnqueueArgs(visibleFieldRange), channels[0]->image, channels[1]->image, channels[2]->image, channels[3]->image, flat->image, perspective->getCLCameraModel(), (float)gcSocket->maxBotHeight, perspective->fieldScale, perspective->visibleFieldExtent[0], perspective->visibleFieldExtent[2]);
	cl::Event e2 = openCl->run(gradientDot, cl::EnqueueArgs(e1, visibleFieldRange), flat->image, gradDot->image, (int)ceilf(perspective->maxBlobRadius / perspective->fieldScale) / 3);
	cl::Event e3 = openCl->run(satHorizontal, cl::EnqueueArgs(e2, cl::NDRange(perspective->reprojectedFieldSize[1])), gradDot->image, gradDotHor->image);
	cl::Event e4 = openCl->run(satVertical, cl::EnqueueArgs(e3, cl::NDRange(perspective->reprojectedFieldSize[0])), gradDotHor->image, gradDotSat->image);
	openCl->await(satBlobCenter, cl::EnqueueArgs(e4, visibleFieldRange), gradDotSat->image, blobCenter->image, (int)ceilf(perspective->minBlobRadius / perspective->fieldScale));
}

void Resources::streamQuad(std::shared_ptr<CLImage>* channels) {
	std::shared_ptr<RawImage> nv12 = openCl->acquireNV12(channels[0]->width, channels[0]->height);
	openCl->await(quad2nv12, cl::EnqueueArgs(cl::NDRange(channels[0]->width, channels[0]->height)), channels[0]->image, channels[1]->image, channels[2]->image, channels[3]->image, nv12->buffer);
	rtpStreamer.sendFrame(nv12);
}

void Resources::streamImage(CLImage &img) {
	cl::Kernel kernel;
	if(img.format == &PixelFormat::RGBA8) {
		kernel = rgba2nv12;
	} else if(img.format == &PixelFormat::F32) {
		kernel = f2nv12;
	} else {
		WARN("Unimplemented pixel format submitted for streaming.");
		return;
	}

	std::shared_ptr<RawImage> nv12 = openCl->acquireNV12(img.width, img.height);
	openCl->await(kernel, cl::EnqueueArgs(cl::NDRange(img.width, img.height)), img.image, nv12->buffer);
	rtpStreamer.sendFrame(nv12);
}

void Resources::applyTunables(const YAML::Node& config) {
	YAML::Node thresholds = getOptional(config["thresholds"]);
	resamplingFactor = thresholds["resampling_factor"].as<float>(1.0f);

	YAML::Node debug = getOptional(config["debug"]);
	debugImages = debug["debug_images"].as<bool>(false);
	debugStreamIntervalMs = debug["debug_stream_interval_ms"].as<int>(0);
}

void Resources::reloadConfigIfChanged() {
	double now = getRealTime();
	if(now - lastConfigCheckTime < 0.5)
		return;
	lastConfigCheckTime = now;

	struct stat st{};
	if(stat(configPath.c_str(), &st) != 0)
		return;
	int64_t mtime = (int64_t)st.st_mtim.tv_sec * 1000000000 + st.st_mtim.tv_nsec;
	if(mtime == configMtime)
		return;
	configMtime = mtime;

	try {
		YAML::Node config = YAML::LoadFile(configPath);
		applyTunables(config);
		VP_LOG("Reloaded tunables from " << configPath);
	} catch(const YAML::Exception& e) {
		WARN("Config reload failed, keeping previous values: " << e.what());
	}
}

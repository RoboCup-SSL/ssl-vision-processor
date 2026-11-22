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
#pragma once


#include <string>
#include <memory>
#include <yaml-cpp/node/node.h>
#include "rtpstreamer.h"
#include "snapshotwriter.h"
#include "udpsocket.h"
#include "Perspective.h"
#include "opencl.h"


typedef struct __attribute__ ((packed)) RGB {
	cl_uchar r;
	cl_uchar g;
	cl_uchar b;

	auto operator<=>(const RGB&) const = default;
} RGB;


class Resources {
public:
	explicit Resources(const std::string& configPath);

	void reloadConfigIfChanged();

	float resamplingFactor;

	int cameraAmount;
	double cameraHeight; // Just for calibration, do not use elsewhere (0.0 as special value for automatic calibration)
	std::vector<Eigen::Vector2f> lineCorners;
	bool geometryRefinement;
	uint8_t fieldLineThreshold;
	double minLineSegmentLength;
	double maxLineSegmentOffset;
	double maxLineSegmentAngle;

	std::string groundTruth;
	bool debugImages;
	int debugStreamIntervalMs;

	std::shared_ptr<GCSocket> gcSocket;
	std::shared_ptr<VisionSocket> socket;
	std::shared_ptr<Perspective> perspective;
	std::shared_ptr<OpenCL> openCl;
	std::shared_ptr<SnapshotWriter> snapshotWriter;

	cl::Kernel raw2quadKernel;
	cl::Kernel resampling;
	cl::Kernel gradientDot;
	cl::Kernel satHorizontal;
	cl::Kernel satVertical;
	cl::Kernel satBlobCenter;
	cl::Kernel quad2rgbaKernel;
	cl::Kernel quad2nv12;
	cl::Kernel rgba2nv12;
	cl::Kernel f2nv12;

	void raw2quad(const RawImage& img, std::shared_ptr<CLImage>* channels);
	std::shared_ptr<CLImage> quad2rgba(std::shared_ptr<CLImage>* channels);
	void rgba2blobCenter(const std::shared_ptr<CLImage>* channels, std::shared_ptr<CLImage>& flat, std::shared_ptr<CLImage>& gradDot, std::shared_ptr<CLImage>& blobCenter);

	void streamQuad(std::shared_ptr<CLImage>* channels);
	void streamImage(CLImage& img);

private:
	std::string configPath;
	int64_t configMtime = 0;
	double lastConfigCheckTime = 0.0;
	void applyTunables(const YAML::Node& config);
};

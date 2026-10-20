/*
     Copyright 2026 Will Stuckey

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
#ifdef SPINNAKER_AVAILABLE

#include <string>
#include <vector>

#include "Spinnaker.h"

/** A camera feature that camera generations expose under different GenICam node names. Shared by the
 * Spinnaker driver and tools/spinnaker_probe, so the probe reports exactly what the driver will use. */
struct SpinnakerFeature {
	const char* label;
	std::vector<const char*> nodes; // In order of preference
};

namespace SpinnakerFeatures {
	// Blackfly S, older firmware, first generation Blackfly GigE manual
	inline const SpinnakerFeature FrameRateControl{"frame rate control", {"AcquisitionFrameRateEnable", "AcquisitionFrameRateEnabled", "AcquisitionFrameRateControlEnabled"}};
	inline const SpinnakerFeature GammaEnable{"gamma enable", {"GammaEnable", "GammaEnabled"}};
	inline const SpinnakerFeature Gamma{"gamma", {"Gamma"}};
	inline const SpinnakerFeature AutoExposureMetering{"auto exposure metering", {"AutoExposureMeteringMode"}};
	inline const SpinnakerFeature AutoExposurePriority{"auto exposure priority", {"AutoExposureControlPriority"}};
	inline const SpinnakerFeature WhiteBalanceProfile{"auto white balance profile", {"BalanceWhiteAutoProfile"}};
	inline const SpinnakerFeature PixelFormatNode{"pixel format", {"PixelFormat"}};
	inline const SpinnakerFeature PixelColorFilter{"pixel color filter", {"PixelColorFilter"}};

	inline const SpinnakerFeature* const All[] = {
			&FrameRateControl, &GammaEnable, &Gamma, &AutoExposureMetering, &AutoExposurePriority,
			&WhiteBalanceProfile, &PixelFormatNode, &PixelColorFilter,
	};

	/** Availability can depend on register reads (pIsAvailable), which may fail; treat that as unavailable. */
	template<typename T>
	inline bool isAvailable(const T& node) {
		try {
			return Spinnaker::GenApi::IsAvailable(node);
		} catch (Spinnaker::Exception &e) {
			return false;
		}
	}

	// Bayer patterns the OpenCL kernels support, the 8 bit pixel formats are named <pattern>8
	inline const char* const BayerPatterns[] = {"BayerRG", "BayerBG", "BayerGR"};

	/** The PixelFormat entry the driver selects: the first offered 8 bit Bayer format, else "Raw8"
	 * (first generation naming, the pattern is then reported by PixelColorFilter), else "". */
	inline std::string findBayerFormat(Spinnaker::GenApi::INodeMap& nodeMap) {
		Spinnaker::GenApi::CEnumerationPtr formatNode = nodeMap.GetNode("PixelFormat");
		if(!isAvailable(formatNode))
			return "";

		for(const char* pattern : BayerPatterns) {
			const std::string name = std::string(pattern) + "8";
			if(isAvailable(Spinnaker::GenApi::CEnumEntryPtr(formatNode->GetEntryByName(name.c_str()))))
				return name;
		}

		if(isAvailable(Spinnaker::GenApi::CEnumEntryPtr(formatNode->GetEntryByName("Raw8"))))
			return "Raw8";
		return "";
	}
}

#endif

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

#include <array>
#include <string>
#include <string_view>

#include "Spinnaker.h"

/** Pixel format selection shared by the Spinnaker driver and tools/spinnaker_probe, so the probe
 * reports exactly what the driver will use. */
namespace SpinnakerFeatures {
	/** Availability can depend on register reads, which may fail (also with non-Spinnaker exceptions); treat that as unavailable. */
	template<typename T>
	inline bool isAvailable(const T& node) {
		try {
			return Spinnaker::GenApi::IsAvailable(node);
		} catch (std::exception &e) {
			return false;
		}
	}

	// Bayer patterns the OpenCL kernels support, the 8 bit pixel formats are named <pattern>8
	inline constexpr auto BayerPatterns = std::to_array<std::string_view>({"BayerRG", "BayerBG", "BayerGR"});

	/** The PixelFormat entry the driver selects: the first offered 8 bit Bayer format, else "Raw8"
	 * (first generation naming, the pattern is then reported by PixelColorFilter), else "". */
	inline std::string findBayerFormat(Spinnaker::GenApi::INodeMap& nodeMap) {
		Spinnaker::GenApi::CEnumerationPtr formatNode = nodeMap.GetNode("PixelFormat");
		if(!isAvailable(formatNode))
			return "";

		for(std::string_view pattern : BayerPatterns) {
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

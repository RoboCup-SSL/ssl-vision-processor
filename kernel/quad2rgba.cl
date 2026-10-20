/*
     Copyright 2025 Felix Weinmann

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
#ifndef CL_VERSION_1_0
#include "clstd.h"
#endif


const sampler_t sampler = CLK_FILTER_LINEAR | CLK_NORMALIZED_COORDS_FALSE | CLK_ADDRESS_CLAMP_TO_EDGE;

kernel void quad2rgba(read_only image2d_t channel0, read_only image2d_t channel1, read_only image2d_t channel2, read_only image2d_t channel3, write_only image2d_t out) {
	int2 pos = (int2)(get_global_id(0), get_global_id(1));

#ifdef BGR
	uint4 color = (uint4)(
			read_imageui(channel2, sampler, pos).x,
			read_imageui(channel1, sampler, pos).x,
			read_imageui(channel0, sampler, pos).x,
			255
	);
#endif

#ifdef RGGB
	uint4 color = (uint4)(
			read_imageui(channel0, sampler, (float2)(pos.x + 0.25f, pos.y + 0.25f)).x,
			read_imageui(channel1, sampler, (float2)(pos.x - 0.25f, pos.y + 0.25f)).x/2 + read_imageui(channel2, sampler, (float2)(pos.x + 0.25f, pos.y - 0.25f)).x/2,
			read_imageui(channel3, sampler, (float2)(pos.x - 0.25f, pos.y - 0.25f)).x,
			255
	);
#endif

#ifdef GRBG
	uint4 color = (uint4)(
			read_imageui(channel1, sampler, (float2)(pos.x - 0.25f, pos.y + 0.25f)).x,
			read_imageui(channel0, sampler, (float2)(pos.x + 0.25f, pos.y + 0.25f)).x/2 + read_imageui(channel3, sampler, (float2)(pos.x - 0.25f, pos.y - 0.25f)).x/2,
			read_imageui(channel2, sampler, (float2)(pos.x + 0.25f, pos.y - 0.25f)).x,
			255
	);
#endif

#ifdef BGGR
	uint4 color = (uint4)(
			read_imageui(channel3, sampler, (float2)(pos.x - 0.25f, pos.y - 0.25f)).x,
			read_imageui(channel1, sampler, (float2)(pos.x - 0.25f, pos.y + 0.25f)).x/2 + read_imageui(channel2, sampler, (float2)(pos.x + 0.25f, pos.y - 0.25f)).x/2,
			read_imageui(channel0, sampler, (float2)(pos.x + 0.25f, pos.y + 0.25f)).x,
			255
	);
#endif

	write_imageui(out, pos, color);
}
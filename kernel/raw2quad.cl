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


kernel void raw2quad(global const uchar* img, write_only image2d_t channel0, write_only image2d_t channel1, write_only image2d_t channel2, write_only image2d_t channel3) {
	const int2 pos = (int2)(get_global_id(0), get_global_id(1));

#ifdef BGR
	const int imgpos = 3*(pos.x + pos.y*get_global_size(0));

	write_imageui(channel0, pos, img[imgpos]);
	write_imageui(channel1, pos, img[imgpos+1]);
	write_imageui(channel2, pos, img[imgpos+2]);
#elif defined RGGB || defined GRBG || defined BGGR
	const int row_size = 2*get_global_size(0);
	const int imgpos = 2*pos.x + 2*pos.y*row_size;

	write_imageui(channel0, pos, img[imgpos]);
	write_imageui(channel1, pos, img[imgpos+1]);
	write_imageui(channel2, pos, img[imgpos+row_size]);
	write_imageui(channel3, pos, img[imgpos+1+row_size]);
#endif
}

#pragma once
#include "vision/ssl_vp_config.pb.h"

extern SSL_VPConfig config;

void init_config(const std::string& configPath);

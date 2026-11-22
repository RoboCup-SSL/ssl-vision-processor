#pragma once
#include "vision/ssl_vp_config.pb.h"
#include "Resources.h"

extern SSL_VPConfig config;

// Live used colors duplicated as Eigen vectors for performance reasons, shall be modified through apply_color_update() only.
extern Eigen::Vector3i liveOrange;
extern Eigen::Vector3i liveField;
extern Eigen::Vector3i liveYellow;
extern Eigen::Vector3i liveBlue;
extern Eigen::Vector3i liveGreen;
extern Eigen::Vector3i livePink;
extern Eigen::Vector3i liveFieldLineColor;

void init_config(const std::string& configPath);
void apply_received_config(Resources& r);

Color color_from_eigen(const Eigen::Vector3i& eigen);
Eigen::Vector3i eigen_from_color(const Color& color);
void apply_color_update();

#include "config.h"
#include <yaml-cpp/yaml.h>


SSL_VPConfig config;


static YAML::Node getOptional(const YAML::Node& node) {
	return node.IsDefined() ? node : YAML::Node();
}

static void set_color(VPColor* color, const YAML::Node& node, int r, int g, int b) {
    color->mutable_reference()->set_red(node[0].as<int>(r));
    color->mutable_reference()->set_green(node[1].as<int>(g));
    color->mutable_reference()->set_blue(node[2].as<int>(b));
    color->mutable_live()->CopyFrom(color->reference());
}

void init_config(const std::string& configPath) {
	char host[256];
	gethostname(host, 256);
    config.set_instance(host);

    // Interim copy from config file until network config is fully operational
    YAML::Node yaml = YAML::LoadFile(configPath);
    config.set_camera_id(yaml["cam_id"].as<int>(0));
    for (auto const& x : YAML::LoadFile(yaml["bot_heights_file"].as<std::string>("robot-heights.yml")).as<std::map<std::string, double>>()) {
        config.mutable_robot_heights()->insert(x);
    }

    std::string driver = getOptional(yaml["camera"])["driver"].as<std::string>("SPINNAKER");
    if(driver == "SPINNAKER") {
        config.mutable_camera()->set_driver(SPINNAKER);
    } else if(driver == "MVIMPACT") {
        config.mutable_camera()->set_driver(MVIMPACT);
    } else if(driver == "DC1394") {
        config.mutable_camera()->set_driver(DC1394);
    } else if(driver == "OPENCV") {
        config.mutable_camera()->set_driver(OPENCV);
    } else {
        config.mutable_camera()->set_driver(DUMMY);
    }
    config.mutable_camera()->set_id(getOptional(yaml["camera"])["id"].as<int>(0));
    config.mutable_camera()->set_path(getOptional(yaml["camera"])["path"].as<std::string>("/dev/video" + std::to_string(config.camera().id())));
    config.mutable_camera()->set_width(getOptional(yaml["camera"])["width"].as<int>(0));
    config.mutable_camera()->set_height(getOptional(yaml["camera"])["height"].as<int>(0));
    config.mutable_camera()->set_exposure(getOptional(yaml["camera"])["exposure"].as<double>(0.0));
    config.mutable_camera()->set_gain(getOptional(yaml["camera"])["gain"].as<double>(0.0));
    config.mutable_camera()->set_gamma(getOptional(yaml["camera"])["gamma"].as<double>(1.0));
    const YAML::Node wb = getOptional(yaml["camera"])["white_balance"].IsDefined() ? getOptional(yaml["camera"])["white_balance"] : YAML::Node();
	if(wb.IsMap()) {
        config.mutable_camera()->set_wb_type(MANUAL);
        config.mutable_camera()->set_wb_blue(wb["blue"].as<double>(1.0));
        config.mutable_camera()->set_wb_red(wb["red"].as<double>(1.0));
	} else {
        config.mutable_camera()->set_wb_type((wb.as<std::string>("OUTDOOR") == "OUTDOOR") ? OUTDOOR : INDOOR);
	}

    config.mutable_geometry()->set_camera_amount(getOptional(yaml["geometry"])["camera_amount"].as<int>(1));
    config.mutable_geometry()->set_camera_height(getOptional(yaml["geometry"])["camera_height"].as<double>(0.0));

    for(auto const& pos : getOptional(getOptional(yaml["geometry"])["line_corners"])) {
        Vector2* v = config.mutable_geometry()->add_line_corners();
        v->set_x(pos[0].as<float>());
        v->set_y(pos[1].as<float>());
    }
    config.mutable_geometry()->set_refinement(getOptional(yaml["geometry"])["refinement"].as<bool>(false));

    config.mutable_thresholds()->set_circularity(getOptional(yaml["thresholds"])["circularity"].as<double>(15.0));
    config.mutable_thresholds()->set_score(getOptional(yaml["thresholds"])["score"].as<double>(5.0));
    config.mutable_thresholds()->set_min_confidence(getOptional(yaml["thresholds"])["min_confidence"].as<float>(0.2f));
    config.mutable_thresholds()->set_min_cam_edge_distance(getOptional(yaml["thresholds"])["min_cam_edge_distance"].as<double>(170.0));
    config.mutable_thresholds()->set_clipping_tolerance(getOptional(yaml["thresholds"])["clipping_tolerance"].as<float>(10.0f));
    config.mutable_thresholds()->set_geometry_tolerance(getOptional(yaml["thresholds"])["geometry_tolerance"].as<float>(10.0f));

    config.mutable_color()->set_reference_force(getOptional(yaml["color"])["reference_force"].as<float>(0.1f));
    config.mutable_color()->set_history_force(getOptional(yaml["color"])["history_force"].as<float>(0.7f));
    set_color(config.mutable_color()->mutable_orange(), getOptional(getOptional(yaml["color"])["orange"]), 192, 128, 64);
    set_color(config.mutable_color()->mutable_field(), getOptional(getOptional(yaml["color"])["field"]), 128, 128, 128);
    set_color(config.mutable_color()->mutable_yellow(), getOptional(getOptional(yaml["color"])["yellow"]), 255, 128, 0);
    set_color(config.mutable_color()->mutable_blue(), getOptional(getOptional(yaml["color"])["blue"]), 0, 128, 255);
    set_color(config.mutable_color()->mutable_green(), getOptional(getOptional(yaml["color"])["green"]), 0, 255, 128);
    set_color(config.mutable_color()->mutable_pink(), getOptional(getOptional(yaml["color"])["pink"]), 255, 0, 128);

    config.mutable_network()->set_gc_ip(getOptional(yaml["network"])["gc_ip"].as<std::string>("224.5.23.1"));
    config.mutable_network()->set_gc_port(getOptional(yaml["network"])["gc_port"].as<int>(10003));
    config.mutable_network()->set_vision_ip(getOptional(yaml["network"])["vision_ip"].as<std::string>("224.5.23.2"));

    config.mutable_stream()->set_active(getOptional(yaml["stream"])["active"].as<bool>(true));
    config.mutable_stream()->set_stream_ip(getOptional(yaml["stream"])["ip_base_prefix"].as<std::string>("224.5.23.") + std::to_string(getOptional(yaml["stream"])["ip_base_end"].as<int>(100) + config.camera_id()));
    config.mutable_stream()->set_stream_port(getOptional(yaml["stream"])["port"].as<int>(10100));
}

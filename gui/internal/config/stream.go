package config

import (
	"fmt"
	"net"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Stream is where a camera's vision_processor sends its live H.264 video, from
// its config.yml-layout stream: block. vision_processor builds the address
// as ip_base_prefix + (ip_base_end + cam_id) : port (src/Resources.cpp).
type Stream struct {
	Active  bool
	Address string
}

// streamBlock is the YAML shape of a stream: block. Pointers tell a missing
// key (use vision_processor's fallback) from a zero one.
type streamBlock struct {
	Active       *bool   `yaml:"active"`
	IPBasePrefix *string `yaml:"ip_base_prefix"`
	IPBaseEnd    *int    `yaml:"ip_base_end"`
	Port         *int    `yaml:"port"`
}

// Stream is the camera with id's stream, from its effective config. ok is
// false if there's no such camera.
func (d Document) Stream(id int) (s Stream, ok bool, err error) {
	c := d.camera(id)
	if c == nil {
		return Stream{}, false, nil
	}

	var parsed streamBlock

	if block := d.Effective(*c)["stream"]; block != nil {
		data, err := yaml.Marshal(block)
		if err != nil {
			return Stream{}, true, err
		}

		if err := yaml.Unmarshal(data, &parsed); err != nil {
			return Stream{}, true, &ValidationError{fmt.Errorf("cameras[%d].stream: %w", id, err)}
		}
	}

	prefix, end, port := "224.5.23.", 100, 10100

	if parsed.IPBasePrefix != nil {
		prefix = *parsed.IPBasePrefix
	}

	if parsed.IPBaseEnd != nil {
		end = *parsed.IPBaseEnd
	}

	if parsed.Port != nil {
		port = *parsed.Port
	}

	return Stream{
		Active:  parsed.Active == nil || *parsed.Active,
		Address: net.JoinHostPort(prefix+strconv.Itoa(end+id), strconv.Itoa(port)),
	}, true, nil
}

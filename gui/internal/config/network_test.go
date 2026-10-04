package config

import (
	"strings"
	"testing"
)

func TestNetworkFallsBackToVisionProcessorDefaults(t *testing.T) {
	doc := loadFixture(t)
	delete(doc.Defaults, "network")

	n, err := doc.Network()
	if err != nil {
		t.Fatalf("Network: %v", err)
	}

	if n.VisionAddress() != "224.5.23.2:10006" || n.GCAddress() != "224.5.23.1:10003" {
		t.Fatalf("addresses = %s / %s, want the vision_processor defaults", n.VisionAddress(), n.GCAddress())
	}
}

func TestNetworkReadsPartialOverrides(t *testing.T) {
	doc := loadFixture(t)
	doc.Defaults["network"] = map[string]any{"vision_ip": "224.5.23.9", "gc_port": 11003}

	n, err := doc.Network()
	if err != nil {
		t.Fatalf("Network: %v", err)
	}

	if n.VisionAddress() != "224.5.23.9:10006" || n.GCAddress() != "224.5.23.1:11003" {
		t.Fatalf("addresses = %s / %s, want overrides merged over defaults", n.VisionAddress(), n.GCAddress())
	}
}

func TestValidateRejectsBadNetwork(t *testing.T) {
	cases := map[string]struct {
		block map[string]any
		want  string
	}{
		"unspecified": {map[string]any{"vision_ip": "0.0.0.0"}, "network.vision_ip"},
		"not an ip":   {map[string]any{"gc_ip": "224.5.23"}, "network.gc_ip"},
		"ipv6":        {map[string]any{"gc_ip": "ff02::1"}, "network.gc_ip"},
		"port range":  {map[string]any{"vision_port": 70000}, "network.vision_port"},
		"zero port":   {map[string]any{"gc_port": 0}, "network.gc_port"},
		"shared":      {map[string]any{"gc_ip": "224.5.23.2", "gc_port": 10006}, "can't share"},
		"wrong type":  {map[string]any{"vision_port": "ten"}, "network"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc := loadFixture(t)
			doc.Defaults["network"] = tc.block

			err := doc.Validate()
			if err == nil || !IsValidation(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want a validation error mentioning %q", err, tc.want)
			}
		})
	}
}

func TestValidateChecksACameraNetworkOverride(t *testing.T) {
	doc := loadFixture(t)
	doc.Cameras[0].Config["network"] = map[string]any{"vision_port": -1}

	err := doc.Validate()
	if err == nil || !strings.Contains(err.Error(), "cameras[0]: network.vision_port") {
		t.Fatalf("Validate = %v, want the camera's network override rejected", err)
	}
}

func TestValidateAcceptsBroadcastAndUnicast(t *testing.T) {
	for _, ip := range []string{"255.255.255.255", "192.168.30.255", "10.0.0.5"} {
		doc := loadFixture(t)
		doc.Defaults["network"] = map[string]any{"vision_ip": ip}

		if err := doc.Validate(); err != nil {
			t.Errorf("%s: Validate = %v, want it accepted (the GUI warns instead)", ip, err)
		}
	}
}

func TestInterfaceSelectionDefaultsToAuto(t *testing.T) {
	doc := loadFixture(t)

	if auto, skip := doc.InterfaceSelection(); !auto || skip != nil {
		t.Fatalf("selection = %v %v, want auto with no skip list", auto, skip)
	}

	manual := false
	doc.Host = &Host{Interfaces: &Interfaces{Auto: &manual, Skip: []string{"docker0"}}}

	if auto, skip := doc.InterfaceSelection(); auto || len(skip) != 1 || skip[0] != "docker0" {
		t.Fatalf("selection = %v %v, want manual skipping docker0", auto, skip)
	}
}

func TestHostRoundTripsAndDiffsAsNetwork(t *testing.T) {
	before := loadFixture(t)
	after := before.clone()

	manual := false
	after.Host = &Host{Interfaces: &Interfaces{Auto: &manual, Skip: []string{"docker0"}}}

	data, err := Marshal(after)
	if err != nil {
		t.Fatalf("Marshal: %v", err)
	}

	parsed, err := Parse(data)
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}

	if auto, skip := parsed.InterfaceSelection(); auto || len(skip) != 1 {
		t.Fatalf("round trip lost the selection: %v %v", auto, skip)
	}

	changes := Diff(before, after)
	if len(changes) != 1 || changes[0].Section != SectionNetwork {
		t.Fatalf("changes = %+v, want one network change", changes)
	}
}

func TestStreamAddressMatchesVisionProcessor(t *testing.T) {
	doc := loadFixture(t)

	s, ok, err := doc.Stream(1)
	if err != nil || !ok {
		t.Fatalf("Stream(1) = %v %v", ok, err)
	}

	if !s.Active || s.Address != "224.5.23.101:10100" {
		t.Fatalf("stream = %+v, want active on 224.5.23.101:10100", s)
	}

	doc.Defaults["stream"] = map[string]any{"active": false, "ip_base_prefix": "239.1.1.", "ip_base_end": 10, "port": 20000}

	if s, _, _ := doc.Stream(1); s.Active || s.Address != "239.1.1.11:20000" {
		t.Fatalf("stream = %+v, want inactive on 239.1.1.11:20000", s)
	}

	if _, ok, _ := doc.Stream(99); ok {
		t.Fatal("Stream(99) found a camera that doesn't exist")
	}
}

package config

import "testing"

func TestValidName(t *testing.T) {
	for _, name := range []string{"vision.yml", "vision.yaml", "field-b.vision.yml", "my config.yml"} {
		if err := ValidName(name); err != nil {
			t.Errorf("ValidName(%q) = %v, want nil", name, err)
		}
	}

	for _, name := range []string{
		"", ".", "..", "../x.yml", "/etc/x.yml", "a/b.yml", "./x.yml", `a\b.yml`, "x\x00.yml",
		".hidden.yml", ".yml", "x.txt", "x", "x.yml/",
	} {
		if err := ValidName(name); err == nil {
			t.Errorf("ValidName(%q) = nil, want an error", name)
		}
	}
}

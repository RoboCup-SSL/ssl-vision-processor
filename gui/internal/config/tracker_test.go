package config

import (
	"strings"
	"testing"
)

func TestTrackerFallsBackToTheStandardGroup(t *testing.T) {
	doc := loadFixture(t)
	doc.Tracker = nil

	if got := doc.TrackerAddress(); got != "224.5.23.2:10010" {
		t.Fatalf("TrackerAddress = %s, want the standard group", got)
	}

	port := 10020
	doc.Tracker = &Tracker{Port: &port}

	if got := doc.TrackerAddress(); got != "224.5.23.2:10020" {
		t.Fatalf("TrackerAddress = %s, want the port override over the standard IP", got)
	}
}

func TestValidateRejectsBadTracker(t *testing.T) {
	str := func(s string) *string { return &s }
	num := func(n int) *int { return &n }

	cases := map[string]struct {
		tracker Tracker
		want    string
	}{
		"not an ip":   {Tracker{IP: str("224.5.23")}, "tracker.ip"},
		"unspecified": {Tracker{IP: str("0.0.0.0")}, "tracker.ip"},
		"port range":  {Tracker{Port: num(70000)}, "tracker.port"},
		"vision":      {Tracker{Port: num(10006)}, "with vision"},
		"gc":          {Tracker{IP: str("224.5.23.1"), Port: num(10003)}, "with the game controller"},
	}

	for name, tc := range cases {
		t.Run(name, func(t *testing.T) {
			doc := loadFixture(t)
			delete(doc.Defaults, "network")
			doc.Tracker = &tc.tracker

			err := doc.Validate()
			if err == nil || !IsValidation(err) || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("Validate = %v, want a validation error mentioning %q", err, tc.want)
			}
		})
	}
}

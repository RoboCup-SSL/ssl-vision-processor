package main

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func TestSystemDescribesTheHost(t *testing.T) {
	h, _ := testServer(t)

	var got systemResponse
	if err := json.NewDecoder(do(t, h, http.MethodGet, "/api/system", nil).Body).Decode(&got); err != nil {
		t.Fatal(err)
	}

	if got.Host.OS != runtime.GOOS || got.Host.CPUs < 1 || got.Build.GoVersion != runtime.Version() {
		t.Errorf("host = %+v, build = %+v", got.Host, got.Build)
	}

	if got.Process.PID != os.Getpid() || got.Process.Started == "" {
		t.Errorf("process = %+v", got.Process)
	}

	// Every host has at least loopback.
	if len(got.Interfaces) == 0 {
		t.Error("no interfaces")
	}
}

func TestOSReleaseReadsQuotedAndBareValues(t *testing.T) {
	path := filepath.Join(t.TempDir(), "os-release")
	data := "NAME=Ubuntu\nPRETTY_NAME=\"Ubuntu 22.04.4 LTS\"\nID=ubuntu\n"

	if err := os.WriteFile(path, []byte(data), 0o644); err != nil {
		t.Fatal(err)
	}

	for key, want := range map[string]string{"PRETTY_NAME": "Ubuntu 22.04.4 LTS", "ID": "ubuntu", "VERSION": ""} {
		if got := osRelease(path, key); got != want {
			t.Errorf("osRelease(%s) = %q, want %q", key, got, want)
		}
	}

	if got := osRelease(filepath.Join(t.TempDir(), "missing"), "ID"); got != "" {
		t.Errorf("missing file = %q, want empty", got)
	}
}

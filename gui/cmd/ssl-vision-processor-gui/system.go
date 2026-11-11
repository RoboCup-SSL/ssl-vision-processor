package main

import (
	"bufio"
	"net"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/buildinfo"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
)

// started is when the host started, near enough: package variables are set
// before main runs.
var started = time.Now()

// localeVars are the environment variables that pick the host's language and
// formats, in the order POSIX looks at them.
var localeVars = []string{"LC_ALL", "LC_MESSAGES", "LC_CTYPE", "LC_NUMERIC", "LANG", "LANGUAGE"}

// systemResponse is what a bug report needs to know about this host and its
// build.
type systemResponse struct {
	Build      buildinfo.Info `json:"build"`
	Host       hostInfo       `json:"host"`
	Process    processInfo    `json:"process"`
	Interfaces []netInterface `json:"interfaces"`
	// Log is the host's last 50 warnings and errors since it started, oldest
	// first, as log file lines.
	Log []string `json:"log"`
}

// hostInfo is the machine the host runs on.
type hostInfo struct {
	// OS and Arch are Go's names, such as linux and amd64.
	OS   string `json:"os"`
	Arch string `json:"arch"`
	// Distribution is os-release's PRETTY_NAME, Kernel the kernel release;
	// empty where the OS doesn't have them.
	Distribution string `json:"distribution"`
	Kernel       string `json:"kernel"`
	Hostname     string `json:"hostname"`
	CPUs         int    `json:"cpus"`
	// TimeZone is the host's local zone, such as Local or Europe/Berlin.
	TimeZone string `json:"timeZone"`
	// Locale lists each locale environment variable that is set, as
	// NAME=value.
	Locale []string `json:"locale"`
}

// processInfo is this run of the host.
type processInfo struct {
	PID int `json:"pid"`
	// Args are the command-line flags it was started with.
	Args []string `json:"args"`
	// Started is when it started, RFC 3339.
	Started string `json:"started"`
	Root    bool   `json:"root"`
}

// netInterface is one of the host's network interfaces, every address
// included, unlike the Network page's list.
type netInterface struct {
	Name string `json:"name"`
	// Flags are Go's names for the interface flags: up, broadcast,
	// loopback, pointtopoint, multicast, running.
	Flags     []string `json:"flags"`
	MTU       int      `json:"mtu"`
	Addresses []string `json:"addresses"`
}

// system describes this host and the build it runs, for a bug report: the
// build's git state, the OS, the process, every network interface, and its
// latest warnings and errors.
func (s *VisionServer) system(*http.Request, none) (systemResponse, error) {
	hostname, _ := os.Hostname()

	return systemResponse{
		Build: buildinfo.Get(),
		Host: hostInfo{
			OS:           runtime.GOOS,
			Arch:         runtime.GOARCH,
			Distribution: osRelease("/etc/os-release", "PRETTY_NAME"),
			Kernel:       readTrimmed("/proc/sys/kernel/osrelease"),
			Hostname:     hostname,
			CPUs:         runtime.NumCPU(),
			TimeZone:     timeZone(),
			Locale:       locale(),
		},
		Process: processInfo{
			PID:     os.Getpid(),
			Args:    nonNil(os.Args[1:]),
			Started: started.Format(time.RFC3339),
			Root:    os.Geteuid() == 0,
		},
		Interfaces: interfaces(),
		Log:        logging.Recent(),
	}, nil
}

// readTrimmed is a small file's contents, empty if it can't be read.
func readTrimmed(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	return strings.TrimSpace(string(data))
}

// osRelease is one key's value from an os-release file, empty if missing.
func osRelease(path, key string) string {
	f, err := os.Open(path)
	if err != nil {
		return ""
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		if value, ok := strings.CutPrefix(scanner.Text(), key+"="); ok {
			return strings.Trim(value, `"'`)
		}
	}

	return ""
}

// timeZone is the local zone's name: TZ if set, else the zoneinfo file
// /etc/localtime links to, else Go's name for it.
func timeZone() string {
	if tz := os.Getenv("TZ"); tz != "" {
		return tz
	}

	if target, err := os.Readlink("/etc/localtime"); err == nil {
		if _, zone, ok := strings.Cut(target, "zoneinfo/"); ok {
			return zone
		}
	}

	return time.Local.String()
}

func locale() []string {
	out := []string{}

	for _, name := range localeVars {
		if value := os.Getenv(name); value != "" {
			out = append(out, name+"="+value)
		}
	}

	return out
}

func interfaces() []netInterface {
	out := []netInterface{}

	ifis, err := net.Interfaces()
	if err != nil {
		return out
	}

	for _, ifi := range ifis {
		i := netInterface{Name: ifi.Name, Flags: []string{}, MTU: ifi.MTU, Addresses: []string{}}

		if ifi.Flags != 0 {
			i.Flags = strings.Split(ifi.Flags.String(), "|")
		}

		if addrs, err := ifi.Addrs(); err == nil {
			for _, addr := range addrs {
				i.Addresses = append(i.Addresses, addr.String())
			}
		}

		out = append(out, i)
	}

	return out
}

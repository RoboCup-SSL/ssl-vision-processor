// Package buildinfo says what this binary was built from: the git state of
// this repository and of its proto submodule, stamped at build time by the
// Makefile (BUILD_LDFLAGS, scripts/build-ldflags.sh), and the Go toolchain.
package buildinfo

import (
	"runtime"
	"runtime/debug"
)

// Set with -ldflags -X, which only sets strings. Left empty by a build
// without the Makefile's flags.
var (
	version = "dev"

	repoCommit, repoBranch, repoTag, repoDirty     string
	protoCommit, protoBranch, protoTag, protoDirty string
)

// Git is one repository's state when the binary was built. An empty Commit
// means it isn't known, and then neither is anything else.
type Git struct {
	Commit string `json:"commit"`
	// Branch is empty for a detached HEAD, as a submodule usually is.
	Branch string `json:"branch"`
	// Tag is a tag on Commit itself, if any.
	Tag string `json:"tag"`
	// Dirty means tracked files differed from Commit, as git describe
	// --dirty counts it: untracked files don't count.
	Dirty bool `json:"dirty"`
}

// Info is the binary's build.
type Info struct {
	// Version is git describe's name for the build, "dev" without the
	// Makefile's flags.
	Version string `json:"version"`
	// Repo is this repository, ssl-vision-processor.
	Repo Git `json:"repo"`
	// Proto is the ssl-protocol-defs submodule the protobuf bindings were
	// generated from.
	Proto Git `json:"proto"`
	// Stamped is false for a build without the Makefile's flags (a plain go
	// build): Repo then comes from Go's own VCS stamp, which has no branch or
	// tag, and Proto isn't known.
	Stamped   bool   `json:"stamped"`
	GoVersion string `json:"goVersion"`
}

// Version is the build's version, for -version and the log.
func Version() string { return version }

// Get returns the binary's build.
func Get() Info {
	info := Info{
		Version:   version,
		Repo:      git(repoCommit, repoBranch, repoTag, repoDirty),
		Proto:     git(protoCommit, protoBranch, protoTag, protoDirty),
		Stamped:   repoCommit != "",
		GoVersion: runtime.Version(),
	}

	if !info.Stamped {
		info.Repo = goStamp()
	}

	return info
}

func git(commit, branch, tag, dirty string) Git {
	return Git{Commit: commit, Branch: branch, Tag: tag, Dirty: dirty == "true"}
}

// goStamp is what go build records about the checkout it ran in.
func goStamp() Git {
	var g Git

	bi, ok := debug.ReadBuildInfo()
	if !ok {
		return g
	}

	for _, s := range bi.Settings {
		switch s.Key {
		case "vcs.revision":
			g.Commit = s.Value
		case "vcs.modified":
			g.Dirty = s.Value == "true"
		}
	}

	return g
}

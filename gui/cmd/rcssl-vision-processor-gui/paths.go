package main

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

// appDir is this tool's folder under the league's rcssl namespace, in the
// user's config and state directories.
var appDir = filepath.Join("rcssl", "vision-processor-gui")

// pathFlags are the path flags as given. A nil pointer is a flag not given,
// where a given empty -logFile means no log file.
type pathFlags struct {
	configDir  string
	configFile string
	logFile    *string
	imgDir     *string
	// runCameras is -runCameras: the host runs the vision_processors, each in
	// its own directory under runDir.
	runCameras bool
}

// paths are every location the host uses, absolute.
type paths struct {
	configDir string
	// configFile is a file name in configDir.
	configFile string
	// logFile is empty for no log file.
	logFile string
	imgDir  string
	// runDir holds each camera's vision_processor working directory, empty
	// without -runCameras.
	runDir string
}

// config is the full path of the config file.
func (p paths) config() string {
	return filepath.Join(p.configDir, p.configFile)
}

// resolvePaths applies the defaults to f and makes everything absolute. It runs
// before logging starts, since the log file is one of its results.
func resolvePaths(f pathFlags) (paths, error) {
	if err := config.ValidName(f.configFile); err != nil {
		return paths{}, fmt.Errorf("-configFile: %w", err)
	}

	p := paths{configFile: f.configFile}

	var err error

	if f.configDir == "" {
		base, err := os.UserConfigDir()
		if err != nil {
			return paths{}, fmt.Errorf("no default config folder, set -configDir: %w", err)
		}

		p.configDir = filepath.Join(base, appDir)
	} else if p.configDir, err = filepath.Abs(f.configDir); err != nil {
		return paths{}, fmt.Errorf("-configDir: %w", err)
	}

	if p.logFile, err = resolveLogFile(f.logFile); err != nil {
		return paths{}, err
	}

	if p.imgDir, err = resolveImgDir(f.imgDir); err != nil {
		return paths{}, err
	}

	if f.runCameras {
		base, err := userStateDir()
		if err != nil {
			return paths{}, fmt.Errorf("no folder to run the cameras in, set $XDG_STATE_HOME: %w", err)
		}

		p.runDir = filepath.Join(base, appDir, "run")
	}

	return p, nil
}

func resolveLogFile(flag *string) (string, error) {
	if flag != nil {
		if *flag == "" {
			return "", nil
		}

		return filepath.Abs(*flag)
	}

	base, err := userStateDir()
	if err != nil {
		return "", fmt.Errorf("no default log file, set -logFile: %w", err)
	}

	return filepath.Join(base, appDir, "logs", "vision-processor-gui.log"), nil
}

func resolveImgDir(flag *string) (string, error) {
	if flag != nil && *flag != "" {
		return filepath.Abs(*flag)
	}

	base, err := userStateDir()
	if err != nil {
		return "", fmt.Errorf("no default image directory, set -imgDir: %w", err)
	}

	return filepath.Join(base, appDir, "img"), nil
}

// userStateDir is $XDG_STATE_HOME, else ~/.local/state: os.UserConfigDir's
// rules, for the directory the standard library has no function for.
func userStateDir() (string, error) {
	if dir := os.Getenv("XDG_STATE_HOME"); dir != "" {
		if !filepath.IsAbs(dir) {
			return "", errors.New("path in $XDG_STATE_HOME is relative")
		}

		return dir, nil
	}

	home := os.Getenv("HOME")
	if home == "" {
		return "", errors.New("neither $XDG_STATE_HOME nor $HOME are defined")
	}

	return filepath.Join(home, ".local", "state"), nil
}

// logPaths says where everything is. Most users won't know these locations, so
// they're printed on every start rather than left to the documentation.
func logPaths(p paths) {
	slog.Info("paths", "configDir", p.configDir, "configFile", p.configFile)

	if p.logFile == "" {
		slog.Info("paths", "logFile", "disabled")
	} else {
		slog.Info("paths", "logFile", p.logFile)
	}

	slog.Info("paths", "imgDir", p.imgDir)

	if p.runDir != "" {
		slog.Info("paths", "runDir", p.runDir)
	}
}

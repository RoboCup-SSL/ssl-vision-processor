package config

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// ValidName reports whether name can name a file in the config folder: one
// path component, not hidden, ending in .yml or .yaml. The folder is flat so
// that nothing reaching the host, the API above all, can write anywhere else.
func ValidName(name string) error {
	if name == "" {
		return fmt.Errorf("file name is required")
	}

	if filepath.Base(name) != name || strings.ContainsAny(name, "/\\\x00") {
		return fmt.Errorf("%q: want a file name in the config folder, not a path", name)
	}

	if strings.HasPrefix(name, ".") {
		return fmt.Errorf("%q: a hidden file can't be a config", name)
	}

	if !slices.Contains([]string{".yml", ".yaml"}, filepath.Ext(name)) {
		return fmt.Errorf("%q: want a .yml or .yaml file", name)
	}

	return nil
}

// resolve is the full path of the file name names in dir, refusing anything
// that isn't a plain file there. Symlinks are refused so that reading or
// replacing one can't reach outside the folder.
func resolve(dir, name string) (string, error) {
	if err := ValidName(name); err != nil {
		return "", &ValidationError{err}
	}

	path := filepath.Join(dir, name)

	if info, err := os.Lstat(path); err == nil && !info.Mode().IsRegular() {
		return "", &ValidationError{fmt.Errorf("%q: not a regular file", name)}
	}

	return path, nil
}

// validateFileNames checks every file the document names is in the config
// folder. The document can arrive from the API, so without this it could write
// a generated config.yml anywhere the host can.
func (d Document) validateFileNames() error {
	var errs error

	heights := func(prefix string, s Settings) {
		if name, ok := s["bot_heights_file"].(string); ok {
			errs = errors.Join(errs, prefixed(prefix+".bot_heights_file", ValidName(name)))
		}
	}

	heights("defaults", d.Defaults)

	// Two cameras writing one file would each overwrite the other's settings.
	users := map[string]int{}

	for _, c := range d.Cameras {
		prefix := fmt.Sprintf("cameras[%d]", c.CameraID)
		heights(prefix+".config", c.Config)

		if c.ConfigPath == "" {
			continue
		}

		errs = errors.Join(errs, prefixed(prefix+".config_path", ValidName(c.ConfigPath)))

		if c.ConfigPath == d.BotHeightsFile() {
			errs = errors.Join(errs, fmt.Errorf("%s.config_path: %q is the robot height table", prefix, c.ConfigPath))
		}

		if other, ok := users[c.ConfigPath]; ok {
			errs = errors.Join(errs, fmt.Errorf("%s.config_path: %q is also cameras[%d]'s", prefix, c.ConfigPath, other))
		} else {
			users[c.ConfigPath] = c.CameraID
		}
	}

	return errs
}

// AssignConfigPaths gives every camera without a config_path one of its own:
// cam<id>.yml, or cam<id>-2.yml and so on when that name is another camera's,
// the robot height table, or ownFile, the config file itself. For a host that
// runs every camera's vision_processor, which each need a file to read.
func (d *Document) AssignConfigPaths(ownFile string) {
	taken := map[string]bool{d.BotHeightsFile(): true, ownFile: true}

	for _, c := range d.Cameras {
		taken[c.ConfigPath] = true
	}

	for i := range d.Cameras {
		c := &d.Cameras[i]
		if c.ConfigPath != "" {
			continue
		}

		name := fmt.Sprintf("cam%d.yml", c.CameraID)
		for n := 2; taken[name]; n++ {
			name = fmt.Sprintf("cam%d-%d.yml", c.CameraID, n)
		}

		c.ConfigPath = name
		taken[name] = true
	}
}

// writesOver reports whether saving doc as name would overwrite a file doc
// itself uses for something else.
func (d Document) writesOver(name string) bool {
	if name == d.BotHeightsFile() {
		return true
	}

	return slices.ContainsFunc(d.Cameras, func(c Camera) bool { return c.ConfigPath == name })
}

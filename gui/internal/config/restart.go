package config

import (
	"reflect"
	"slices"
)

// RestartKeys lists the settings that differ between two of a camera's
// rendered config.yml contents (Document.Rendered) and that vision_processor
// reads only at startup: everything but live and host-only settings. Unknown
// keys count, since nothing says vision_processor reloads them.
func RestartKeys(before, after map[string]any) []string {
	paths := map[string]bool{}

	for _, m := range []map[string]any{before, after} {
		for _, p := range settingPaths(m) {
			paths[p] = true
		}
	}

	var keys []string

	for p := range paths {
		b, _ := lookup(before, p)
		a, _ := lookup(after, p)

		if reflect.DeepEqual(b, a) {
			continue
		}

		if s, ok := settings[p]; ok && (s.Apply == ApplyLive || s.Apply == ApplyHost) {
			continue
		}

		keys = append(keys, p)
	}

	slices.Sort(keys)

	return keys
}

// settingPaths is every setting in m as a dotted path: block.key for the
// keys of a block, the key alone for a top-level value.
func settingPaths(m map[string]any) []string {
	var paths []string

	for k, v := range m {
		block, ok := v.(map[string]any)
		if !ok {
			paths = append(paths, k)

			continue
		}

		for key := range block {
			paths = append(paths, k+"."+key)
		}
	}

	return paths
}

package geometry

import (
	"fmt"
	"os"
	"regexp"
	"strings"
	"sync"

	"gopkg.in/yaml.v3"
)

// instanceConfigMu serializes every WriteLineCorners/ReadLineCorners call.
// Unlike Geometry, there's no per-instance object to hang a mutex off of --
// this is a bare read-modify-write of a file on disk -- so a single
// package-level lock is what keeps two concurrent PUT /api/config/line-corners
// requests (two browser tabs, a double-click before the Save button disables)
// from each doing their own unsynchronized os.ReadFile/os.WriteFile and
// silently clobbering one another, and keeps a concurrent read from ever
// seeing a half-written file. Matches this package's existing single,
// same-host-instance assumption (see gui/CLAUDE.md) -- one config.yml, one
// lock.
var instanceConfigMu sync.Mutex

// Corner is one point in image pixel coordinates, as produced by the corner
// picker. Only the first element of a slice passed to WriteLineCorners is
// meaningful to src/calib/GeomModel.cpp's cornerCalibration -- it must be the
// field's (minX, minY) corner. The other three may be in any order.
type Corner struct {
	X int `json:"x"`
	Y int `json:"y"`
}

var (
	geometryHeader   = regexp.MustCompile(`^geometry:\s*$`)
	lineCornersKey   = regexp.MustCompile(`^(\s*)line_corners:\s*$`)
	goalSideMarkerKV = regexp.MustCompile(`^(\s*)goal_side_marker:.*$`)
)

// WriteLineCorners updates geometry.line_corners in the vision_processor
// instance config at path, in place, along with a goal_side_marker record of
// which corner-picker marker (1-4, matching its on-screen number) was chosen
// as the goal-side corner -- vision_processor itself never reads
// goal_side_marker (yaml-cpp's Node["key"] lookups ignore unknown keys), it's
// purely a record for whoever looks at this file next.
//
// A bad corners/goalSideMarker argument is reported as a *ValidationError
// (the caller's mistake), same type UpdateField/SaveAs use for the same
// purpose -- callers use errors.As to give it a 400 instead of a 500, so this
// bounds check only has to live here, not also at whatever HTTP handler
// calls in.
//
// This edits the text directly rather than decoding/re-encoding the file as
// YAML (contrast Geometry.saveYAML for geometry.yml): config.yml ships full
// of comments and commented-out example values meant to be hand-read, and
// gopkg.in/yaml.v3 does not reliably preserve those across a decode/re-encode
// round trip (confirmed while building this -- entire comment blocks
// elsewhere in the file were dropped and indentation drifted). A line-based
// splice touches nothing outside the two keys it's asked to set.
func WriteLineCorners(path string, corners []Corner, goalSideMarker int) error {
	if len(corners) == 0 {
		return &ValidationError{fmt.Errorf("no corners given")}
	}

	if goalSideMarker < 1 || goalSideMarker > len(corners) {
		return &ValidationError{fmt.Errorf("goalSideMarker must identify one of the %d corners, got %d", len(corners), goalSideMarker)}
	}

	instanceConfigMu.Lock()
	defer instanceConfigMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return fmt.Errorf("read %s: %w", path, err)
	}

	updated, err := spliceLineCorners(string(data), corners)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	updated, err = spliceGoalSideMarker(updated, goalSideMarker)
	if err != nil {
		return fmt.Errorf("%s: %w", path, err)
	}

	if err := os.WriteFile(path, []byte(updated), 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// instanceConfig mirrors just the two geometry fields WriteLineCorners
// writes. Unlike WriteLineCorners, reading doesn't need to preserve
// anything -- nothing gets written back -- so a plain yaml.Unmarshal is fine
// here even though it isn't for the write path.
type instanceConfig struct {
	Geometry struct {
		LineCorners    [][]int `yaml:"line_corners"`
		GoalSideMarker int     `yaml:"goal_side_marker"`
	} `yaml:"geometry"`
}

// ReadLineCorners reads back whatever WriteLineCorners last saved to path.
// Returns a nil/empty corners slice and a zero marker (not an error) if the
// file has no line_corners yet -- that's the normal, not-yet-calibrated
// state, not a failure.
func ReadLineCorners(path string) ([]Corner, int, error) {
	instanceConfigMu.Lock()
	defer instanceConfigMu.Unlock()

	data, err := os.ReadFile(path)
	if err != nil {
		return nil, 0, fmt.Errorf("read %s: %w", path, err)
	}

	var cfg instanceConfig
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, 0, fmt.Errorf("parse %s: %w", path, err)
	}

	corners := make([]Corner, len(cfg.Geometry.LineCorners))

	for i, pair := range cfg.Geometry.LineCorners {
		if len(pair) != 2 {
			return nil, 0, fmt.Errorf("%s: geometry.line_corners[%d] has %d values, want 2", path, i, len(pair))
		}

		corners[i] = Corner{X: pair[0], Y: pair[1]}
	}

	return corners, cfg.Geometry.GoalSideMarker, nil
}

// geometrySectionBounds finds the 'geometry:' top-level key and the line
// index where its section ends (the next top-level key, or len(lines)).
func geometrySectionBounds(lines []string) (start, end int, err error) {
	start = -1

	for i, line := range lines {
		if geometryHeader.MatchString(line) {
			start = i

			break
		}
	}

	if start == -1 {
		return 0, 0, fmt.Errorf("missing top-level 'geometry' section")
	}

	end = len(lines)

	for i := start + 1; i < len(lines); i++ {
		if isTopLevelKeyLine(lines[i]) {
			end = i

			break
		}
	}

	return start, end, nil
}

// spliceLineCorners is the pure text transform behind WriteLineCorners' list
// value, split out so it's testable without touching disk.
func spliceLineCorners(doc string, corners []Corner) (string, error) {
	lines, trailingNewline := splitDoc(doc)

	geometryAt, sectionEnd, err := geometrySectionBounds(lines)
	if err != nil {
		return "", err
	}

	keyAt := -1
	indent := ""

	for i := geometryAt + 1; i < sectionEnd; i++ {
		if m := lineCornersKey.FindStringSubmatch(lines[i]); m != nil {
			keyAt = i
			indent = m[1]

			break
		}
	}

	items := make([]string, len(corners))

	if keyAt == -1 {
		indent = firstChildIndent(lines[geometryAt+1 : sectionEnd])

		for i, c := range corners {
			items[i] = fmt.Sprintf("%s- [%d, %d]", indent, c.X, c.Y)
		}

		out := make([]string, 0, len(lines)+len(items)+1)
		out = append(out, lines[:geometryAt+1]...)
		out = append(out, indent+"line_corners:")
		out = append(out, items...)
		out = append(out, lines[geometryAt+1:]...)

		return joinLines(out, trailingNewline), nil
	}

	// Consume any existing list items (real or commented-out placeholders)
	// immediately following the key -- everything after that, including
	// blank lines and the next section's comments, is left untouched.
	itemLine := regexp.MustCompile(`^` + regexp.QuoteMeta(indent) + `#?-`)
	end := keyAt + 1
	for end < sectionEnd && itemLine.MatchString(lines[end]) {
		end++
	}

	for i, c := range corners {
		items[i] = fmt.Sprintf("%s- [%d, %d]", indent, c.X, c.Y)
	}

	out := make([]string, 0, len(lines)+len(items))
	out = append(out, lines[:keyAt+1]...)
	out = append(out, items...)
	out = append(out, lines[end:]...)

	return joinLines(out, trailingNewline), nil
}

// spliceGoalSideMarker is the pure text transform for the goal_side_marker
// scalar. A one-line explanatory comment is added the first time the key is
// created; updating an existing value leaves whatever's around it alone.
func spliceGoalSideMarker(doc string, marker int) (string, error) {
	lines, trailingNewline := splitDoc(doc)

	geometryAt, sectionEnd, err := geometrySectionBounds(lines)
	if err != nil {
		return "", err
	}

	for i := geometryAt + 1; i < sectionEnd; i++ {
		if m := goalSideMarkerKV.FindStringSubmatch(lines[i]); m != nil {
			lines[i] = fmt.Sprintf("%sgoal_side_marker: %d", m[1], marker)

			return joinLines(lines, trailingNewline), nil
		}
	}

	indent := firstChildIndent(lines[geometryAt+1 : sectionEnd])
	comment := indent + "# Which corner-picker marker (1-4, its on-screen number) was chosen as the goal-side corner. Not read by vision_processor -- record only."

	out := make([]string, 0, len(lines)+2)
	out = append(out, lines[:sectionEnd]...)
	out = append(out, comment, fmt.Sprintf("%sgoal_side_marker: %d", indent, marker))
	out = append(out, lines[sectionEnd:]...)

	return joinLines(out, trailingNewline), nil
}

// isTopLevelKeyLine reports whether line starts a new root-level YAML key
// (unindented, not blank, not a comment) -- the boundary of the 'geometry'
// section.
func isTopLevelKeyLine(line string) bool {
	if line == "" {
		return false
	}

	return line[0] != ' ' && line[0] != '\t' && line[0] != '#'
}

// firstChildIndent finds the leading whitespace of the first real (non-blank,
// non-comment) line in a section, for a key that needs to be appended fresh.
// Falls back to two spaces -- the indent every shipped config.yml template
// uses -- if the section has no other real keys yet.
func firstChildIndent(sectionLines []string) string {
	for _, line := range sectionLines {
		trimmed := strings.TrimSpace(line)
		if trimmed == "" || strings.HasPrefix(trimmed, "#") {
			continue
		}

		return line[:len(line)-len(strings.TrimLeft(line, " \t"))]
	}

	return "  "
}

func splitDoc(doc string) (lines []string, trailingNewline bool) {
	return strings.Split(strings.TrimSuffix(doc, "\n"), "\n"), strings.HasSuffix(doc, "\n")
}

func joinLines(lines []string, trailingNewline bool) string {
	out := strings.Join(lines, "\n")
	if trailingNewline {
		out += "\n"
	}

	return out
}

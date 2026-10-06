// Package referee follows the game controller's referee messages for what
// matters to vision: which teams are playing, and so (from the robot height
// table vision_processor also reads) how tall their robots are.
//
// vision_processor does the same lookup itself (src/udpsocket.cpp,
// GCSocket::parse): a team found in the table sets that color's height; a
// team that isn't keeps whatever height that color had, which is the table's
// mean before any known team has played. This package reports what it can
// tell from outside, so the GUI can flag a team missing from the table.
package referee

import (
	"fmt"
	"os"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

// Teams records the team names from the latest referee message.
// Safe for concurrent use.
type Teams struct {
	mu     sync.Mutex
	yellow string
	blue   string
	at     time.Time
}

func (t *Teams) Record(yellow, blue string, now time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	t.yellow, t.blue, t.at = yellow, blue, now
}

func (t *Teams) snapshot() (yellow, blue string, at time.Time) {
	t.mu.Lock()
	defer t.mu.Unlock()

	return t.yellow, t.blue, t.at
}

// Heights is a robot height table file (team name: height in mm), re-read
// when the file changes. Safe for concurrent use.
type Heights struct {
	mu      sync.Mutex
	path    string
	modTime time.Time
	size    int64
	table   map[string]float64
	err     error
}

// load returns the table at path, reading it again only if the path, size,
// or modification time changed.
func (h *Heights) load(path string) (map[string]float64, error) {
	h.mu.Lock()
	defer h.mu.Unlock()

	info, err := os.Stat(path)
	if err != nil {
		h.path, h.table, h.err = path, nil, err

		return nil, err
	}

	if path == h.path && info.ModTime().Equal(h.modTime) && info.Size() == h.size {
		return h.table, h.err
	}

	h.path, h.modTime, h.size = path, info.ModTime(), info.Size()
	h.table, h.err = readHeights(path)

	return h.table, h.err
}

func readHeights(path string) (map[string]float64, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}

	var table map[string]float64
	if err := yaml.Unmarshal(data, &table); err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	if len(table) == 0 {
		return nil, fmt.Errorf("%s lists no teams", path)
	}

	return table, nil
}

// Team is one side of the current match.
type Team struct {
	Name string `json:"name"`
	// Height is the team's robot height from the table, nil when the team
	// isn't in it.
	Height *float64 `json:"height,omitempty"`
}

// State is what the GUI shows about the current match.
type State struct {
	// Heard is when the last referee message arrived, nil if none has.
	Heard  *time.Time `json:"heard,omitempty"`
	Yellow Team       `json:"yellow"`
	Blue   Team       `json:"blue"`
	// HeightsFile is the table vision_processor reads (bot_heights_file).
	HeightsFile  string `json:"heightsFile"`
	HeightsError string `json:"heightsError,omitempty"`
	// MeanHeight is vision_processor's height for a team not in the table,
	// until a known team has played; MaxHeight is the tallest in the table,
	// which it uses for its field extent checks.
	MeanHeight float64 `json:"meanHeight,omitempty"`
	MaxHeight  float64 `json:"maxHeight,omitempty"`
	// Heights is the whole table, team name to height in mm.
	Heights map[string]float64 `json:"heights,omitempty"`
}

// Describe combines the latest team names with the height table at
// heightsFile.
func Describe(teams *Teams, heights *Heights, heightsFile string) State {
	yellow, blue, at := teams.snapshot()
	state := State{
		Yellow:      Team{Name: yellow},
		Blue:        Team{Name: blue},
		HeightsFile: heightsFile,
	}

	if !at.IsZero() {
		state.Heard = &at
	}

	table, err := heights.load(heightsFile)
	if err != nil {
		state.HeightsError = err.Error()

		return state
	}

	sum := 0.0
	for _, h := range table {
		sum += h
		state.MaxHeight = max(state.MaxHeight, h)
	}

	state.MeanHeight = sum / float64(len(table))
	state.Heights = table

	for _, team := range []*Team{&state.Yellow, &state.Blue} {
		if h, ok := table[team.Name]; ok {
			team.Height = &h
		}
	}

	return state
}

// Package alerts decides every problem the host can see right now: which
// ones, how serious, and the facts behind each. Clients word and link them
// themselves, keyed by Code; Message is a plain English fallback for those
// that don't, such as logs and scripts.
package alerts

import (
	"cmp"
	"fmt"
	"slices"
	"strconv"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
)

// Severities, most serious first.
const (
	Error   = "error"   // broken: something isn't working
	Warning = "warning" // likely wrong: fix before a match
	Caution = "caution" // worth knowing, often fine
)

// Codes. A camera warning from config's CameraStatus keeps its own code.
const (
	CodeRenderFailed      = "render_failed"
	CodeCameraIDConflict  = "camera_id_conflict"
	CodeRegionUncovered   = "region_uncovered"
	CodeCameraUnplaced    = "camera_unplaced"
	CodeFileChanged       = "file_changed"
	CodeSocketProblem     = "socket_problem"
	CodeSocketSilent      = "socket_silent"
	CodeHeightsUnreadable = "heights_unreadable"
	CodeTeamDefaultHeight = "team_default_height"
	CodeLoopbackMulticast = "loopback_multicast_off"
)

// Alert is one problem.
type Alert struct {
	// ID is stable while the problem lasts, for keyed lists.
	ID       string `json:"id"`
	Code     string `json:"code"`
	Severity string `json:"severity" enum:"error,warning,caution"`
	CameraID *int   `json:"cameraId,omitempty"`
	// Params are the facts the alert is about, named per code.
	Params  map[string]any `json:"params,omitempty"`
	Message string         `json:"message"`
}

// cameraWarnings are config.CameraStatus warning codes by severity. Codes
// not listed are cautions.
var cameraWarnings = map[string]string{
	config.WarningFieldChanged:              Warning,
	config.WarningCalibrationAspectMismatch: Warning,
	config.WarningSeedStale:                 Warning,
	config.WarningSeedAspectMismatch:        Warning,
	config.WarningUnknownSetting:            Warning,
	config.WarningSeedRescalable:            Caution,
	config.WarningSeedResolutionUnknown:     Caution,
}

// Codes is every alert code Collect can raise, sorted: its own and the
// camera warning codes it passes through.
func Codes() []string {
	codes := append([]string{
		CodeRenderFailed, CodeCameraIDConflict, CodeRegionUncovered, CodeCameraUnplaced, CodeFileChanged,
		CodeSocketProblem, CodeSocketSilent, CodeHeightsUnreadable, CodeTeamDefaultHeight, CodeLoopbackMulticast,
	}, config.WarningCodes...)

	slices.Sort(codes)

	return codes
}

// Network is what the host's sockets report.
type Network struct {
	Vision            multicast.Status
	GC                multicast.Status
	Sources           []detections.Source
	Referee           referee.State
	Loopback          string
	LoopbackMulticast bool
}

// Collect lists every alert for the working document doc, the store's state,
// and the network, most serious first.
func Collect(doc config.Document, state config.State, net Network) []Alert {
	out := []Alert{}

	add := func(a Alert) { out = append(out, a) }

	for _, id := range sortedKeys(state.RenderErrors) {
		add(Alert{
			ID: "render-" + strconv.Itoa(id), Code: CodeRenderFailed, Severity: Error, CameraID: &id,
			Params:  map[string]any{"error": state.RenderErrors[id]},
			Message: fmt.Sprintf("Camera %d's config.yml couldn't be written: %s", id, state.RenderErrors[id]),
		})
	}

	for id := range doc.CameraCount() {
		receiving := receivingAddresses(net.Sources, id)

		if len(receiving) > 1 {
			add(Alert{
				ID: "conflict-" + strconv.Itoa(id), Code: CodeCameraIDConflict, Severity: Error, CameraID: ptr(id),
				Params:  map[string]any{"addresses": receiving},
				Message: fmt.Sprintf("Camera %d is being sent from %d places at once: %v", id, len(receiving), receiving),
			})
		}

		if !hasCamera(doc, id) {
			add(Alert{
				ID: "uncovered-" + strconv.Itoa(id), Code: CodeRegionUncovered, Severity: Warning,
				Params:  map[string]any{"slot": id},
				Message: fmt.Sprintf("No camera covers region %d of %d.", id, doc.CameraCount()),
			})
		}
	}

	for _, s := range net.Sources {
		if hasCamera(doc, int(s.CameraID)) {
			continue
		}

		add(Alert{
			ID: fmt.Sprintf("unplaced-%d-%s", s.CameraID, s.Address), Code: CodeCameraUnplaced, Severity: Warning,
			Params:  map[string]any{"slot": s.CameraID, "address": s.Address},
			Message: fmt.Sprintf("Camera %d is sending from %s but isn't in the layout.", s.CameraID, s.Address),
		})
	}

	for _, status := range state.Cameras {
		for _, w := range status.Warnings {
			add(Alert{
				ID: fmt.Sprintf("camera-%d-%s", status.CameraID, w.Code), Code: w.Code,
				Severity: cmp.Or(cameraWarnings[w.Code], Caution), CameraID: ptr(status.CameraID),
				Message: w.Message,
			})
		}
	}

	if ext := state.External; ext != nil {
		add(Alert{
			ID: "file-changed", Code: CodeFileChanged, Severity: Warning,
			Params:  map[string]any{"path": state.Path, "error": ext.Error, "changes": len(ext.Changes)},
			Message: fmt.Sprintf("%s was changed outside the GUI.", state.Path),
		})
	}

	for _, socket := range []struct {
		kind   string
		status multicast.Status
	}{{"vision", net.Vision}, {"gc", net.GC}} {
		params := map[string]any{"socket": socket.kind, "address": socket.status.Address}

		switch {
		case socket.status.Problem != "":
			params["problem"] = socket.status.Problem
			add(Alert{
				ID: "socket-" + socket.kind, Code: CodeSocketProblem, Severity: Error, Params: params,
				Message: fmt.Sprintf("The %s socket on %s isn't open: %s", socket.kind, socket.status.Address, socket.status.Problem),
			})
		case !socket.status.Receiving:
			// Off a competition field a game controller is optional: lab use
			// often runs without one. On one, a match can't run without it.
			if socket.kind == "gc" && !doc.CompetitionField {
				continue
			}

			severity := Warning
			if socket.kind == "gc" {
				severity = Error
			}

			params["everHeard"] = socket.status.LastHeard != nil
			add(Alert{
				ID: "silent-" + socket.kind, Code: CodeSocketSilent, Severity: severity, Params: params,
				Message: fmt.Sprintf("Nothing is arriving on the %s socket (%s).", socket.kind, socket.status.Address),
			})
		}
	}

	if ref := net.Referee; ref.HeightsError != "" {
		add(Alert{
			ID: "heights-unreadable", Code: CodeHeightsUnreadable, Severity: Warning,
			Params:  map[string]any{"file": ref.HeightsFile, "error": ref.HeightsError},
			Message: fmt.Sprintf("The robot height table %s can't be read: %s", ref.HeightsFile, ref.HeightsError),
		})
	} else {
		for _, color := range []string{"yellow", "blue"} {
			issue := HeightIssue(doc, state, net, color)
			if issue == "" {
				continue
			}

			severity := Warning
			if doc.CompetitionField {
				severity = Error
			}

			team := teamName(net, color)
			add(Alert{
				ID: "team-height-" + color, Code: CodeTeamDefaultHeight, Severity: severity,
				Params:  map[string]any{"color": color, "team": team, "issue": issue},
				Message: fmt.Sprintf("The %s team's robots get vision_processor's default height: %s.", color, issueText[issue]),
			})
		}
	}

	if !net.LoopbackMulticast {
		add(Alert{
			ID: "loopback-multicast", Code: CodeLoopbackMulticast, Severity: Caution,
			Params:  map[string]any{"interface": net.Loopback},
			Message: fmt.Sprintf("Multicast is off on %s, so vision_processor on this machine can't be heard here.", net.Loopback),
		})
	}

	rank := map[string]int{Error: 0, Warning: 1, Caution: 2}
	slices.SortStableFunc(out, func(a, b Alert) int { return rank[a.Severity] - rank[b.Severity] })

	return out
}

// Height issues: why a color's robots would get vision_processor's default
// height.
const (
	IssueNotInTable    = "notInTable"    // the game controller's team isn't in the table, no choice
	IssueNoHeight      = "noHeight"      // no game controller team and no choice for the color
	IssueCustomMissing = "customMissing" // a custom height chosen but not entered yet
	IssueUnsaved       = "unsaved"       // a choice not written to disk yet
)

var issueText = map[string]string{
	IssueNotInTable:    "the team isn't in the height table",
	IssueNoHeight:      "no team is named and no height is set",
	IssueCustomMissing: "a custom height is chosen but not entered",
	IssueUnsaved:       "the height choice isn't saved yet",
}

// HeightIssue is why color's robots would get vision_processor's default
// height, or "" if they have a known one.
func HeightIssue(doc config.Document, state config.State, net Network, color string) string {
	team := teamName(net, color)

	// On a competition field choices are kept by team, and nothing is playing
	// until the game controller names one; the game controller alert covers
	// that. Off one they're kept by color.
	var key string

	var own *config.TeamOverride

	switch {
	case doc.CompetitionField && team == "":
		return ""
	case doc.CompetitionField:
		key = "teams.overrides." + team

		if doc.Teams != nil {
			if o, ok := doc.Teams.Overrides[team]; ok {
				own = &o
			}
		}
	default:
		key = "teams.by_color." + color

		if doc.Teams != nil && doc.Teams.ByColor != nil {
			own = map[string]*config.TeamOverride{"yellow": doc.Teams.ByColor.Yellow, "blue": doc.Teams.ByColor.Blue}[color]
		}
	}

	switch {
	case own == nil && team == "":
		return IssueNoHeight
	case own == nil:
		if _, ok := net.Referee.Heights[team]; !ok {
			return IssueNotInTable
		}

		return ""
	case own.Name == "" && own.Height == nil:
		return IssueCustomMissing
	case unsaved(state, key):
		return IssueUnsaved
	}

	return ""
}

// unsaved is whether the saved file lacks the choice at key as it is now.
// Paths are compared whole, since team names can contain dots ("N.R.G").
func unsaved(state config.State, key string) bool {
	paths := []string{"teams", "teams.overrides", "teams.by_color", key, key + ".name", key + ".height"}

	for _, c := range state.Changes {
		if slices.Contains(paths, c.Path) {
			return true
		}
	}

	return false
}

// teamName is the team the game controller puts in color, or "" if none, or
// if the game controller has gone quiet (its last names may be a past
// match's).
func teamName(net Network, color string) string {
	if !net.GC.Receiving || net.Referee.Heard == nil {
		return ""
	}

	if color == "yellow" {
		return net.Referee.Yellow.Name
	}

	return net.Referee.Blue.Name
}

func receivingAddresses(sources []detections.Source, id int) []string {
	out := []string{}

	for _, s := range sources {
		if int(s.CameraID) == id && s.Receiving {
			out = append(out, s.Address)
		}
	}

	return out
}

func hasCamera(doc config.Document, id int) bool {
	return slices.ContainsFunc(doc.Cameras, func(c config.Camera) bool { return c.CameraID == id })
}

func sortedKeys(m map[int]string) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}

	slices.Sort(keys)

	return keys
}

func ptr(v int) *int { return &v }

package alerts

import (
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
)

// healthy is a two-camera lab setup with nothing wrong.
func healthy() (config.Document, config.State, Network) {
	now := time.Now()
	doc := config.Document{Cameras: []config.Camera{{CameraID: 0}, {CameraID: 1}}}

	return doc, config.State{Path: "vision.yml"}, Network{
		Vision:            multicast.Status{Address: "224.5.23.2:10006", Receiving: true},
		GC:                multicast.Status{Address: "224.5.23.1:10003", Receiving: true},
		Sources:           []detections.Source{{CameraID: 0, Address: "10.0.0.1", Receiving: true}, {CameraID: 1, Address: "10.0.0.2", Receiving: true}},
		Referee:           referee.State{Heard: &now, Yellow: referee.Team{Name: "A"}, Blue: referee.Team{Name: "B"}, Heights: map[string]float64{"A": 150, "B": 140}},
		LoopbackMulticast: true,
	}
}

func codes(alerts []Alert) map[string]string {
	out := map[string]string{}
	for _, a := range alerts {
		out[a.ID] = a.Code + "/" + a.Severity
	}

	return out
}

func TestCollectHealthy(t *testing.T) {
	if got := Collect(healthy()); len(got) != 0 {
		t.Errorf("Collect = %+v, want nothing", got)
	}
}

func TestCollect(t *testing.T) {
	for name, tc := range map[string]struct {
		mutate func(*config.Document, *config.State, *Network)
		id     string
		want   string
	}{
		"conflict": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Sources = append(n.Sources, detections.Source{CameraID: 1, Address: "10.0.0.9", Receiving: true})
		}, "conflict-1", "camera_id_conflict/error"},
		"uncovered": {func(d *config.Document, _ *config.State, _ *Network) {
			d.Cameras = d.Cameras[:1]
			d.Layout = &config.Layout{CameraCount: 2}
		}, "uncovered-1", "region_uncovered/warning"},
		"unplaced": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Sources = append(n.Sources, detections.Source{CameraID: 3, Address: "10.0.0.3", Receiving: true})
		}, "unplaced-3-10.0.0.3", "camera_unplaced/warning"},
		"camera warning": {func(_ *config.Document, s *config.State, _ *Network) {
			s.Cameras = []config.CameraStatus{{CameraID: 0, Warnings: []config.Warning{{Code: "seed_stale"}}}}
		}, "camera-0-seed_stale", "seed_stale/warning"},
		"unlisted camera warning": {func(_ *config.Document, s *config.State, _ *Network) {
			s.Cameras = []config.CameraStatus{{CameraID: 0, Warnings: []config.Warning{{Code: "something_new"}}}}
		}, "camera-0-something_new", "something_new/caution"},
		"render failed": {func(_ *config.Document, s *config.State, _ *Network) {
			s.RenderErrors = map[int]string{1: "permission denied"}
		}, "render-1", "render_failed/error"},
		"file changed": {func(_ *config.Document, s *config.State, _ *Network) {
			s.External = &config.ExternalState{}
		}, "file-changed", "file_changed/warning"},
		"socket problem": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Vision.Problem = "no usable network interface"
		}, "socket-vision", "socket_problem/error"},
		"vision silent": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Vision.Receiving = false
		}, "silent-vision", "socket_silent/warning"},
		"gc silent on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.GC.Receiving = false
		}, "silent-gc", "socket_silent/error"},
		"team not in table": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.Yellow.Name = "C"
		}, "team-height-yellow", "team_default_height/warning"},
		"heights unreadable": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.HeightsError = "no such file"
		}, "heights-unreadable", "heights_unreadable/warning"},
		"loopback": {func(_ *config.Document, _ *config.State, n *Network) {
			n.LoopbackMulticast = false
			n.Loopback = "lo"
		}, "loopback-multicast", "loopback_multicast_off/caution"},
	} {
		t.Run(name, func(t *testing.T) {
			doc, state, net := healthy()
			tc.mutate(&doc, &state, &net)

			if got := codes(Collect(doc, state, net)); got[tc.id] != tc.want {
				t.Errorf("alerts = %v, want %s = %s", got, tc.id, tc.want)
			}
		})
	}
}

func TestGCSilenceOffACompetitionFieldIsFine(t *testing.T) {
	doc, state, net := healthy()
	net.GC.Receiving = false

	// No game controller also means no team names: each color has no height.
	got := codes(Collect(doc, state, net))
	if _, ok := got["silent-gc"]; ok {
		t.Errorf("alerts = %v, want no silent-gc off a competition field", got)
	}
}

func TestHeightIssue(t *testing.T) {
	custom := 120.0

	for name, tc := range map[string]struct {
		mutate func(*config.Document, *config.State, *Network)
		want   string
	}{
		"in table": {func(*config.Document, *config.State, *Network) {}, ""},
		"no gc":    {func(_ *config.Document, _ *config.State, n *Network) { n.GC.Receiving = false }, IssueNoHeight},
		"by color, saved": {func(d *config.Document, _ *config.State, _ *Network) {
			d.Teams = &config.Teams{ByColor: &config.ColorOverrides{Yellow: &config.TeamOverride{Height: &custom}}}
		}, ""},
		"by color, unsaved": {func(d *config.Document, s *config.State, _ *Network) {
			d.Teams = &config.Teams{ByColor: &config.ColorOverrides{Yellow: &config.TeamOverride{Height: &custom}}}
			s.Changes = []config.Change{{Path: "teams.by_color.yellow.height"}}
		}, IssueUnsaved},
		"custom missing": {func(d *config.Document, _ *config.State, _ *Network) {
			d.Teams = &config.Teams{ByColor: &config.ColorOverrides{Yellow: &config.TeamOverride{}}}
		}, IssueCustomMissing},
		"competition, by team": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.Referee.Yellow.Name = "N.R.G"
			d.Teams = &config.Teams{Overrides: map[string]config.TeamOverride{"N.R.G": {Name: "A"}}}
		}, ""},
		// Only N.R.G's override is unsaved; "N" mustn't be tied to it.
		"competition, unsaved matched by whole path": {func(d *config.Document, s *config.State, n *Network) {
			d.CompetitionField = true
			n.Referee.Yellow.Name = "N"
			d.Teams = &config.Teams{Overrides: map[string]config.TeamOverride{"N": {Height: &custom}, "N.R.G": {Height: &custom}}}
			s.Changes = []config.Change{{Path: "teams.overrides.N.R.G.height"}}
		}, ""},
		"competition, no team named": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.Referee.Heard = nil
		}, ""},
	} {
		t.Run(name, func(t *testing.T) {
			doc, state, net := healthy()
			tc.mutate(&doc, &state, &net)

			if got := HeightIssue(doc, state, net, "yellow"); got != tc.want {
				t.Errorf("HeightIssue = %q, want %q", got, tc.want)
			}
		})
	}
}

package alerts

import (
	"fmt"
	"testing"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/peers"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/supervisor"
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
		Networked:         true,
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
		"vision never heard": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Vision.Receiving = false
		}, "silent-vision", "socket_silent/caution"},
		"vision went silent": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Vision.Receiving = false
			n.Vision.LastHeard = n.Referee.Heard
		}, "silent-vision", "socket_silent/warning"},
		"vision never heard on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.Vision.Receiving = false
		}, "silent-vision", "socket_silent/warning"},
		"vision never heard though one runs here": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Vision.Receiving = false
			n.Processes = []supervisor.Status{{State: supervisor.StateRunning}}
		}, "silent-vision", "socket_silent/warning"},
		"no height, no game controller heard": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.Heard = nil
			n.Referee.Yellow.Name = ""
		}, "team-height-yellow", "team_default_height/caution"},
		"no height, game controller heard": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.Yellow.Name = ""
		}, "team-height-yellow", "team_default_height/warning"},
		"half-entered height, no game controller heard": {func(d *config.Document, _ *config.State, n *Network) {
			n.Referee.Heard = nil
			n.Referee.Yellow.Name = ""
			d.Teams = &config.Teams{ByColor: &config.ColorOverrides{Yellow: &config.TeamOverride{}}}
		}, "team-height-yellow", "team_default_height/warning"},
		"gc silent on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.GC.Receiving = false
		}, "silent-gc", "socket_silent/error"},
		"team not in table": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.Yellow.Name = "C"
		}, "team-height-yellow", "team_default_height/warning"},
		"first run": {func(d *config.Document, _ *config.State, _ *Network) {
			d.FirstRun = true
		}, "first-run", "first_run_not_publishing/warning"},
		"device conflict": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Processes = []supervisor.Status{
				{CameraID: 0, State: supervisor.StateRunning},
				{CameraID: 1, State: supervisor.StateConflict, Conflict: "camera 0 already uses OPENCV /dev/video0"},
			}
		}, "device-1", "device_conflict/error"},
		"no camera chosen": {func(d *config.Document, _ *config.State, _ *Network) {
			d.Cameras[1].ConfigPath = "cam1.yml"
			d.Cameras[1].Config = noCamera
		}, "no-camera-1", "camera_not_selected/warning"},
		"no camera chosen on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			d.Cameras[1].Config = noCamera
			n.Sources = n.Sources[:1]
		}, "no-camera-1", "camera_not_selected/error"},
		"crash loop": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Processes = []supervisor.Status{{
				CameraID: 0, State: supervisor.StateWaiting, Crashes: 2, LastExit: "exit status 1",
				Output: []string{"opening /dev/video0", "VIDIOC_REQBUFS: Device or resource busy", ""},
			}}
		}, "crashing-0", "process_crashing/error"},
		"heights unreadable": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Referee.HeightsError = "no such file"
		}, "heights-unreadable", "heights_unreadable/warning"},
		"calibration from two senders": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Conflicts = []peers.Conflict{{Kind: peers.KindCalibration, Subject: "camera 0 calibration", Senders: []peers.Sender{
				{Address: "10.0.0.1:10006"}, {Address: "10.0.0.1:41234", Local: true},
			}}}
		}, "sender-calibration-camera 0 calibration", "sender_conflict/error"},
		"two game controllers": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Conflicts = []peers.Conflict{{Kind: peers.KindReferee, Subject: "game controller", Senders: []peers.Sender{
				{Address: "10.0.0.4:10003"}, {Address: "10.0.0.5:10003"},
			}}}
		}, "sender-referee-game controller", "sender_conflict/warning"},
		"two game controllers on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.Conflicts = []peers.Conflict{{Kind: peers.KindReferee, Subject: "game controller", Senders: []peers.Sender{
				{Address: "10.0.0.4:10003"}, {Address: "10.0.0.5:10003"},
			}}}
		}, "sender-referee-game controller", "sender_conflict/error"},
		"two video senders": {func(_ *config.Document, _ *config.State, n *Network) {
			n.Conflicts = []peers.Conflict{{Kind: peers.KindVideo, Subject: "camera 1 video", Senders: []peers.Sender{
				{Address: "10.0.0.1:50000"}, {Address: "10.0.0.2:50000"},
			}}}
		}, "sender-video-camera 1 video", "sender_conflict/warning"},
		"two video senders on a competition field": {func(d *config.Document, _ *config.State, n *Network) {
			d.CompetitionField = true
			n.Conflicts = []peers.Conflict{{Kind: peers.KindVideo, Subject: "camera 1 video", Senders: []peers.Sender{
				{Address: "10.0.0.1:50000"}, {Address: "10.0.0.2:50000"},
			}}}
		}, "sender-video-camera 1 video", "sender_conflict/error"},
		"packets on two interfaces": {func(_ *config.Document, _ *config.State, n *Network) {
			both := []string{"eth0", "eth1"}
			n.Vision.Duplicates = []multicast.Duplicate{{Address: "10.0.0.1:10006", Interfaces: both}, {Address: "10.0.0.2:10006", Interfaces: both}}
			n.VideoDuplicates = map[int][]multicast.Duplicate{0: {{Address: "10.0.0.1:50000", Interfaces: both}}}
		}, "duplicates-eth0-eth1", "duplicate_packets/warning"},
		"loopback": {func(_ *config.Document, _ *config.State, n *Network) {
			n.LoopbackMulticast = false
			n.Networked = false
			n.Loopback = "lo"
		}, "loopback-multicast", "loopback_multicast_off/warning"},
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

// Every kind of sender conflict, in a lab and on a competition field: the
// game controller and video are warnings in a lab and errors at a match;
// calibration and field geometry are always errors.
func TestSenderConflictSeverity(t *testing.T) {
	want := map[string][2]string{ // kind -> {lab, competition}
		peers.KindCalibration: {Error, Error},
		peers.KindGeometry:    {Error, Error},
		peers.KindInstance:    {Error, Error},
		peers.KindReferee:     {Warning, Error},
		peers.KindVideo:       {Warning, Error},
	}

	for kind, severities := range want {
		for i, competition := range []bool{false, true} {
			t.Run(fmt.Sprintf("%s/competition=%v", kind, competition), func(t *testing.T) {
				doc, state, net := healthy()
				doc.CompetitionField = competition
				net.Conflicts = []peers.Conflict{{Kind: kind, Subject: "subject", Senders: []peers.Sender{
					{Address: "10.0.0.1:10006"}, {Address: "10.0.0.2:41234", Host: "other", Local: false},
				}}}

				var found *Alert

				for _, a := range Collect(doc, state, net) {
					if a.Code == CodeSenderConflict {
						found = &a
					}
				}

				if found == nil {
					t.Fatal("no sender_conflict alert")
				}

				if found.Severity != severities[i] {
					t.Errorf("severity = %s, want %s", found.Severity, severities[i])
				}

				senders, _ := found.Params["senders"].([]string)
				if found.Params["kind"] != kind || len(senders) != 2 || senders[1] != "10.0.0.2:41234 (other)" {
					t.Errorf("params = %v", found.Params)
				}
			})
		}
	}
}

// One failed start can be a slow device; two in a row is a loop worth an
// alert, and its last output line usually says why.
func TestCrashingProcessNeedsTwoFailuresAndQuotesTheLastLine(t *testing.T) {
	doc, state, net := healthy()

	net.Processes = []supervisor.Status{{CameraID: 0, State: supervisor.StateWaiting, Crashes: 1, Output: []string{"bad"}}}
	for _, a := range Collect(doc, state, net) {
		if a.Code == CodeProcessCrashing {
			t.Fatalf("one crash raised %v", a)
		}
	}

	net.Processes[0].Crashes = 3
	net.Processes[0].Output = []string{"opening", "  no such device  ", " "}

	for _, a := range Collect(doc, state, net) {
		if a.Code == CodeProcessCrashing {
			if a.Params["lastLine"] != "no such device" || a.Params["crashes"] != 3 {
				t.Errorf("params = %v", a.Params)
			}

			return
		}
	}

	t.Fatal("three crashes in a row raised no process_crashing alert")
}

var noCamera = config.Settings{"camera": map[string]any{"driver": config.DriverNone}}

// A camera whose config the host doesn't write has its own; sending, it has a
// driver, whatever this document says.
func TestNoCameraChosenIsFineForAnExternalCameraThatSends(t *testing.T) {
	doc, state, net := healthy()
	doc.Cameras[1].Config = noCamera

	if got := Collect(doc, state, net); len(got) != 0 {
		t.Errorf("Collect = %+v, want nothing", got)
	}
}

// A camera held for having no driver isn't also crash-looping.
func TestNoCameraChosenIsNotCrashing(t *testing.T) {
	doc, state, net := healthy()
	net.Processes = []supervisor.Status{{CameraID: 0, State: supervisor.StateNoCamera, Crashes: 5}}

	if got := Collect(doc, state, net); len(got) != 0 {
		t.Errorf("Collect = %+v, want nothing", got)
	}
}

func TestLoopbackMulticastOffIsFineWithANetwork(t *testing.T) {
	doc, state, net := healthy()
	net.LoopbackMulticast = false

	if got := Collect(doc, state, net); len(got) != 0 {
		t.Errorf("Collect = %+v, want nothing", got)
	}
}

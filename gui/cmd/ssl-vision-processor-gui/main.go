package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/defaults"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/discovery"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/livefield"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/origins"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/supervisor"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
	"google.golang.org/protobuf/encoding/protojson"
)

var address = flag.String("address", ":8085", "The address on which the vision processor GUI and API are served, default: :8085")

// The vision and game controller addresses (vision.yml's defaults.network)
// and the interfaces the host uses (host.interfaces) can change at runtime, so
// there are no flags for them.

// vision.yml holds everything the host owns: the field template, shared and
// per-camera vision_processor settings, and locked calibrations. It lives in
// one flat folder; see config.ValidName.
var configDir = flag.String("configDir", "", "Folder holding every config file, created if missing. Default: $XDG_CONFIG_HOME or ~/.config, then rcssl/vision-processor-gui")
var configFile = flag.String("configFile", "vision.yml", "Config file (field, cameras, calibrations): a file name in -configDir, created if missing. Default: vision.yml")

var allowOrigin = flag.String("allowOrigin", "", "Comma-separated origins (scheme://host:port) whose browser pages may use the API on a competition field, besides the host's own; * for any. In a lab, pages from this network are allowed anyway. Default: none")
var allowHost = flag.String("allowHost", "", "Comma-separated extra names this host is reached by, e.g. a DNS alias; * to turn the check off. Its IPs, hostname, .local name and reverse-DNS names are always accepted. Default: none")
var openapiOut = flag.String("openapi", "", "Write the API's OpenAPI spec to this path and exit (make builds openapi.json with it)")
var imgDir = flag.String("imgDir", "", "Directory the vision processor writes debug snapshot images to. Default: $XDG_STATE_HOME or ~/.local/state, then rcssl/vision-processor-gui/img; with -vp, img in -configDir")
var logLevelFlag = flag.String("logLevel", "Info", "Log Level: Debug, Info, Warn, Error. Default: Info")

// Until vision_processor takes config over the network: run it for each camera
// with a config_path, restarting it when a setting it reads only at startup
// changes.
var vpCommand = flag.String("vp", "", "Run this vision_processor (binary, then any arguments; the config path is appended) for each camera with a config_path, restarting it when a startup-only setting changes. Default: none, run them yourself")

var logFile = flag.String("logFile", "", "Rotating log file to write alongside stderr, empty to disable. Default: $XDG_STATE_HOME or ~/.local/state, then rcssl/vision-processor-gui/logs/vision-processor-gui.log")
var showVersion = flag.Bool("version", false, "Print the version and exit")
var showReadme = flag.Bool("readme", false, "Print the user guide (setup, file locations, network, troubleshooting) and exit")

// version is set at build time (make release); see the Makefile's LDFLAGS.
var version = "dev"

func main() {
	// Deferred cleanup (closeLog) must run even on a fatal startup error, which
	// os.Exit would skip -- so main just reports run's exit code instead of
	// exiting directly.
	os.Exit(run())
}

func run() int {
	flag.Parse()

	if *showVersion {
		fmt.Println("ssl-vision-processor-gui", version)

		return 0
	}

	if *showReadme {
		return printReadme()
	}

	if *openapiOut != "" {
		return writeOpenAPI(*openapiOut)
	}

	// Resolved before Setup, which needs the log file.
	p, err := resolvePaths(givenPaths())
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	// Parsed before Setup so that the level is known when the handler is built,
	// but reported after it, so the warning goes through the same handler as
	// everything else.
	level, levelErr := logging.ParseLevel(*logLevelFlag)

	closeLog, logErr := logging.Setup(logging.Config{
		Level: level,
		File:  p.logFile,
	})
	defer closeLog()

	if levelErr != nil {
		slog.Warn("falling back to info level", "err", levelErr)
	}

	if logErr != nil {
		slog.Warn("file logging disabled", "err", logErr)
	}

	slog.Info("Init VP GUI", "version", version)
	logPaths(p)

	if err := makeDirs(p); err != nil {
		slog.Error("creating folders", "err", err)

		return 1
	}

	if err := bootstrapConfig(p.configDir, p.configFile); err != nil {
		slog.Error("creating config file", "err", err)

		return 1
	}

	h, err := newHost(p.config(), hostOptions{
		trustedOrigins: splitList(*allowOrigin),
		allowedHosts:   splitList(*allowHost),
		imgDir:         p.imgDir,
		verbose:        level == slog.LevelDebug,
		vpCommand:      strings.Fields(*vpCommand),
	})
	if err != nil {
		slog.Error("starting", "config", p.config(), "err", err)

		return 1
	}

	slog.Info("loaded config", "path", p.config())

	// register Ctrl+C handler
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	warnLoopbackMulticast()

	// Reverse lookups can be slow; until they finish, the host is still
	// reachable by IP, hostname and .local name.
	go func() { h.policy.AddOwnHosts(ownHostNames(ctx)...) }()

	geom, store, sockets, match, wsHub, status := h.geom, h.store, h.sockets, h.match, h.hub, h.status

	var wg sync.WaitGroup

	runBackground(&wg, "vision multicast", func() error { return sockets.vision.Run(ctx) })
	runBackground(&wg, "game controller multicast", func() error { return sockets.gc.Run(ctx) })
	runBackground(&wg, "tracker multicast", func() error { return sockets.tracker.Run(ctx) })
	suspend := multicast.NewSuspendDetector()

	runBackground(&wg, "geometry publish loop", func() error {
		return geom.Run(ctx, func(encoded []byte) {
			sendGeometry(sockets, store, encoded)
			publishGeometryToHub(wsHub, geom)
			// Live calibration and socket state come from the network, not the
			// store, so they're refreshed on this tick rather than only on
			// store changes.
			publishConfigState(wsHub, store)
			// After a suspend every socket is reopened: the network may have
			// gone away and come back while nothing ran to notice.
			if slept := suspend.Check(); slept > 0 {
				slog.Info("resumed after suspend", "duration", slept.Round(time.Second))
				sockets.reopen()
			}
			// Re-evaluated every tick so automatic interface selection
			// follows cables, Wi-Fi, and containers coming and going.
			sockets.apply(store)
			match.SetHeightsFile(botHeightsFile(store))
			publishStatus(wsHub, status)
			publishJSON(wsHub, processesStateTopic, h.procs.Statuses())
		})
	})
	runBackground(&wg, "config file watcher", func() error { return store.Watch(ctx, configWatchInterval) })
	runBackground(&wg, "live field publish loop", func() error { return publishLiveField(ctx, wsHub, status.live) })

	if h.procs != nil {
		runBackground(&wg, "vision_processor supervisor", func() error { return h.procs.Run(ctx) })
	}

	srv := &http.Server{
		Addr:              *address,
		Handler:           h.handler,
		ReadHeaderTimeout: 10 * time.Second,
	}

	serveFailed := make(chan struct{})

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server init failed", "err", err)

			// ListenAndServer always returns, but ErrServerClosed is the graceful shutdown condition
			// if we got anything else, the system didn't start and the process should die
			// call stopSignals() now to cancel the context otherwise we continue to block on Ctrl+C
			close(serveFailed)
			stopSignals()
		}
	}()

	slog.Info("UI is available", "url", formattedAddress(*address))

	<-ctx.Done()

	// Restore default signal handling: while shutdown is in progress a second
	// Ctrl-C should kill the process outright rather than being swallowed.
	stopSignals()

	slog.Info("initiating graceful shutdown")

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		slog.Error("http server failed to close gracefully", "err", err)
	}

	wg.Wait()

	slog.Info("shutdown complete")

	// A port already in use has to read as a failure, or a service manager's
	// restart-on-failure never fires.
	select {
	case <-serveFailed:
		return 1
	default:
		return 0
	}
}

// host is everything the process runs, wired together but not yet running:
// nothing opens a socket or starts a loop until run() starts its background
// tasks, which is what lets tests build one.
type host struct {
	geom    *geometry.Geometry
	store   *config.Store
	sockets *networkSockets
	match   *referee.Match
	hub     *hub.Hub
	status  *hostStatus
	policy  *origins.Policy
	// procs is nil unless the host runs the vision_processors itself (-vp).
	procs   *supervisor.Supervisor
	handler http.Handler
}

type hostOptions struct {
	trustedOrigins []string
	allowedHosts   []string
	imgDir         string
	verbose        bool
	// vpCommand runs a vision_processor per local camera; empty for none.
	vpCommand []string
}

// newHost opens the config at path and wires the store, geometry, sockets,
// hub and HTTP handler together. A store change re-points the sockets,
// switches the origin rules for a competition field, and publishes the
// config, network and alert topics.
func newHost(path string, opts hostOptions) (*host, error) {
	geom, store, err := openConfig(path)
	if err != nil {
		return nil, err
	}

	policy, err := origins.New(origins.Options{TrustedOrigins: opts.trustedOrigins, AllowedHosts: opts.allowedHosts})
	if err != nil {
		return nil, fmt.Errorf("-allowOrigin / -allowHost: %w", err)
	}

	doc, addrs, auto, ifaces := currentNetwork(store)
	mcOpts := multicast.Options{Verbose: opts.verbose}
	tracker := detections.NewTracker()
	instances := discovery.NewRegistry()
	match := &referee.Match{}
	watches := newPeerWatches()
	trackTeams := func(yellow, blue string, from *net.UDPAddr) {
		now := time.Now()
		match.Teams.Record(yellow, blue, now)
		watches.referee.Record("game controller", nil, from, now)
	}
	live := livefield.NewFeed()
	track := func(d *vision.SSL_DetectionFrame, from *net.UDPAddr) {
		now := time.Now()
		tracker.Record(d.GetCameraId(), from.IP, now)
		live.RecordDetection(d, from.IP, now)
		watches.detection(d.GetCameraId(), from, now)
	}
	videos := video.NewManager(opts.verbose)
	videos.OnPacket(watches.videoPacket)
	trackAutoref := func(p *vision.TrackerWrapperPacket, from net.IP) { live.RecordTracked(p, from, time.Now()) }
	sockets := &networkSockets{
		vision:     multicast.NewEndpoint("vision", addrs.VisionAddress(), ifaces, withSend(mcOpts), multicast.VisionConsumer(watches.absorber(geom), track, watches.announcer(instances))),
		detections: tracker,
		gc:         multicast.NewEndpoint("game controller", addrs.GCAddress(), ifaces, mcOpts, multicast.RefereeConsumer(trackTeams)),
		tracker:    multicast.NewEndpoint("tracker", doc.TrackerAddress(), ifaces, mcOpts, multicast.TrackerConsumer(trackAutoref)),
		video:      videos,
		peers:      watches,
		auto:       auto,
		ifaces:     ifaces,
	}
	sockets.apply(store)
	match.SetHeightsFile(botHeightsFile(store))

	wsHub := hub.New(wrapperPacketTopic, configStateTopic, networkStateTopic, alertsStateTopic, processesStateTopic, liveFieldTopic, instancesStateTopic)
	status := &hostStatus{store: store, sockets: sockets, match: match, live: live, instances: instances}
	policy.SetStrict(competitionField(store))

	var procs *supervisor.Supervisor

	if len(opts.vpCommand) > 0 {
		procs = supervisor.New(supervisor.Options{
			Command:  opts.vpCommand,
			Dir:      filepath.Dir(path),
			OnChange: func() { publishJSON(wsHub, processesStateTopic, procs.Statuses()) },
			// A calibration solved against the old field is wrong for the new
			// one, locked or not; withholding it makes the restart recalibrate.
			Recalibrate: func(cameraID int) {
				if _, err := store.UnlockCalibration(cameraID); err != nil {
					slog.Warn("clearing calibration to recalibrate", "cam", cameraID, "err", err)
				}
			},
		})

		doc, _ := store.Working()
		procs.Sync(doc)
	}

	store.OnChange(func() {
		// A changed address or interface selection takes effect at once: the
		// endpoints close their sockets and reopen.
		sockets.apply(store)
		match.SetHeightsFile(botHeightsFile(store))
		policy.SetStrict(competitionField(store))

		publishConfigState(wsHub, store)
		publishStatus(wsHub, status)

		if procs != nil {
			doc, _ := store.Working()
			procs.Sync(doc)
		}
	})

	return &host{
		geom:    geom,
		store:   store,
		sockets: sockets,
		match:   match,
		hub:     wsHub,
		status:  status,
		policy:  policy,
		procs:   procs,
		handler: NewVisionServer(geom, store, status, policy, wsHub, sockets.video, procs, opts.imgDir),
	}, nil
}

// writeOpenAPI writes the spec to path, for make.
func writeOpenAPI(path string) int {
	spec, err := openapiSpec()
	if err == nil {
		err = os.WriteFile(path, append(spec, '\n'), 0o644)
	}

	if err != nil {
		fmt.Fprintln(os.Stderr, "writing the OpenAPI spec:", err)

		return 1
	}

	return 0
}

// configWatchInterval is how often vision.yml is checked for edits made
// outside the GUI.
const configWatchInterval = time.Second

// bootstrapConfig creates the config file from the embedded defaults if it
// doesn't exist yet, and with it the robot height table vision_processor
// needs. Never touches an existing config.
func bootstrapConfig(dir, name string) error {
	path := filepath.Join(dir, name)

	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	doc, err := config.Default()
	if err != nil {
		return fmt.Errorf("default config: %w", err)
	}

	data, err := config.Marshal(doc)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	slog.Info("created config file", "path", path)

	return writeRobotHeights(filepath.Join(dir, defaults.RobotHeightsFile))
}

// writeRobotHeights writes the embedded table to path unless one is there.
func writeRobotHeights(path string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	}

	data, err := defaults.RobotHeights()
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	return nil
}

// printReadme writes the embedded user guide to stdout, for -readme. The binary
// is often all someone has, so the guide travels with it and always matches
// its version.
func printReadme() int {
	readme, err := defaults.Readme()
	if err != nil {
		fmt.Fprintln(os.Stderr, err)

		return 1
	}

	os.Stdout.Write(readme)

	return 0
}

// givenPaths collects the path flags. A flag not given is nil, so an empty
// -logFile can mean no log file.
func givenPaths() pathFlags {
	f := pathFlags{configDir: *configDir, configFile: *configFile, supervising: *vpCommand != ""}

	if flagSet("logFile") {
		f.logFile = logFile
	}

	if flagSet("imgDir") {
		f.imgDir = imgDir
	}

	return f
}

// makeDirs creates the config folder and image directory. The log directory
// is logging.Setup's, which only warns if it can't.
func makeDirs(p paths) error {
	for _, dir := range []string{p.configDir, p.imgDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return err
		}
	}

	return nil
}

// openConfig builds the live Geometry from path's field template and opens
// the store that keeps it in sync with the file.
func openConfig(path string) (*geometry.Geometry, *config.Store, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, nil, err
	}

	doc, err := config.Parse(data)
	if err != nil {
		return nil, nil, err
	}

	models, err := doc.ModelsProto()
	if err != nil {
		return nil, nil, err
	}

	geom, err := geometry.New(doc.Field, doc.OptionalFieldLines, models)
	if err != nil {
		return nil, nil, err
	}

	store, err := config.Open(path, geom)
	if err != nil {
		return nil, nil, err
	}

	return geom, store, nil
}

// flagSet reports whether the named flag was given on the command line.
func flagSet(name string) bool {
	set := false

	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			set = true
		}
	})

	return set
}

// configStateTopic carries config.Store's State to the frontend.
const configStateTopic = "config.state"

func publishConfigState(wsHub *hub.Hub, store *config.Store) {
	data, err := json.Marshal(store.State())
	if err != nil {
		slog.Error("marshalling config state", "err", err)

		return
	}

	wsHub.Publish(configStateTopic, data)
}

// networkSockets is the host's multicast groups.
type networkSockets struct {
	vision  *multicast.Endpoint
	gc      *multicast.Endpoint
	tracker *multicast.Endpoint
	// detections is which camera_ids the vision socket hears, from where.
	detections *detections.Tracker
	// video opens each camera's stream only while someone watches it.
	video *video.Manager
	// peers is who sends what on these groups.
	peers *peerWatches

	mu     sync.Mutex
	auto   bool
	ifaces []multicast.Interface
	// used is the previous apply's used interfaces, nil before the first,
	// so a network going away or coming back is logged once.
	used []string
}

// reopen forces every socket to reopen, after a suspend.
func (s *networkSockets) reopen() {
	s.vision.Reopen()
	s.gc.Reopen()
	s.tracker.Reopen()
	s.video.Reopen()
}

// apply moves every endpoint to the store's current addresses and interface
// selection. Each endpoint reopens only if something it uses changed.
func (s *networkSockets) apply(store *config.Store) {
	doc, addrs, auto, ifaces := currentNetwork(store)

	s.vision.SetAddress(addrs.VisionAddress())
	s.gc.SetAddress(addrs.GCAddress())
	s.tracker.SetAddress(doc.TrackerAddress())
	s.vision.SetInterfaces(ifaces)
	s.gc.SetInterfaces(ifaces)
	s.tracker.SetInterfaces(ifaces)
	s.video.Configure(cameraStreams(doc), ifaces)

	used := []string{}
	for _, i := range ifaces {
		if i.Used {
			used = append(used, i.Name)
		}
	}

	s.mu.Lock()
	previous := s.used
	s.auto, s.ifaces, s.used = auto, ifaces, used
	s.mu.Unlock()

	switch {
	case len(used) == 0 && (previous == nil || len(previous) > 0):
		slog.Warn("no usable network interface; waiting for network", "auto", auto)
	case len(used) > 0 && previous != nil && len(previous) == 0:
		slog.Info("network back", "interfaces", used)
	}
}

// currentNetwork is the store's working document, its network block, and its
// interface selection applied to this machine's interfaces.
func currentNetwork(store *config.Store) (config.Document, config.Network, bool, []multicast.Interface) {
	doc, _ := store.Working()

	auto, skip := doc.InterfaceSelection()
	ifaces, _ := multicast.Select(multicast.ListInterfaces(), auto, skip)

	return doc, hostNetwork(doc), auto, ifaces
}

// cameraStreams is every camera's live video stream settings.
func cameraStreams(doc config.Document) map[int]video.Stream {
	streams := make(map[int]video.Stream, len(doc.Cameras))

	for _, c := range doc.Cameras {
		s, ok, err := doc.Stream(c.CameraID)
		if err != nil || !ok {
			slog.Warn("reading camera stream settings", "camera", c.CameraID, "err", err)

			continue
		}

		streams[c.CameraID] = video.Stream{Active: s.Active, Address: s.Address}
	}

	return streams
}

// warnLoopbackMulticast logs once at startup if the loopback interface has
// multicast off, as Ubuntu ships it. The fix needs root, so it's only
// suggested.
func warnLoopbackMulticast() {
	if name, enabled, ok := multicast.LoopbackMulticast(); ok && !enabled {
		slog.Warn("multicast is off on the loopback interface", "interface", name, "enable with", "sudo ip link set "+name+" multicast on")
	}
}

// networkStateTopic carries NetworkState, and alertsStateTopic every
// current alert.
const (
	networkStateTopic = "network.state"
	alertsStateTopic  = "alerts.state"
	// processesStateTopic carries the supervised vision_processors (-vp).
	processesStateTopic = "processes.state"
	// instancesStateTopic carries every vision_processor heard announcing.
	instancesStateTopic = "instances.state"
)

// publishStatus publishes the network state and the alerts, which depend on
// it and on the store, and the vision_processors announcing.
func publishStatus(wsHub *hub.Hub, status StatusSource) {
	publishJSON(wsHub, networkStateTopic, status.Network())
	publishJSON(wsHub, alertsStateTopic, status.Alerts())
	publishJSON(wsHub, instancesStateTopic, status.Instances())
}

func publishJSON(wsHub *hub.Hub, topic string, v any) {
	data, err := json.Marshal(v)
	if err != nil {
		slog.Error("marshalling websocket topic", "topic", topic, "err", err)

		return
	}

	wsHub.Publish(topic, data)
}

// hostNetwork is doc's network block. The store only holds validated
// documents, so the fallback is for a bug, not a user error.
func hostNetwork(doc config.Document) config.Network {
	n, err := doc.Network()
	if err != nil {
		slog.Error("reading network config, keeping defaults", "err", err)

		n, _ = config.Document{}.Network()
	}

	return n
}

func withSend(opts multicast.Options) multicast.Options {
	opts.Send = true

	return opts
}

// wrapperPacketTopic is the hub topic name the frontend already subscribes to
// (see gui/frontend/src/App.svelte).
const wrapperPacketTopic = "wrapper_packet.out"

// sendGeometry publishes the geometry on the vision group, unless this is a
// first run: a host somebody is only trying out has no business telling a real
// network what the field looks like.
func sendGeometry(sockets *networkSockets, store *config.Store, encoded []byte) {
	if !store.FirstRun() {
		sockets.vision.Send(encoded)
	}
}

// publishGeometryToHub re-encodes the current geometry as protojson and
// publishes it to the browser-facing hub. Called once per publish tick
// alongside the raw-bytes multicast send, so both destinations stay in sync.
func publishGeometryToHub(wsHub *hub.Hub, geom *geometry.Geometry) {
	data, err := protojson.Marshal(geom.Snapshot())
	if err != nil {
		slog.Error("marshalling geometry for websocket", "err", err)

		return
	}

	wsHub.Publish(wrapperPacketTopic, data)
}

// runBackground runs task in its own goroutine tracked by wg, logging its
// error unless it is the expected result of ctx being cancelled.
func runBackground(wg *sync.WaitGroup, name string, task func() error) {
	wg.Add(1)

	go func() {
		defer wg.Done()

		if err := task(); err != nil && !errors.Is(err, context.Canceled) {
			slog.Error(name+" stopped", "err", err)
		}
	}()
}

// formattedAddress turns -address into a URL a person can actually open. A
// bind-all host (empty, "0.0.0.0", "::") isn't itself reachable -- printing
// the LAN-facing IP instead of "localhost" is what makes this useful to
// someone else on the venue network, not just the machine running it.
// localhost is only the fallback if that IP can't be determined.
func formattedAddress(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return "http://" + addr
	}

	if host != "" && host != "0.0.0.0" && host != "::" {
		return "http://" + net.JoinHostPort(host, port)
	}

	if ip, err := outboundIP(); err == nil {
		return "http://" + net.JoinHostPort(ip, port)
	}

	return "http://" + net.JoinHostPort("localhost", port)
}

// outboundIP reports this host's IP as seen by external routing. Dialing UDP
// doesn't send a packet, just consults the routing table for which local
// interface/address would be used -- so this works without real
// connectivity, only a route to the internet.
func outboundIP() (string, error) {
	conn, err := net.Dial("udp", "8.8.8.8:80")
	if err != nil {
		return "", err
	}
	defer conn.Close()

	addr, ok := conn.LocalAddr().(*net.UDPAddr)
	if !ok {
		return "", fmt.Errorf("unexpected local address type %T", conn.LocalAddr())
	}

	return addr.IP.String(), nil
}

// competitionField is whether the working document is a match setup, which
// limits browser pages to the -allowOrigin list.
func competitionField(store *config.Store) bool {
	doc, _ := store.Working()

	return doc.CompetitionField
}

// ownHostNames is this machine's hostname and the names its addresses
// reverse-resolve to, for the API's host check. Lookups that take too long
// are skipped.
func ownHostNames(ctx context.Context) []string {
	names := []string{}

	if name, err := os.Hostname(); err == nil {
		names = append(names, name, name+".local")
	}

	addrs, _ := net.InterfaceAddrs()

	ctx, cancel := context.WithTimeout(ctx, 3*time.Second)
	defer cancel()

	for _, a := range addrs {
		if ipnet, ok := a.(*net.IPNet); ok {
			found, _ := net.DefaultResolver.LookupAddr(ctx, ipnet.IP.String())
			names = append(names, found...)
		}
	}

	return names
}

// splitList splits a comma-separated flag, dropping empty entries.
func splitList(s string) []string {
	var out []string

	for _, item := range strings.Split(s, ",") {
		if item = strings.TrimSpace(item); item != "" {
			out = append(out, item)
		}
	}

	return out
}

// botHeightsFile is the robot height table the cameras read.
func botHeightsFile(store *config.Store) string {
	doc, _ := store.Working()

	return store.InDir(doc.BotHeightsFile())
}

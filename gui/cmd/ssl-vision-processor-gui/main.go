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
	"sync"
	"syscall"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/detections"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/referee"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/video"
	"google.golang.org/protobuf/encoding/protojson"
)

var address = flag.String("address", ":8085", "The address on which the vision processor GUI and API are served, default: :8085")

// The vision and game controller addresses (vision.yml's defaults.network)
// and the interfaces the host uses (host.interfaces) can change at runtime, so
// there are no flags for them.

// vision.yml holds everything the host owns: the field template, shared and
// per-camera vision_processor settings, and locked calibrations.
var configPath = flag.String("config", "vision.yml", "Host configuration file (field, cameras, calibrations), default: vision.yml")

// Used only when -config doesn't exist yet, to build it from a pre-vision.yml
// setup. Without -importGeometry the field comes from -geometryPreset.
var importGeometry = flag.String("importGeometry", "geometry.yml", "Legacy field geometry file to import into a new -config, default: geometry.yml")
var importConfig = flag.String("importConfig", "config.yml", "Legacy vision_processor config.yml to import into a new -config as its camera, default: config.yml")
var geometryPreset = flag.String("geometryPreset", "config/legacy/geometry-divB.yml", "Field preset for a new -config when there's no -importGeometry, default: config/legacy/geometry-divB.yml")
var imgDir = flag.String("imgDir", "img", "Directory the vision processor writes debug snapshot images to, default: img")
var logLevelFlag = flag.String("logLevel", "Info", "Log Level: Debug, Info, Warn, Error. Default: Info")
var logFile = flag.String("logFile", "logs/vision-processor-gui.log", "Rotating log file to write alongside stderr, empty to disable. Default: logs/vision-processor-gui.log")

func main() {
	// Deferred cleanup (closeLog) must run even on a fatal startup error, which
	// os.Exit would skip -- so main just reports run's exit code instead of
	// exiting directly.
	os.Exit(run())
}

func run() int {
	flag.Parse()

	// Parsed before Setup so that the level is known when the handler is built,
	// but reported after it, so the warning goes through the same handler as
	// everything else.
	level, levelErr := logging.ParseLevel(*logLevelFlag)

	closeLog, logErr := logging.Setup(logging.Config{
		Level: level,
		File:  *logFile,
	})
	defer closeLog()

	if levelErr != nil {
		slog.Warn("falling back to info level", "err", levelErr)
	}

	if logErr != nil {
		slog.Warn("file logging disabled", "err", logErr)
	}

	slog.Info("Init VP GUI")

	if err := bootstrapConfig(*configPath, *importGeometry, *importConfig, *geometryPreset); err != nil {
		slog.Error("creating config file", "err", err)

		return 1
	}

	geom, store, err := openConfig(*configPath)
	if err != nil {
		slog.Error("loading config", "path", *configPath, "err", err)

		return 1
	}

	slog.Info("loaded config", "path", *configPath)

	// register Ctrl+C handler
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	warnLoopbackMulticast()

	_, addrs, auto, ifaces := currentNetwork(store)
	opts := multicast.Options{Verbose: level == slog.LevelDebug}
	tracker := detections.NewTracker()
	teams := &referee.Teams{}
	trackTeams := func(yellow, blue string) { teams.Record(yellow, blue, time.Now()) }
	track := func(id uint32, from net.IP) { tracker.Record(id, from, time.Now()) }
	sockets := &networkSockets{
		vision:     multicast.NewEndpoint("vision", addrs.VisionAddress(), ifaces, withSend(opts), multicast.VisionConsumer(geom.Absorb, track)),
		detections: tracker,
		teams:      teams,
		heights:    &referee.Heights{},
		gc:         multicast.NewEndpoint("game controller", addrs.GCAddress(), ifaces, opts, multicast.RefereeConsumer(trackTeams)),
		video:      video.NewManager(level == slog.LevelDebug),
		auto:       auto,
		ifaces:     ifaces,
	}
	sockets.apply(store)
	wsHub := hub.New()

	store.OnChange(func() {
		// A changed address or interface selection takes effect at once: the
		// endpoints close their sockets and reopen.
		sockets.apply(store)

		publishConfigState(wsHub, store)
		publishNetworkState(wsHub, sockets)
	})

	var wg sync.WaitGroup

	runBackground(&wg, "vision multicast", func() error { return sockets.vision.Run(ctx) })
	runBackground(&wg, "game controller multicast", func() error { return sockets.gc.Run(ctx) })
	suspend := multicast.NewSuspendDetector()

	runBackground(&wg, "geometry publish loop", func() error {
		return geom.Run(ctx, func(encoded []byte) {
			sockets.vision.Send(encoded)
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
			publishNetworkState(wsHub, sockets)
		})
	})
	runBackground(&wg, "config file watcher", func() error { return store.Watch(ctx, configWatchInterval) })

	srv := &http.Server{
		Addr:              *address,
		Handler:           NewVisionServer(geom, store, wsHub, sockets.video, *imgDir),
		ReadHeaderTimeout: 10 * time.Second,
	}

	go func() {
		if err := srv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server init failed", "err", err)

			// ListenAndServer always returns, but ErrServerClosed is the graceful shutdown condition
			// if we got anything else, the system didn't start and the process should die
			// call stopSignals() now to cancel the context otherwise we continue to block on Ctrl+C
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

	return 0
}

// configWatchInterval is how often vision.yml is checked for edits made
// outside the GUI.
const configWatchInterval = time.Second

// bootstrapConfig creates path if it doesn't exist yet: imported from the
// legacy geometry/config files where present, otherwise from preset. Never
// touches an existing path.
func bootstrapConfig(path, legacyGeometry, legacyConfig, preset string) error {
	if _, err := os.Stat(path); err == nil {
		return nil
	} else if !os.IsNotExist(err) {
		return err
	}

	geometrySource := preset
	if fileExists(legacyGeometry) {
		geometrySource = legacyGeometry
	}

	configSource := ""
	if fileExists(legacyConfig) {
		configSource = legacyConfig
	}

	doc, err := config.Import(geometrySource, configSource)
	if err != nil {
		return fmt.Errorf("import %s / %q: %w", geometrySource, configSource, err)
	}

	data, err := config.Marshal(doc)
	if err != nil {
		return err
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	slog.Info("created config file", "path", path, "geometry", geometrySource, "config", configSource)

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

func fileExists(path string) bool {
	_, err := os.Stat(path)

	return err == nil
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

// networkSockets is the host's two multicast groups.
type networkSockets struct {
	vision *multicast.Endpoint
	gc     *multicast.Endpoint
	// detections is which camera_ids the vision socket hears, from where.
	detections *detections.Tracker
	// teams is the current match's teams, from the game controller socket;
	// heights is the robot height table they're looked up in.
	teams       *referee.Teams
	heights     *referee.Heights
	heightsFile string
	// video opens each camera's stream only while someone watches it.
	video *video.Manager

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
	s.video.Reopen()
}

// apply moves both endpoints to the store's current addresses and interface
// selection. Each endpoint reopens only if something it uses changed.
func (s *networkSockets) apply(store *config.Store) {
	doc, addrs, auto, ifaces := currentNetwork(store)

	s.vision.SetAddress(addrs.VisionAddress())
	s.gc.SetAddress(addrs.GCAddress())
	s.vision.SetInterfaces(ifaces)
	s.gc.SetInterfaces(ifaces)
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
	s.heightsFile = doc.BotHeightsFile()
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

// networkStateTopic carries each socket's address and what it has heard, plus
// the host facts the Network page's presets and notes need.
const networkStateTopic = "network.state"

func publishNetworkState(wsHub *hub.Hub, sockets *networkSockets) {
	now := time.Now()

	sockets.mu.Lock()
	auto, ifaces, heightsFile := sockets.auto, sockets.ifaces, sockets.heightsFile
	sockets.mu.Unlock()

	loopback, loopbackMulticast, _ := multicast.LoopbackMulticast()

	data, err := json.Marshal(struct {
		Vision                multicast.Status      `json:"vision"`
		GC                    multicast.Status      `json:"gc"`
		Cameras               []detections.Source   `json:"cameras"`
		Referee               referee.State         `json:"referee"`
		AutoInterfaces        bool                  `json:"autoInterfaces"`
		Interfaces            []multicast.Interface `json:"interfaces"`
		Loopback              string                `json:"loopback"`
		LoopbackMulticast     bool                  `json:"loopbackMulticast"`
		UnprivilegedPortStart int                   `json:"unprivilegedPortStart"`
		Root                  bool                  `json:"root"`
	}{
		Vision:                sockets.vision.Status(now),
		GC:                    sockets.gc.Status(now),
		Cameras:               sockets.detections.Sources(now),
		Referee:               referee.Describe(sockets.teams, sockets.heights, heightsFile),
		AutoInterfaces:        auto,
		Interfaces:            ifaces,
		Loopback:              loopback,
		LoopbackMulticast:     loopbackMulticast,
		UnprivilegedPortStart: multicast.UnprivilegedPortStart(),
		Root:                  os.Geteuid() == 0,
	})
	if err != nil {
		slog.Error("marshalling network state", "err", err)

		return
	}

	wsHub.Publish(networkStateTopic, data)
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

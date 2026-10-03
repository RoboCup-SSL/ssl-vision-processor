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
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"google.golang.org/protobuf/encoding/protojson"
)

var address = flag.String("address", ":8085", "The address on which the vision processor GUI and API are served, default: :8085")
var visionAddress = flag.String("visionAddress", "224.5.23.2:10006", "The multicast address of field vision, default: 224.5.23.2:10006")
var skipInterfaces = flag.String("skipInterfaces", "", "Comma separated list of interface names to ignore when receiving multicast packets")

// vision.yml holds everything the host owns: the field template, shared and
// per-camera vision_processor settings, and locked calibrations.
var configPath = flag.String("config", "vision.yml", "Host configuration file (field, cameras, calibrations), default: vision.yml")

// Used only when -config doesn't exist yet, to build it from a pre-vision.yml
// setup. Without -importGeometry the field comes from -geometryPreset.
var importGeometry = flag.String("importGeometry", "geometry.yml", "Legacy field geometry file to import into a new -config, default: geometry.yml")
var importConfig = flag.String("importConfig", "config.yml", "Legacy vision_processor config.yml to import into a new -config as its camera, default: config.yml")
var geometryPreset = flag.String("geometryPreset", "geometry-divB.yml", "Field preset for a new -config when there's no -importGeometry, default: geometry-divB.yml")
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

	bridge := multicast.New(*visionAddress, splitInterfaces(*skipInterfaces), level == slog.LevelDebug)
	wsHub := hub.New()

	store.OnChange(func() { publishConfigState(wsHub, store) })

	var wg sync.WaitGroup

	runBackground(&wg, "multicast bridge", func() error { return bridge.Run(ctx, geom.Absorb) })
	runBackground(&wg, "geometry publish loop", func() error {
		return geom.Run(ctx, func(encoded []byte) {
			bridge.Send(encoded)
			publishGeometryToHub(wsHub, geom)
			// Live calibration state comes from the network, not the store, so
			// it's refreshed on this tick rather than only on store changes.
			publishConfigState(wsHub, store)
		})
	})
	runBackground(&wg, "config file watcher", func() error { return store.Watch(ctx, configWatchInterval) })

	srv := &http.Server{
		Addr:              *address,
		Handler:           NewVisionServer(geom, store, wsHub, *imgDir),
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

// splitInterfaces turns the comma separated flag value into a slice, or nil
// when the flag is unset -- an unset flag must not become []string{""}.
func splitInterfaces(flagValue string) []string {
	if flagValue == "" {
		return nil
	}

	return strings.Split(flagValue, ",")
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

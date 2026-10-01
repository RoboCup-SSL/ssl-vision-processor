package main

import (
	"context"
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

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/hub"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/logging"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/multicast"
	"google.golang.org/protobuf/encoding/protojson"
)

var address = flag.String("address", ":8085", "The address on which the vision processor GUI and API are served, default: :8085")
var visionAddress = flag.String("visionAddress", "224.5.23.2:10006", "The multicast address of field vision, default: 224.5.23.2:10006")
var skipInterfaces = flag.String("skipInterfaces", "", "Comma separated list of interface names to ignore when receiving multicast packets")

// geometry-divA.yml/geometry-divB.yml are read-only rulebook presets (see
// geometry.ReadOnlyFiles) -- the default here must never be one of them, or
// the first Save in the Virtual Field editor would silently overwrite a
// tracked competition preset instead of a working copy.
var geometryFile = flag.String("geometryFile", "geometry.yml", "Field geometry config file, default: geometry.yml")
var geometryPreset = flag.String("geometryPreset", "geometry-divB.yml", "Preset to seed -geometryFile from if it doesn't exist yet, default: geometry-divB.yml")
var imgDir = flag.String("imgDir", "img", "Directory the vision processor writes debug snapshot images to, default: img")

// The vision_processor instance's own config file -- distinct from
// -geometryFile (the shared field template). Matches vision_processor's own
// default (argv[1] falling back to "config.yml", see src/main.cpp). Assumes
// a single, same-host instance, same as -imgDir; see gui/CLAUDE.md's
// "internal/discovery"/"internal/config" notes for the multi-instance plan.
var configFile = flag.String("configFile", "config.yml", "vision_processor instance config file to write calibration values into, default: config.yml")
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

	if err := bootstrapGeometryFile(*geometryFile, *geometryPreset); err != nil {
		slog.Error("bootstrapping geometry file", "err", err)

		return 1
	}

	geom, err := geometry.New(*geometryFile)
	if err != nil {
		slog.Error("loading geometry", "err", err)

		return 1
	}

	// register Ctrl+C handler
	ctx, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

	bridge := multicast.New(*visionAddress, splitInterfaces(*skipInterfaces), level == slog.LevelDebug)
	wsHub := hub.New()

	var wg sync.WaitGroup

	runBackground(&wg, "multicast bridge", func() error { return bridge.Run(ctx, geom.Absorb) })
	runBackground(&wg, "geometry publish loop", func() error {
		return geom.Run(ctx, func(encoded []byte) {
			bridge.Send(encoded)
			publishGeometryToHub(wsHub, geom)
		})
	})

	srv := &http.Server{
		Addr:              *address,
		Handler:           NewVisionServer(geom, wsHub, *imgDir, *configFile),
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

// bootstrapGeometryFile seeds path with a copy of preset if path doesn't
// exist yet, so a fresh checkout with no working geometry.yml starts from a
// real, complete config instead of failing at startup. A plain byte copy,
// not a YAML round trip, so the preset's own comments survive into the new
// working file. Never touches path if it already exists.
func bootstrapGeometryFile(path, preset string) error {
	if _, err := os.Stat(path); err == nil {
		return nil // already exists, nothing to do
	} else if !os.IsNotExist(err) {
		return err
	}

	data, err := os.ReadFile(preset)
	if err != nil {
		return fmt.Errorf("read preset %s: %w", preset, err)
	}

	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	slog.Info("seeded a fresh geometry file from preset", "path", path, "preset", preset)

	return nil
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

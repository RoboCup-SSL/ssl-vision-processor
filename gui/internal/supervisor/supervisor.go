// Package supervisor runs a vision_processor for each local camera (one with
// a config_path) and restarts it when a setting it reads only at startup
// changes. Settings it hot-reloads reach it through the regenerated
// config.yml alone. A stand-in until vision_processor accepts config over
// the network.
package supervisor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"sync"
	"syscall"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

// ErrNotSupervised means no supervised vision_processor runs that camera.
var ErrNotSupervised = errors.New("no supervised vision_processor for this camera")

// ErrHeld means the camera's vision_processor is held back: see
// Status.Conflict.
var ErrHeld = errors.New("held back: another camera uses its device")

// Process states.
const (
	StateRunning    = "running"
	StateRestarting = "restarting" // stopping, to start again at once
	StateWaiting    = "waiting"    // exited or failed to start; starts again after a delay
	StateStopped    = "stopped"
	StateConflict   = "conflict" // not started: another camera uses its device
)

// Status is one supervised vision_processor.
type Status struct {
	CameraID   int    `json:"cameraId"`
	ConfigPath string `json:"configPath"`
	State      string `json:"state" enum:"running,restarting,waiting,stopped,conflict"`
	// Conflict says which camera already uses this one's device, while State
	// is conflict.
	Conflict string `json:"conflict,omitempty"`
	PID      int    `json:"pid,omitempty"`
	// Since is when it entered State.
	Since time.Time `json:"since"`
	// Restarts counts every start after the first.
	Restarts int `json:"restarts"`
	// PendingKeys are settings changed since it started that it reads only at
	// startup. It restarts once they stop changing for a moment.
	PendingKeys []string `json:"pendingKeys"`
	// LastExit is how the previous run ended, empty before the first.
	LastExit string `json:"lastExit,omitempty"`
	// Crashes counts failed runs in a row; a run that lasts resets it.
	Crashes int `json:"crashes"`
	// Output is the last lines it printed.
	Output []string `json:"output"`
}

// Options configures a Supervisor. Zero durations take the defaults.
type Options struct {
	// Command runs vision_processor; the config path is appended.
	Command []string
	// ConfigDir is the folder the config_paths are in. Empty means the host's
	// working directory.
	ConfigDir string
	// RunDir holds each camera's working directory, cam<id>, where its
	// vision_processor writes debug files, so that no two share one. Empty
	// means the host's working directory, for every camera.
	RunDir string
	// ImgDir holds a folder per camera, cam<id>, linked as img in its
	// working directory under RunDir: where vision_processor writes its
	// snapshots. A folder each, since not every debug file it writes there
	// is prefixed by camera_id.
	ImgDir string
	// Debounce is how long startup-only settings must stay unchanged before
	// a restart, so dragging a slider restarts once. Default 1s.
	Debounce time.Duration
	// StopTimeout is how long to wait after SIGTERM before SIGKILL. It only
	// checks for SIGTERM between frames. Default 3s.
	StopTimeout time.Duration
	// RetryDelay is the wait before starting again after an exit, doubled
	// per crash in a row up to MaxRetryDelay. Defaults 1s and 30s.
	RetryDelay    time.Duration
	MaxRetryDelay time.Duration
	// OnChange runs after any status change, without the lock held.
	OnChange func()
	// Recalibrate runs, without the lock held, between stopping a camera's
	// vision_processor and starting it again after the field changed. It
	// must stop the host publishing any calibration for the camera, or the
	// new process reuses the one solved against the old field.
	Recalibrate func(cameraID int)
}

// FieldKey is the pending key for a restart to recalibrate against a
// changed field.
const FieldKey = "field"

// outputLines is how many lines of output Status keeps.
const outputLines = 20

// Supervisor runs one vision_processor per config_path.
type Supervisor struct {
	opts Options

	mu    sync.Mutex
	ctx   context.Context // nil until Run
	procs map[string]*proc
	wg    sync.WaitGroup
	// field is the last synced field and field lines, encoded; empty before
	// the first Sync.
	field string
}

// proc is one config file's vision_processor.
type proc struct {
	status  Status
	latest  map[string]any // the camera's rendered config now
	started map[string]any // what the running process was started with
	timer   *time.Timer
	restart chan struct{}
	cancel  context.CancelFunc
	starts  int
	// recalibrate is set when the field changed since it started.
	recalibrate bool
	// done is closed when the loop last spawned has returned, nil before the
	// first.
	done chan struct{}
}

// New returns a Supervisor that starts nothing until Run.
func New(opts Options) *Supervisor {
	if opts.Debounce == 0 {
		opts.Debounce = time.Second
	}

	if opts.StopTimeout == 0 {
		opts.StopTimeout = 3 * time.Second
	}

	if opts.RetryDelay == 0 {
		opts.RetryDelay = time.Second
	}

	if opts.MaxRetryDelay == 0 {
		opts.MaxRetryDelay = 30 * time.Second
	}

	return &Supervisor{opts: opts, procs: map[string]*proc{}}
}

// Sync matches the running processes to doc: one per camera with a
// config_path, which the store has already written. A camera whose
// startup-only settings changed restarts after Options.Debounce; a change to
// the field restarts every camera to recalibrate. A camera whose device
// another camera already uses is held, not started (StateConflict).
func (s *Supervisor) Sync(doc config.Document) {
	conflicts := deviceConflicts(doc)

	s.mu.Lock()

	field := fieldKey(doc)
	fieldChanged := s.field != "" && field != s.field
	s.field = field

	seen := map[string]bool{}

	for _, c := range doc.Cameras {
		if c.ConfigPath == "" || seen[c.ConfigPath] {
			continue
		}

		seen[c.ConfigPath] = true
		rendered := doc.Rendered(c)

		p := s.procs[c.ConfigPath]
		if p == nil {
			p = &proc{
				status:  Status{CameraID: c.CameraID, ConfigPath: c.ConfigPath, State: StateStopped, PendingKeys: []string{}, Output: []string{}},
				latest:  rendered,
				restart: make(chan struct{}, 1),
			}
			s.procs[c.ConfigPath] = p
			p.status.Conflict = conflicts[c.CameraID]
			s.spawnLocked(p)

			continue
		}

		p.status.CameraID = c.CameraID

		if held := conflicts[c.CameraID]; held != p.status.Conflict {
			p.status.Conflict = held
			if held != "" {
				s.holdLocked(p)
			} else {
				p.latest = rendered
				s.spawnLocked(p)
			}

			continue
		}

		if p.status.Conflict != "" {
			p.latest = rendered // starts with it once released

			continue
		}

		if fieldChanged {
			p.recalibrate = true
		} else if reflect.DeepEqual(p.latest, rendered) {
			continue
		}

		p.latest = rendered
		s.scheduleLocked(p)
	}

	for path, p := range s.procs {
		if !seen[path] {
			s.stopTimerLocked(p)

			if p.cancel != nil {
				p.cancel()
			}

			delete(s.procs, path)
		}
	}

	s.mu.Unlock()
	s.notify()
}

// Restart restarts the camera's vision_processor now.
func (s *Supervisor) Restart(cameraID int) error {
	if s == nil {
		return ErrNotSupervised
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	for _, p := range s.procs {
		if p.status.CameraID != cameraID {
			continue
		}

		if p.status.Conflict != "" {
			return ErrHeld
		}

		p.requestRestart()

		return nil
	}

	return ErrNotSupervised
}

// Statuses lists every supervised vision_processor by camera_id. Empty for
// a nil Supervisor, which supervises nothing.
func (s *Supervisor) Statuses() []Status {
	if s == nil {
		return []Status{}
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	out := make([]Status, 0, len(s.procs))

	for _, p := range s.procs {
		st := p.status
		if st.Conflict != "" {
			st.State = StateConflict
		}
		st.PendingKeys = slices.Clone(st.PendingKeys)
		st.Output = slices.Clone(st.Output)
		out = append(out, st)
	}

	slices.SortFunc(out, func(a, b Status) int { return a.CameraID - b.CameraID })

	return out
}

// Run starts every process and keeps them running until ctx is cancelled,
// then stops them all.
func (s *Supervisor) Run(ctx context.Context) error {
	s.mu.Lock()
	s.ctx = ctx

	for _, p := range s.procs {
		s.spawnLocked(p)
	}
	s.mu.Unlock()

	<-ctx.Done()
	s.wg.Wait()

	return ctx.Err()
}

func (s *Supervisor) notify() {
	if s.opts.OnChange != nil {
		s.opts.OnChange()
	}
}

// spawnLocked starts p's loop, once Run has started, unless p is held. A
// loop held and then released waits for its previous one to stop the old
// process first.
func (s *Supervisor) spawnLocked(p *proc) {
	if s.ctx == nil || s.ctx.Err() != nil || p.cancel != nil || p.status.Conflict != "" {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	previous, done := p.done, make(chan struct{})
	p.cancel, p.done = cancel, done

	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		defer close(done)

		if previous != nil {
			<-previous
		}

		s.loop(ctx, p)
	}()
}

// holdLocked stops p's loop, and with it the process, for a device conflict.
func (s *Supervisor) holdLocked(p *proc) {
	s.stopTimerLocked(p)
	p.status.PendingKeys = []string{}

	if p.cancel != nil {
		slog.Warn("holding vision_processor back", "cam", p.status.CameraID, "reason", p.status.Conflict)
		p.cancel()
		p.cancel = nil
	}
}

// scheduleLocked (re)arms p's debounced restart if its startup-only
// settings differ from what it runs with, and disarms it if not.
func (s *Supervisor) scheduleLocked(p *proc) {
	s.stopTimerLocked(p)

	p.status.PendingKeys = []string{}
	if p.started == nil {
		return // not running: it starts with p.latest anyway
	}

	keys := config.RestartKeys(p.started, p.latest)
	if p.recalibrate {
		keys = append([]string{FieldKey}, keys...)
	}

	if len(keys) == 0 {
		return
	}

	p.status.PendingKeys = keys
	p.timer = time.AfterFunc(s.opts.Debounce, p.requestRestart)
}

func (s *Supervisor) stopTimerLocked(p *proc) {
	if p.timer != nil {
		p.timer.Stop()
		p.timer = nil
	}
}

func (p *proc) requestRestart() {
	select {
	case p.restart <- struct{}{}:
	default:
	}
}

// loop runs p's process until ctx is cancelled, starting it again after
// every exit.
func (s *Supervisor) loop(ctx context.Context, p *proc) {
	defer s.setState(p, StateStopped, 0)

	for ctx.Err() == nil {
		cmd, exited, err := s.start(p)

		var delay time.Duration

		if err != nil {
			delay = s.ended(p, fmt.Sprintf("start: %v", err), true, 0)
		} else {
			began := time.Now()

			select {
			case err := <-exited:
				delay = s.ended(p, exitText(err), err != nil, time.Since(began))
			case <-p.restart:
				s.setState(p, StateRestarting, cmd.Process.Pid)
				s.stop(p, cmd, exited)

				continue
			case <-ctx.Done():
				s.stop(p, cmd, exited)

				return
			}
		}

		select {
		case <-time.After(delay):
		case <-p.restart:
		case <-ctx.Done():
		}
	}
}

// start starts p's process with its latest config.
func (s *Supervisor) start(p *proc) (*exec.Cmd, <-chan error, error) {
	s.mu.Lock()
	path, id := p.status.ConfigPath, p.status.CameraID
	p.started = p.latest
	p.status.PendingKeys = []string{}
	s.stopTimerLocked(p)
	recalibrate := p.recalibrate
	p.recalibrate = false
	s.mu.Unlock()

	// The process is stopped here, so it can't resend its old calibration
	// between this and starting again.
	if recalibrate && s.opts.Recalibrate != nil {
		slog.Info("field changed, recalibrating", "cam", id)
		s.opts.Recalibrate(id)
	}

	dir, err := s.workDir(id)
	if err != nil {
		return nil, nil, err
	}

	// Absolute, since it runs in its own directory rather than the config
	// folder.
	args := append(slices.Clone(s.opts.Command[1:]), filepath.Join(s.opts.ConfigDir, path))
	cmd := exec.Command(s.opts.Command[0], args...)
	cmd.Dir = dir
	cmd.Stdout = &lineWriter{emit: func(line string) { s.output(p, slog.LevelInfo, line) }}
	cmd.Stderr = &lineWriter{emit: func(line string) { s.output(p, slog.LevelWarn, line) }}
	cmd.SysProcAttr = sysProcAttr()
	cmd.WaitDelay = s.opts.StopTimeout

	if err := cmd.Start(); err != nil {
		return nil, nil, err
	}

	exited := make(chan error, 1)

	go func() { exited <- cmd.Wait() }()

	s.mu.Lock()
	if p.starts > 0 {
		p.status.Restarts++
	}
	p.starts++
	s.mu.Unlock()

	slog.Info("started vision_processor", "cam", id, "config", path, "pid", cmd.Process.Pid)
	s.setState(p, StateRunning, cmd.Process.Pid)

	return cmd, exited, nil
}

// stop asks cmd to stop, kills it if it doesn't in time, and waits.
func (s *Supervisor) stop(p *proc, cmd *exec.Cmd, exited <-chan error) {
	_ = cmd.Process.Signal(syscall.SIGTERM)

	var err error

	select {
	case err = <-exited:
	case <-time.After(s.opts.StopTimeout):
		slog.Warn("vision_processor ignored SIGTERM, killing it", "pid", cmd.Process.Pid)
		_ = cmd.Process.Kill()
		err = <-exited
	}

	s.mu.Lock()
	p.status.LastExit = "stopped by the host: " + exitText(err)
	s.mu.Unlock()
}

// ended records a run that ended on its own (or never started) and returns
// how long to wait before the next start. A crash right after another
// doubles the wait; a clean exit, such as a video file ending, doesn't.
func (s *Supervisor) ended(p *proc, how string, crashed bool, ran time.Duration) time.Duration {
	s.mu.Lock()

	switch {
	case !crashed:
		p.status.Crashes = 0
	case ran >= s.opts.MaxRetryDelay:
		p.status.Crashes = 1
	default:
		p.status.Crashes++
	}

	delay := s.opts.RetryDelay
	for i := 1; i < p.status.Crashes && delay < s.opts.MaxRetryDelay; i++ {
		delay *= 2
	}

	delay = min(delay, s.opts.MaxRetryDelay)
	p.status.LastExit = how
	id, path := p.status.CameraID, p.status.ConfigPath
	s.mu.Unlock()

	slog.Warn("vision_processor exited", "cam", id, "config", path, "how", how, "retryIn", delay)
	s.setState(p, StateWaiting, 0)

	return delay
}

func (s *Supervisor) setState(p *proc, state string, pid int) {
	s.mu.Lock()
	p.status.State = state
	p.status.PID = pid
	p.status.Since = time.Now()
	s.mu.Unlock()
	s.notify()
}

// output records one line the process printed and logs it.
func (s *Supervisor) output(p *proc, level slog.Level, line string) {
	s.mu.Lock()
	p.status.Output = append(p.status.Output, line)
	if n := len(p.status.Output); n > outputLines {
		p.status.Output = slices.Clone(p.status.Output[n-outputLines:])
	}
	id := p.status.CameraID
	s.mu.Unlock()

	slog.Log(context.Background(), level, "vision_processor: "+line, "cam", id)
}

// workDir makes camera id's working directory, RunDir/cam<id>, with img in it
// linking to ImgDir/cam<id>. An img left there that isn't a link is refused rather
// than removed: it may hold someone's images.
func (s *Supervisor) workDir(id int) (string, error) {
	if s.opts.RunDir == "" {
		return "", nil
	}

	dir := filepath.Join(s.opts.RunDir, fmt.Sprintf("cam%d", id))
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}

	if s.opts.ImgDir == "" {
		return dir, nil
	}

	img := filepath.Join(s.opts.ImgDir, fmt.Sprintf("cam%d", id))
	if err := os.MkdirAll(img, 0o755); err != nil {
		return "", err
	}

	link := filepath.Join(dir, "img")

	info, err := os.Lstat(link)

	switch {
	case errors.Is(err, fs.ErrNotExist):
	case err != nil:
		return "", err
	case info.Mode()&fs.ModeSymlink == 0:
		return "", fmt.Errorf("%s should link to the image directory %s, but isn't a link; move it away", link, img)
	default:
		if target, err := os.Readlink(link); err == nil && target == img {
			return dir, nil
		}

		if err := os.Remove(link); err != nil {
			return "", err
		}
	}

	return dir, os.Symlink(img, link)
}

// deviceConflicts finds the cameras in doc that would open a device a camera
// with a lower camera_id already does, keyed by camera_id, each with why. The
// host runs them all, so every camera counts whatever its instance. Device
// paths are compared with symlinks resolved, so a /dev/v4l/by-id name and
// the /dev/video node it points at match.
func deviceConflicts(doc config.Document) map[int]string {
	cameras := slices.Clone(doc.Cameras)
	slices.SortFunc(cameras, func(a, b config.Camera) int { return a.CameraID - b.CameraID })

	owners := map[string]int{}
	conflicts := map[int]string{}

	for _, c := range cameras {
		if c.ConfigPath == "" {
			continue
		}

		driver, device := doc.CaptureDevice(c)
		key := driver + " " + device

		if strings.HasPrefix(device, "/") {
			if resolved, err := filepath.EvalSymlinks(device); err == nil {
				key = driver + " " + resolved
			}
		}

		if owner, ok := owners[key]; ok {
			conflicts[c.CameraID] = fmt.Sprintf("camera %d already uses %s %s", owner, driver, device)
		} else {
			owners[key] = c.CameraID
		}
	}

	return conflicts
}

// fieldKey is what of doc a calibration is solved against.
func fieldKey(doc config.Document) string {
	data, err := json.Marshal([]any{doc.Field, doc.OptionalFieldLines})
	if err != nil {
		return ""
	}

	return string(data)
}

func exitText(err error) string {
	if err == nil {
		return "exited cleanly"
	}

	return err.Error()
}

// lineWriter calls emit once per complete line written to it.
type lineWriter struct {
	emit func(string)
	buf  []byte
}

func (w *lineWriter) Write(b []byte) (int, error) {
	w.buf = append(w.buf, b...)

	for {
		i := slices.Index(w.buf, '\n')
		if i < 0 {
			break
		}

		w.emit(string(w.buf[:i]))
		w.buf = w.buf[i+1:]
	}

	return len(b), nil
}

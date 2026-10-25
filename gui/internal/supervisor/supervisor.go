// Package supervisor runs a vision_processor for each local camera (one with
// a config_path) and restarts it when a setting it reads only at startup
// changes. Settings it hot-reloads reach it through the regenerated
// config.yml alone. A stand-in until vision_processor accepts config over
// the network.
package supervisor

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"os/exec"
	"reflect"
	"slices"
	"sync"
	"syscall"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/config"
)

// ErrNotSupervised means no supervised vision_processor runs that camera.
var ErrNotSupervised = errors.New("no supervised vision_processor for this camera")

// Process states.
const (
	StateRunning    = "running"
	StateRestarting = "restarting" // stopping, to start again at once
	StateWaiting    = "waiting"    // exited or failed to start; starts again after a delay
	StateStopped    = "stopped"
)

// Status is one supervised vision_processor.
type Status struct {
	CameraID   int    `json:"cameraId"`
	ConfigPath string `json:"configPath"`
	State      string `json:"state" enum:"running,restarting,waiting,stopped"`
	PID        int    `json:"pid,omitempty"`
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
	// Dir is its working directory, where it writes img/ and finds
	// robot-heights.yml. Empty means the host's.
	Dir string
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
}

// outputLines is how many lines of output Status keeps.
const outputLines = 20

// Supervisor runs one vision_processor per config_path.
type Supervisor struct {
	opts Options

	mu    sync.Mutex
	ctx   context.Context // nil until Run
	procs map[string]*proc
	wg    sync.WaitGroup
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
// startup-only settings changed restarts after Options.Debounce.
func (s *Supervisor) Sync(doc config.Document) {
	s.mu.Lock()

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
			s.spawnLocked(p)

			continue
		}

		p.status.CameraID = c.CameraID

		if reflect.DeepEqual(p.latest, rendered) {
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
		if p.status.CameraID == cameraID {
			p.requestRestart()

			return nil
		}
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

// spawnLocked starts p's loop, once Run has started.
func (s *Supervisor) spawnLocked(p *proc) {
	if s.ctx == nil || s.ctx.Err() != nil || p.cancel != nil {
		return
	}

	ctx, cancel := context.WithCancel(s.ctx)
	p.cancel = cancel

	s.wg.Add(1)

	go func() {
		defer s.wg.Done()
		s.loop(ctx, p)
	}()
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
	s.mu.Unlock()

	args := append(slices.Clone(s.opts.Command[1:]), path)
	cmd := exec.Command(s.opts.Command[0], args...)
	cmd.Dir = s.opts.Dir
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

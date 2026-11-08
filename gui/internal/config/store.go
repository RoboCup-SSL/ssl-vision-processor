package config

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"
	"time"

	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/geometry"
	"github.com/RoboCup-SSL/ssl-vision-processor/gui/internal/vision"
)

var (
	// ErrStale means the caller edited an older revision than the current one
	// (another browser tab got there first). Refetch and retry.
	ErrStale = errors.New("stale revision")
	// ErrDiskChanged means vision.yml was changed by something else since it
	// was last loaded or saved; saving would silently discard that change.
	ErrDiskChanged = errors.New("file changed on disk since it was loaded")
	// ErrNoLiveCalibration means Lock was asked for a camera no calibration
	// has been received for.
	ErrNoLiveCalibration = errors.New("no calibration received for this camera yet")
	// ErrUnknownCamera means the camera_id isn't in the document.
	ErrUnknownCamera = errors.New("no such camera")
)

// Applier is the live system a Store pushes the working document into.
// *geometry.Geometry implements it.
type Applier interface {
	ApplyConfig(geometry.FieldConfig, geometry.OptionalLinesConfig, *vision.SSL_GeometryModels) error
	SetLocked(map[uint32]*vision.SSL_GeometryCameraCalibration) error
	Unlock(camID uint32) error
	LiveCalibration(camID uint32) *vision.SSL_GeometryCameraCalibration
}

// Store holds the working document -- live, already applied to the running
// system -- alongside the document as last loaded from or saved to disk.
// Edits change only working; Save writes working to disk. A watcher notices
// when the file changes underneath and records it for the user to resolve.
type Store struct {
	mu sync.Mutex
	// dir is the config folder: where relative file names resolve.
	dir        string
	path       string
	disk       Document
	diskHash   string
	working    Document
	revision   int64
	external   *external
	renderErrs map[int]string
	live       Applier
	onChange   func()
	stamp      fileStamp
	// local is Options.LocalCameras.
	local bool
}

// Options configures a Store.
type Options struct {
	// LocalCameras is a host that runs every camera's vision_processor
	// itself: each camera gets a config_path (Document.AssignConfigPaths) when
	// loaded or edited without one.
	LocalCameras bool
}

// external is a version of the file on disk that differs from what the
// store last loaded or wrote.
type external struct {
	hash string
	doc  *Document // nil if it doesn't parse or validate
	err  string
}

type fileStamp struct {
	modTime time.Time
	size    int64
}

// State is the store's status as the GUI sees it.
type State struct {
	// Path is where the config file is on the host, for showing to people.
	Path string `json:"path"`
	// File is its name in the config folder, what Load and SaveAs take.
	File string `json:"file"`
	// FirstRun is whether the host is holding back publishing until the first
	// save. Clearing it changes no revision, so it's reported here.
	FirstRun bool  `json:"firstRun"`
	Revision int64 `json:"revision"`
	// Changes are what saving would write: disk -> working.
	Changes []Change `json:"changes"`
	// External is set while the file on disk differs from what was last
	// loaded or saved.
	External     *ExternalState `json:"external,omitempty"`
	Cameras      []CameraStatus `json:"cameras"`
	RenderErrors map[int]string `json:"renderErrors,omitempty"`
}

// ExternalState describes a change made to the file on disk by something
// else. Changes are what loading it would do to the working document.
type ExternalState struct {
	Error   string   `json:"error,omitempty"`
	Changes []Change `json:"changes"`
}

// Open loads path, applies it to live, and renders every local camera's
// config.yml.
func Open(path string, live Applier) (*Store, error) {
	return OpenWith(path, live, Options{})
}

// OpenWith is Open with options.
func OpenWith(path string, live Applier, opts Options) (*Store, error) {
	s := &Store{dir: filepath.Dir(path), path: path, live: live, local: opts.LocalCameras}

	if err := s.loadLocked(path); err != nil {
		return nil, err
	}

	return s, nil
}

// FirstRun reports whether the config hasn't been saved since the host created
// it.
func (s *Store) FirstRun() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.working.FirstRun
}

// InDir is the full path of a file in the config folder.
func (s *Store) InDir(name string) string {
	return filepath.Join(s.dir, name)
}

// OnChange registers fn to run (without the store's lock held) after any
// change to the store's state.
func (s *Store) OnChange(fn func()) {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.onChange = fn
}

func (s *Store) notify() {
	s.mu.Lock()
	fn := s.onChange
	s.mu.Unlock()

	if fn != nil {
		fn()
	}
}

// Working returns a copy of the working document and its revision.
func (s *Store) Working() (Document, int64) {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.working.clone(), s.revision
}

// File is a YAML file in the config folder.
type File struct {
	// Name is the file's name in the config folder.
	Name string `json:"name"`
	// Current is the file the store loads from and saves to.
	Current bool `json:"current"`
	// ReadOnly files (no write permission) can't be saved over.
	ReadOnly bool `json:"readOnly"`
}

// Files lists the .yml and .yaml files in the config folder: candidates for
// Load and SaveAs. Generated config.yml files are listed too; they don't parse
// as a vision.yml, so Load refuses them.
func (s *Store) Files() ([]File, error) {
	s.mu.Lock()
	current := filepath.Base(s.path)
	s.mu.Unlock()

	entries, err := os.ReadDir(s.dir)
	if err != nil {
		return nil, err
	}

	files := []File{}

	for _, e := range entries {
		if e.IsDir() || !slices.Contains([]string{".yml", ".yaml"}, filepath.Ext(e.Name())) {
			continue
		}

		readOnly := false

		if info, err := e.Info(); err == nil && info.Mode().Perm()&0o222 == 0 {
			readOnly = true
		}

		files = append(files, File{Name: e.Name(), Current: e.Name() == current, ReadOnly: readOnly})
	}

	return files, nil
}

// Revision is the working document's current revision.
func (s *Store) Revision() int64 {
	s.mu.Lock()
	defer s.mu.Unlock()

	return s.revision
}

// Update replaces the working document and applies it live. rev must be the
// revision the caller's edit was based on.
func (s *Store) Update(rev int64, doc Document) (int64, error) {
	next, err := s.update(rev, doc)
	if err == nil {
		s.notify()
	}

	return next, err
}

func (s *Store) update(rev int64, doc Document) (int64, error) {
	if s.local {
		s.mu.Lock()
		own := filepath.Base(s.path)
		s.mu.Unlock()

		doc.AssignConfigPaths(own)
	}

	doc.normalize()

	if err := doc.Validate(); err != nil {
		return 0, err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if rev != s.revision {
		return 0, ErrStale
	}

	if err := checkNotOverActive(doc, s.path); err != nil {
		return 0, err
	}

	doc.FirstRun = s.working.FirstRun

	if err := s.apply(doc); err != nil {
		return 0, err
	}

	// A calibration removed by the edit (e.g. a camera moved to another
	// slot) is withheld, as UnlockCalibration does, not just unlocked.
	for _, c := range s.working.Cameras {
		if next := doc.camera(c.CameraID); c.Calibration != nil && (next == nil || next.Calibration == nil) {
			if err := s.live.Unlock(uint32(c.CameraID)); err != nil {
				return 0, err
			}
		}
	}

	s.working = doc.clone()
	s.revision++

	return s.revision, nil
}

// apply pushes doc into the live system and regenerates config.yml files.
// doc must already be validated. Callers must hold mu.
func (s *Store) apply(doc Document) error {
	models, err := doc.ModelsProto()
	if err != nil {
		return err
	}

	if err := s.live.ApplyConfig(doc.Field, doc.OptionalFieldLines, models); err != nil {
		return err
	}

	locked := map[uint32]*vision.SSL_GeometryCameraCalibration{}

	for _, c := range doc.Cameras {
		if c.Calibration == nil {
			continue
		}

		calib, err := c.Calibration.Proto()
		if err != nil {
			return err
		}

		locked[uint32(c.CameraID)] = calib
	}

	if err := s.live.SetLocked(locked); err != nil {
		return err
	}

	s.renderErrs = renderAll(doc, s.dir, s.path)

	return nil
}

// checkNotOverActive refuses a document that would render a camera's config.yml
// over the config file itself.
func checkNotOverActive(doc Document, path string) error {
	name := filepath.Base(path)

	for _, c := range doc.Cameras {
		if c.ConfigPath == name {
			return &ValidationError{fmt.Errorf("cameras[%d].config_path: %q is this config file", c.CameraID, name)}
		}
	}

	return nil
}

// Save writes the working document to disk. rev must be the current
// revision. Unless force is set, a file changed on disk since it was loaded
// is not overwritten (ErrDiskChanged).
func (s *Store) Save(rev int64, force bool) error {
	err := s.save(rev, force)
	s.notify()

	return err
}

func (s *Store) save(rev int64, force bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if rev != s.revision {
		return ErrStale
	}

	if !force {
		if data, err := os.ReadFile(s.path); err == nil && hashBytes(data) != s.diskHash {
			s.recordExternal(data)

			return ErrDiskChanged
		}
	}

	return s.writeLocked(s.path)
}

// SaveAs writes the working document to name, a file in the config folder, and
// makes it the file this store loads from, saves to, and watches.
func (s *Store) SaveAs(rev int64, name string) error {
	err := s.saveAs(rev, name)
	if err == nil {
		s.notify()
	}

	return err
}

func (s *Store) saveAs(rev int64, name string) error {
	path, err := resolve(s.dir, name)
	if err != nil {
		return err
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	if rev != s.revision {
		return ErrStale
	}

	if s.working.writesOver(name) {
		return &ValidationError{fmt.Errorf("%q is used by this config for a camera or the robot height table", name)}
	}

	if err := s.writeLocked(path); err != nil {
		return err
	}

	// Regenerated so each config.yml's header names the new source.
	s.renderErrs = renderAll(s.working, s.dir, path)

	return nil
}

// writeLocked writes working to path and records it as the disk version.
// Callers must hold mu.
func (s *Store) writeLocked(path string) error {
	// Saving is the confirmation a first run waits for, once it has reached
	// the disk.
	saved := s.working
	saved.FirstRun = false

	data, err := Marshal(saved)
	if err != nil {
		return err
	}

	if err := atomicWrite(path, data); errors.Is(err, errReadOnly) {
		return &ValidationError{err}
	} else if err != nil {
		return fmt.Errorf("write %s: %w", path, err)
	}

	s.working.FirstRun = false
	s.path = path
	s.disk = s.working.clone()
	s.diskHash = hashBytes(data)
	s.external = nil
	s.stamp = statFile(path)

	return nil
}

// Load replaces both the working and disk documents with the contents of name,
// a file in the config folder, applies it live, and switches to editing it.
func (s *Store) Load(name string) error {
	path, err := resolve(s.dir, name)
	if err != nil {
		return err
	}

	return s.load(path)
}

func (s *Store) load(path string) error {
	s.mu.Lock()
	err := s.loadLocked(path)
	s.mu.Unlock()

	if err == nil {
		s.notify()
	}

	return err
}

// Reload is Load of the current path: it accepts whatever is on disk now,
// discarding unsaved working changes.
func (s *Store) Reload() error {
	s.mu.Lock()
	path := s.path
	s.mu.Unlock()

	return s.load(path)
}

func (s *Store) loadLocked(path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return &ValidationError{fmt.Errorf("read %s: %w", path, err)}
	}

	// Parse only fails on content, so this stays a ValidationError.
	doc, err := Parse(data)
	if err != nil {
		return &ValidationError{prefixed(path, err)}
	}

	// The file as it is, so assigned config_paths show as unsaved changes.
	disk := doc.clone()

	if s.local {
		doc.AssignConfigPaths(filepath.Base(path))
	}

	if err := checkNotOverActive(doc, path); err != nil {
		return err
	}

	previousPath := s.path
	s.path = path

	if err := s.apply(doc); err != nil {
		s.path = previousPath

		return err
	}

	s.working = doc
	s.disk = disk
	s.diskHash = hashBytes(data)
	s.external = nil
	s.stamp = statFile(path)
	s.revision++

	return nil
}

// LockCalibration stores the latest calibration received for camID in the
// working document, so it's published in place of anything the camera sends
// and survives restarts once saved.
func (s *Store) LockCalibration(camID int) (int64, error) {
	rev, err := s.editCamera(camID, func(doc *Document, c *Camera) error {
		live := s.live.LiveCalibration(uint32(camID))
		if live == nil {
			return ErrNoLiveCalibration
		}

		m, err := protoMap(live)
		if err != nil {
			return err
		}

		c.Calibration = &Calibration{
			LockedAt:  time.Now().UTC().Truncate(time.Second),
			FieldHash: fieldHash(doc.Field),
			Camera:    m,
		}

		return nil
	})
	if err == nil {
		s.notify()
	}

	return rev, err
}

// UnlockCalibration removes camID's locked calibration and stops publishing
// any calibration for it, so its vision_processor calibrates the next time
// it starts.
func (s *Store) UnlockCalibration(camID int) (int64, error) {
	rev, err := s.editCamera(camID, func(_ *Document, c *Camera) error {
		c.Calibration = nil

		return s.live.Unlock(uint32(camID))
	})
	if err == nil {
		s.notify()
	}

	return rev, err
}

func (s *Store) editCamera(camID int, edit func(*Document, *Camera) error) (int64, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	doc := s.working.clone()

	c := doc.camera(camID)
	if c == nil {
		return 0, ErrUnknownCamera
	}

	if err := edit(&doc, c); err != nil {
		return 0, err
	}

	if err := s.apply(doc); err != nil {
		return 0, err
	}

	s.working = doc
	s.revision++

	return s.revision, nil
}

// State reports the store's current status, including live per-camera state.
func (s *Store) State() State {
	s.mu.Lock()
	defer s.mu.Unlock()

	state := State{
		Path:         s.path,
		File:         filepath.Base(s.path),
		FirstRun:     s.working.FirstRun,
		Revision:     s.revision,
		Changes:      nonNil(Diff(s.disk, s.working)),
		Cameras:      make([]CameraStatus, 0, len(s.working.Cameras)),
		RenderErrors: s.renderErrs,
	}

	for _, c := range s.working.Cameras {
		state.Cameras = append(state.Cameras, cameraStatus(s.working, c, s.live.LiveCalibration(uint32(c.CameraID))))
	}

	if s.external != nil {
		ext := &ExternalState{Error: s.external.err, Changes: []Change{}}
		if s.external.doc != nil {
			ext.Changes = nonNil(Diff(s.working, *s.external.doc))
		}

		state.External = ext
	}

	return state
}

// Watch polls the file every interval until ctx is cancelled, recording a
// change made by anything other than this store.
func (s *Store) Watch(ctx context.Context, interval time.Duration) error {
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-ticker.C:
			if s.checkDisk() {
				s.notify()
			}
		}
	}
}

// checkDisk reports whether the external-change state changed.
func (s *Store) checkDisk() bool {
	s.mu.Lock()
	defer s.mu.Unlock()

	stamp := statFile(s.path)
	if stamp == s.stamp {
		return false
	}

	s.stamp = stamp

	data, err := os.ReadFile(s.path)
	if err != nil {
		return false // mid-replace or deleted; the next tick will see it
	}

	if hashBytes(data) == s.diskHash {
		cleared := s.external != nil
		s.external = nil

		return cleared
	}

	return s.recordExternal(data)
}

// recordExternal notes data as the on-disk version, reporting whether it's
// new. Callers must hold mu.
func (s *Store) recordExternal(data []byte) bool {
	hash := hashBytes(data)
	if s.external != nil && s.external.hash == hash {
		return false
	}

	ext := &external{hash: hash}

	if doc, err := Parse(data); err != nil {
		ext.err = err.Error()
	} else {
		ext.doc = &doc
	}

	s.external = ext

	return true
}

func statFile(path string) fileStamp {
	info, err := os.Stat(path)
	if err != nil {
		return fileStamp{}
	}

	return fileStamp{modTime: info.ModTime(), size: info.Size()}
}

func hashBytes(data []byte) string {
	sum := sha256.Sum256(data)

	return hex.EncodeToString(sum[:])
}

func nonNil(changes []Change) []Change {
	if changes == nil {
		return []Change{}
	}

	return changes
}

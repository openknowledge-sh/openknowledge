package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
)

const (
	viewerLiveReloadDebounce       = 250 * time.Millisecond
	viewerLiveReloadReconcile      = 2 * time.Second
	viewerLiveReloadHeartbeat      = 15 * time.Second
	viewerLiveReloadMaxSubscribers = 64
)

type viewerLiveReloadRoot struct {
	Alias string
	Root  string
}

type viewerLiveReloadRootSource func() ([]viewerLiveReloadRoot, error)

type viewerLiveReloadEvent struct {
	Revision       string   `json:"revision,omitempty"`
	KnowledgeBases []string `json:"knowledgeBases,omitempty"`
	Status         string   `json:"status,omitempty"`
	Message        string   `json:"message,omitempty"`
}

type viewerLiveReloadSubscription struct {
	id     uint64
	events chan viewerLiveReloadEvent
}

type viewerLiveReloadFileRevision struct {
	size    int64
	modTime int64
	digest  string
}

type viewerLiveReload struct {
	ctx       context.Context
	cancel    context.CancelFunc
	watcher   *fsnotify.Watcher
	roots     viewerLiveReloadRootSource
	trigger   chan struct{}
	done      chan struct{}
	closeOnce sync.Once

	mu          sync.Mutex
	revision    string
	subvisions  map[string]string
	subscribers map[uint64]chan viewerLiveReloadEvent
	nextID      uint64
	watched     map[string]bool
	files       map[string]viewerLiveReloadFileRevision
	dirtyPaths  map[string]bool
}

func newViewerLiveReload(parent context.Context, roots viewerLiveReloadRootSource) (*viewerLiveReload, error) {
	watcher, err := fsnotify.NewBufferedWatcher(256)
	if err != nil {
		return nil, err
	}
	ctx, cancel := context.WithCancel(parent)
	hub := &viewerLiveReload{
		ctx:         ctx,
		cancel:      cancel,
		watcher:     watcher,
		roots:       roots,
		trigger:     make(chan struct{}, 1),
		done:        make(chan struct{}),
		subvisions:  make(map[string]string),
		subscribers: make(map[uint64]chan viewerLiveReloadEvent),
		watched:     make(map[string]bool),
		files:       make(map[string]viewerLiveReloadFileRevision),
		dirtyPaths:  make(map[string]bool),
	}
	if err := hub.refresh(true); err != nil {
		_ = watcher.Close()
		cancel()
		return nil, err
	}
	go hub.run()
	return hub, nil
}

func (hub *viewerLiveReload) Close() error {
	var closeErr error
	hub.closeOnce.Do(func() {
		hub.cancel()
		closeErr = hub.watcher.Close()
		<-hub.done
	})
	return closeErr
}

func (hub *viewerLiveReload) run() {
	defer close(hub.done)
	reconcile := time.NewTicker(viewerLiveReloadReconcile)
	defer reconcile.Stop()
	var timer *time.Timer
	var timerC <-chan time.Time
	schedule := func() {
		if timer == nil {
			timer = time.NewTimer(viewerLiveReloadDebounce)
			timerC = timer.C
			return
		}
		if !timer.Stop() {
			select {
			case <-timer.C:
			default:
			}
		}
		timer.Reset(viewerLiveReloadDebounce)
		timerC = timer.C
	}
	for {
		select {
		case <-hub.ctx.Done():
			if timer != nil {
				timer.Stop()
			}
			return
		case event, ok := <-hub.watcher.Events:
			if !ok {
				return
			}
			if strings.TrimSpace(event.Name) != "" {
				hub.dirtyPaths[filepath.Clean(event.Name)] = true
			}
			schedule()
		case err, ok := <-hub.watcher.Errors:
			if !ok {
				return
			}
			message := "Live reload is rescanning after a file watch error."
			if errors.Is(err, fsnotify.ErrEventOverflow) {
				message = "Live reload is rescanning after too many file changes."
			}
			hub.publishStatus(message)
			schedule()
		case <-hub.trigger:
			schedule()
		case <-reconcile.C:
			if err := hub.refresh(false); err != nil {
				hub.publishStatus("Live reload waits for readable source files.")
			}
		case <-timerC:
			timerC = nil
			if err := hub.refresh(false); err != nil {
				hub.publishStatus("Live reload waits for readable source files.")
			}
		}
	}
}

func (hub *viewerLiveReload) refresh(initial bool) error {
	roots, err := hub.roots()
	if err != nil {
		return err
	}
	roots = normalizeViewerLiveReloadRoots(roots)
	if err := hub.reconcileWatches(roots); err != nil {
		return err
	}
	revision, subvisions, files, _, err := viewerLiveReloadRevisionCached(roots, hub.files, hub.dirtyPaths)
	if err != nil {
		return err
	}
	hub.files = files
	hub.dirtyPaths = make(map[string]bool)

	hub.mu.Lock()
	previous := hub.revision
	previousSubs := hub.subvisions
	hub.revision = revision
	hub.subvisions = subvisions
	affected := changedViewerLiveReloadAliases(previousSubs, subvisions)
	hub.mu.Unlock()
	if !initial && previous != revision {
		hub.publish(viewerLiveReloadEvent{Revision: revision, KnowledgeBases: affected})
	}
	return nil
}

func normalizeViewerLiveReloadRoots(roots []viewerLiveReloadRoot) []viewerLiveReloadRoot {
	normalized := make([]viewerLiveReloadRoot, 0, len(roots))
	seen := make(map[string]bool)
	for _, root := range roots {
		absolute, err := filepath.Abs(root.Root)
		if err != nil {
			continue
		}
		absolute = filepath.Clean(absolute)
		key := root.Alias + "\x00" + absolute
		if seen[key] {
			continue
		}
		seen[key] = true
		normalized = append(normalized, viewerLiveReloadRoot{Alias: root.Alias, Root: absolute})
	}
	sort.Slice(normalized, func(left int, right int) bool {
		if normalized[left].Alias == normalized[right].Alias {
			return normalized[left].Root < normalized[right].Root
		}
		return normalized[left].Alias < normalized[right].Alias
	})
	return normalized
}

func (hub *viewerLiveReload) reconcileWatches(roots []viewerLiveReloadRoot) error {
	desired := make(map[string]bool)
	for _, root := range roots {
		err := filepath.WalkDir(root.Root, func(current string, entry fs.DirEntry, walkErr error) error {
			if walkErr != nil {
				return walkErr
			}
			if !entry.IsDir() {
				return nil
			}
			if current != root.Root && viewerLiveReloadIgnoredName(entry.Name(), true) {
				return filepath.SkipDir
			}
			desired[filepath.Clean(current)] = true
			return nil
		})
		if err != nil {
			return err
		}
	}
	for watched := range hub.watched {
		if desired[watched] {
			continue
		}
		_ = hub.watcher.Remove(watched)
		delete(hub.watched, watched)
	}
	for directory := range desired {
		if hub.watched[directory] {
			continue
		}
		if err := hub.watcher.Add(directory); err != nil {
			return err
		}
		hub.watched[directory] = true
	}
	return nil
}

func viewerLiveReloadRevision(roots []viewerLiveReloadRoot) (string, map[string]string, error) {
	revision, subvisions, _, _, err := viewerLiveReloadRevisionCached(roots, nil, nil)
	return revision, subvisions, err
}

func viewerLiveReloadRevisionCached(roots []viewerLiveReloadRoot, previous map[string]viewerLiveReloadFileRevision, dirtyPaths map[string]bool) (string, map[string]string, map[string]viewerLiveReloadFileRevision, int, error) {
	global := sha256.New()
	subvisions := make(map[string]string, len(roots))
	next := make(map[string]viewerLiveReloadFileRevision)
	filesRead := 0
	for _, root := range roots {
		digest, read, err := viewerLiveReloadRootRevisionCached(root.Root, previous, next, dirtyPaths)
		if err != nil {
			return "", nil, nil, filesRead, err
		}
		filesRead += read
		identity := sha256.Sum256([]byte(root.Root))
		_, _ = fmt.Fprintf(global, "%s\x00%x\x00%s\x00", root.Alias, identity, digest)
		subvisions[root.Alias] = digest + ":" + hex.EncodeToString(identity[:])
	}
	return hex.EncodeToString(global.Sum(nil)), subvisions, next, filesRead, nil
}

func viewerLiveReloadRootRevision(root string) (string, error) {
	digest, _, err := viewerLiveReloadRootRevisionCached(root, nil, make(map[string]viewerLiveReloadFileRevision), nil)
	return digest, err
}

func viewerLiveReloadRootRevisionCached(root string, previous map[string]viewerLiveReloadFileRevision, next map[string]viewerLiveReloadFileRevision, dirtyPaths map[string]bool) (string, int, error) {
	hash := sha256.New()
	filesRead := 0
	err := filepath.WalkDir(root, func(current string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if current == root {
			return nil
		}
		if viewerLiveReloadIgnoredName(entry.Name(), entry.IsDir()) {
			if entry.IsDir() {
				return filepath.SkipDir
			}
			return nil
		}
		if entry.Type()&os.ModeSymlink != 0 {
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		if !info.Mode().IsRegular() {
			return nil
		}
		current = filepath.Clean(current)
		rel, err := filepath.Rel(root, current)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		state, cached := previous[current]
		if !cached || state.size != info.Size() || state.modTime != info.ModTime().UnixNano() || dirtyPaths[current] {
			fileHash := sha256.New()
			file, err := os.Open(current)
			if err != nil {
				return err
			}
			reader := bufio.NewReader(file)
			_, copyErr := io.Copy(fileHash, reader)
			closeErr := file.Close()
			if copyErr != nil {
				return copyErr
			}
			if closeErr != nil {
				return closeErr
			}
			state = viewerLiveReloadFileRevision{
				size:    info.Size(),
				modTime: info.ModTime().UnixNano(),
				digest:  hex.EncodeToString(fileHash.Sum(nil)),
			}
			filesRead++
		}
		next[current] = state
		_, _ = io.WriteString(hash, rel)
		_, _ = io.WriteString(hash, "\x00file\x00")
		_, _ = io.WriteString(hash, state.digest)
		_, _ = io.WriteString(hash, "\x00")
		return nil
	})
	if err != nil {
		return "", filesRead, err
	}
	return hex.EncodeToString(hash.Sum(nil)), filesRead, nil
}

func viewerLiveReloadIgnoredName(name string, directory bool) bool {
	lower := strings.ToLower(strings.TrimSpace(name))
	if lower == "" || lower == ".git" || strings.HasPrefix(lower, ".") {
		return true
	}
	if lower == "openknowledge.toml" || strings.HasSuffix(lower, "~") || strings.HasPrefix(lower, "#") && strings.HasSuffix(lower, "#") {
		return true
	}
	if directory {
		return false
	}
	for _, suffix := range []string{".swp", ".swo", ".tmp", ".temp"} {
		if strings.HasSuffix(lower, suffix) {
			return true
		}
	}
	return false
}

func changedViewerLiveReloadAliases(before map[string]string, after map[string]string) []string {
	changed := make(map[string]bool)
	for alias, revision := range before {
		if after[alias] != revision {
			changed[alias] = true
		}
	}
	for alias, revision := range after {
		if before[alias] != revision {
			changed[alias] = true
		}
	}
	aliases := make([]string, 0, len(changed))
	for alias := range changed {
		if alias != "" {
			aliases = append(aliases, alias)
		}
	}
	sort.Strings(aliases)
	return aliases
}

func (hub *viewerLiveReload) publishStatus(message string) {
	hub.mu.Lock()
	revision := hub.revision
	hub.mu.Unlock()
	hub.publish(viewerLiveReloadEvent{Revision: revision, Status: "waiting", Message: message})
}

func (hub *viewerLiveReload) publish(event viewerLiveReloadEvent) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	for _, subscriber := range hub.subscribers {
		select {
		case subscriber <- event:
		default:
			select {
			case <-subscriber:
			default:
			}
			select {
			case subscriber <- event:
			default:
			}
		}
	}
}

func (hub *viewerLiveReload) subscribe() (viewerLiveReloadSubscription, viewerLiveReloadEvent, bool) {
	hub.mu.Lock()
	defer hub.mu.Unlock()
	if len(hub.subscribers) >= viewerLiveReloadMaxSubscribers {
		return viewerLiveReloadSubscription{}, viewerLiveReloadEvent{}, false
	}
	hub.nextID++
	subscription := viewerLiveReloadSubscription{id: hub.nextID, events: make(chan viewerLiveReloadEvent, 1)}
	hub.subscribers[subscription.id] = subscription.events
	return subscription, viewerLiveReloadEvent{Revision: hub.revision}, true
}

func (hub *viewerLiveReload) unsubscribe(subscription viewerLiveReloadSubscription) {
	hub.mu.Lock()
	delete(hub.subscribers, subscription.id)
	hub.mu.Unlock()
}

func viewerLiveReloadHandler(next http.Handler, hub *viewerLiveReload) http.Handler {
	if hub == nil {
		return next
	}
	return http.HandlerFunc(func(response http.ResponseWriter, request *http.Request) {
		if request.URL.Path != "/api/viewer-events" {
			next.ServeHTTP(response, request)
			return
		}
		hub.serveEvents(response, request)
	})
}

func (hub *viewerLiveReload) serveEvents(response http.ResponseWriter, request *http.Request) {
	if request.Method != http.MethodGet {
		response.Header().Set("Allow", http.MethodGet)
		http.Error(response, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	flusher, ok := response.(http.Flusher)
	if !ok {
		http.Error(response, "streaming is not supported", http.StatusInternalServerError)
		return
	}
	subscription, initial, ok := hub.subscribe()
	if !ok {
		response.Header().Set("Retry-After", "3")
		http.Error(response, "too many live reload clients", http.StatusServiceUnavailable)
		return
	}
	defer hub.unsubscribe(subscription)
	response.Header().Set("Content-Type", "text/event-stream")
	response.Header().Set("Cache-Control", "no-store")
	response.Header().Set("X-Accel-Buffering", "no")
	_, _ = io.WriteString(response, "retry: 3000\n")
	eventName := "ready"
	if last := strings.TrimSpace(request.Header.Get("Last-Event-ID")); last != "" && last != initial.Revision {
		eventName = "change"
	}
	if err := writeViewerLiveReloadEvent(response, eventName, initial); err != nil {
		return
	}
	flusher.Flush()
	heartbeat := time.NewTicker(viewerLiveReloadHeartbeat)
	defer heartbeat.Stop()
	for {
		select {
		case <-request.Context().Done():
			return
		case <-hub.ctx.Done():
			return
		case event := <-subscription.events:
			name := "change"
			if event.Status != "" {
				name = "status"
			}
			if err := writeViewerLiveReloadEvent(response, name, event); err != nil {
				return
			}
			flusher.Flush()
		case <-heartbeat.C:
			if _, err := io.WriteString(response, ": heartbeat\n\n"); err != nil {
				return
			}
			flusher.Flush()
		}
	}
}

func writeViewerLiveReloadEvent(writer io.Writer, name string, event viewerLiveReloadEvent) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return err
	}
	if event.Revision != "" {
		if _, err := fmt.Fprintf(writer, "id: %s\n", event.Revision); err != nil {
			return err
		}
	}
	_, err = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", name, payload)
	return err
}

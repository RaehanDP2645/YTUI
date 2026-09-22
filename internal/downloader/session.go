package downloader

import (
	"context"
	"errors"
	"strings"
	"sync"
)

// batchSessionKeyType membedakan key context session dari key lain.
type batchSessionKeyType struct{}

var batchSessionKey batchSessionKeyType

// BatchItem menyimpan state satu URL di dalam BatchSession.
type BatchItem struct {
	URL       string
	Status    string // queued | downloading | completed | failed | canceled
	Percent   float64
	Message   string
	Type      DownloadType
	Quality   string
	OutputDir string
}

// BatchSession menyimpan state batch yang tetap bertahan setelah DownloadBatch
// (yang single-shot/stateless) selesai, sehingga item gagal bisa di-retry satu
// per satu tanpa mengulang seluruh batch. Identity item = URL, konsisten dengan
// key yang dipakai frontend dan progressTracker.
type BatchSession struct {
	mu      sync.Mutex
	base    context.Context // konteks app yang panjang umurnya (tidak pernah di-cancel)
	tracker *progressTracker

	items    map[string]*BatchItem
	retrying map[string]bool // URL yang sedang di-retry (anti duplicate download)
}

func newBatchSession(ctx context.Context, req BatchDownloadRequest, urls []string) *BatchSession {
	s := &BatchSession{
		base:     ctx,
		tracker:  newProgressTracker(),
		items:    make(map[string]*BatchItem, len(urls)),
		retrying: make(map[string]bool),
	}

	for _, url := range urls {
		s.items[url] = &BatchItem{
			URL:       url,
			Status:    "queued",
			Type:      req.Type,
			Quality:   req.Quality,
			OutputDir: req.OutputDir,
		}
	}

	return s
}

// applyEvent menyinkronkan state item dari aliran event progress yang sama
// dengan yang dipakai frontend dan progressTracker, termasuk event dari retry.
func (s *BatchSession) applyEvent(ev ProgressEvent) {
	if ev.URL == "" {
		return
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	item, ok := s.items[ev.URL]
	if !ok {
		item = &BatchItem{URL: ev.URL}
		s.items[ev.URL] = item
	}

	if ev.Status != "" {
		item.Status = ev.Status
	}
	if ev.Percent > 0 {
		item.Percent = ev.Percent
	}
	if ev.Message != "" {
		item.Message = ev.Message
	}
}

// Status mengembalikan status terakhir sebuah URL, atau "" jika tidak dikenal.
func (s *BatchSession) Status(url string) string {
	s.mu.Lock()
	defer s.mu.Unlock()

	if item, ok := s.items[url]; ok {
		return item.Status
	}
	return ""
}

var (
	errItemUnknown     = errors.New("item tidak dikenal")
	errRetryNotFailed  = errors.New("hanya item dengan status failed yang bisa di-retry")
	errRetryInProgress = errors.New("item sedang di-retry")
)

// executeItem adalah titik injection untuk test deterministik (default = DownloadDefault).
// Worker batch tetap memanggil DownloadDefault langsung; seam ini hanya dipakai Retry.
var executeItem = DownloadDefault

// Retry mengulang download SATU item yang berstatus failed. Memakai flow download
// yang sudah ada (DownloadDefault) dengan konteks baru, bukan konteks batch lama
// yang sudah selesai. Duplicate retry untuk URL yang sama dicegah.
func (s *BatchSession) Retry(ctx context.Context, url string) (DownloadResult, error) {
	url = strings.TrimSpace(url)

	s.mu.Lock()
	item, ok := s.items[url]
	if !ok {
		s.mu.Unlock()
		return DownloadResult{}, errItemUnknown
	}
	if item.Status != "failed" {
		s.mu.Unlock()
		return DownloadResult{}, errRetryNotFailed
	}
	if s.retrying[url] {
		s.mu.Unlock()
		return DownloadResult{}, errRetryInProgress
	}
	s.retrying[url] = true

	// Reset status item: kembali antri, progress 0, hapus pesan error lama.
	// Pengaturan Type/Quality/OutputDir memakai nilai asli item dari batch.
	item.Status = "queued"
	item.Percent = 0
	item.Message = ""
	req := DownloadRequest{
		URL:       item.URL,
		Type:      item.Type,
		Quality:   item.Quality,
		OutputDir: item.OutputDir,
	}
	s.mu.Unlock()

	defer func() {
		s.mu.Lock()
		delete(s.retrying, url)
		s.mu.Unlock()
	}()

	// Konteks retry dibangun dari ctx panggilan (di app: context.WithCancel(a.ctx)),
	// TIDAK memakai konteks batch lama. Tracker & session disuntikkan supaya seluruh
	// event retry tetap meng-update overall progress dan state item.
	retryCtx := context.WithValue(ctx, progressTrackerKey, s.tracker)
	retryCtx = context.WithValue(retryCtx, batchSessionKey, s)

	// Item kembali 0% (overall ikut turun sementara), diikuti status kartu ke "queued".
	emitProgress(retryCtx, ProgressEvent{
		URL:     url,
		Status:  "queued",
		Message: "Retry: antri ulang",
	})

	result, err := executeItem(retryCtx, req)

	// Jalur error yang tidak mengirim event terminal (mis. gagal menentukan nama
	// file output di awal DownloadDefault) dipastikan tercatat "failed". Jika event
	// terminal sudah muncul (completed/canceled/failed), tidak ditimpa.
	s.mu.Lock()
	current, exists := s.items[url]
	status := ""
	if exists {
		status = current.Status
	}
	alreadyTerminal := status == "completed" || status == "failed" || status == "canceled"
	s.mu.Unlock()

	if err != nil && !alreadyTerminal {
		emitProgress(retryCtx, ProgressEvent{
			URL:     url,
			Status:  "failed",
			Message: err.Error(),
		})
	}

	return result, err
}

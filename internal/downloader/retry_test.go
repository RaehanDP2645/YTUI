package downloader

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
)

// eventRecorder menangkap event yang dikirim lewat seam emitEvent.
type eventRecorder struct {
	mu     sync.Mutex
	events []ProgressEvent
}

func (r *eventRecorder) add(ev ProgressEvent) {
	r.mu.Lock()
	r.events = append(r.events, ev)
	r.mu.Unlock()
}

func (r *eventRecorder) overallValues() []float64 {
	r.mu.Lock()
	defer r.mu.Unlock()

	var values []float64
	for _, ev := range r.events {
		if ev.Status == "overall" {
			values = append(values, ev.Percent)
		}
	}
	return values
}

func (r *eventRecorder) containsOverall(want float64) bool {
	for _, v := range r.overallValues() {
		if v == want {
			return true
		}
	}
	return false
}

// installRetrySeams mengganti executeItem & emitEvent untuk test deterministik.
func installRetrySeams(t *testing.T, ejec func(ctx context.Context, req DownloadRequest) (DownloadResult, error)) *eventRecorder {
	t.Helper()

	rec := &eventRecorder{}
	oldEmit := emitEvent
	oldExec := executeItem

	emitEvent = func(ctx context.Context, eventName string, data ...interface{}) {
		if len(data) >= 1 {
			if ev, ok := data[0].(ProgressEvent); ok {
				rec.add(ev)
			}
		}
	}
	executeItem = ejec

	t.Cleanup(func() {
		emitEvent = oldEmit
		executeItem = oldExec
	})

	return rec
}

// newFailedSession membuat session batch dengan seluruh item berstatus failed
// (mensimulasikan hasil batch yang sudah selesai dengan beberapa kegagalan).
func newFailedSession(t *testing.T, urls ...string) *BatchSession {
	t.Helper()

	ctx := context.Background()
	s := newBatchSession(ctx, BatchDownloadRequest{
		Type:      DownloadTypeVideo,
		Quality:   "720p",
		OutputDir: "C:\\test-out",
	}, urls)

	for _, u := range urls {
		s.applyEvent(ProgressEvent{URL: u, Status: "failed", Message: "error asli"})
	}

	return s
}

func TestRetryRejectsCompletedItem(t *testing.T) {
	installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		return DownloadResult{}, nil
	})

	s := newFailedSession(t, "https://example.com/a")
	s.applyEvent(ProgressEvent{URL: "https://example.com/a", Status: "completed"})

	_, err := s.Retry(context.Background(), "https://example.com/a")
	if !errors.Is(err, errRetryNotFailed) {
		t.Fatalf("retry item completed harus ditolak, dapat: %v", err)
	}
	if got := s.Status("https://example.com/a"); got != "completed" {
		t.Fatalf("status item tidak boleh berubah, dapat: %s", got)
	}
}

func TestRetryRejectsUnknownURL(t *testing.T) {
	installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		return DownloadResult{}, nil
	})

	s := newFailedSession(t, "https://example.com/a")

	_, err := s.Retry(context.Background(), "https://example.com/tidak-dikenal")
	if !errors.Is(err, errItemUnknown) {
		t.Fatalf("URL tidak dikenal harus ditolak, dapat: %v", err)
	}
}

func TestRetrySuccessFlow(t *testing.T) {
	var called int
	rec := installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		called++
		if req.Type != DownloadTypeVideo || req.Quality != "720p" || req.OutputDir != "C:\\test-out" {
			t.Errorf("pengaturan item tidak diteruskan: %+v", req)
		}
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "downloading", Percent: 60})
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "completed", Percent: 100, Message: "selesai"})
		return DownloadResult{Message: "selesai", OutputDir: req.OutputDir}, nil
	})

	url := "https://example.com/a"
	s := newFailedSession(t, url)

	res, err := s.Retry(context.Background(), url)
	if err != nil {
		t.Fatalf("retry sukses harus berhasil, dapat: %v", err)
	}
	if res.Message != "selesai" {
		t.Fatalf("hasil retry salah: %+v", res)
	}
	if called != 1 {
		t.Fatalf("executeItem harus dipanggil 1x, dapat %d", called)
	}
	if got := s.Status(url); got != "completed" {
		t.Fatalf("status harus completed, dapat: %s", got)
	}
	if !rec.containsOverall(100) {
		t.Fatalf("overall harus mencapai 100, events: %v", rec.overallValues())
	}
}

func TestRetryFailAgainThenAllowed(t *testing.T) {
	var calls int32
	rec := installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		n := atomic.AddInt32(&calls, 1)
		if n == 1 {
			emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "failed", Message: "gagal lagi"})
			return DownloadResult{}, errors.New("gagal lagi")
		}
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "completed", Percent: 100, Message: "berhasil"})
		return DownloadResult{Message: "berhasil", OutputDir: req.OutputDir}, nil
	})

	url := "https://example.com/a"
	s := newFailedSession(t, url)

	_, err := s.Retry(context.Background(), url)
	if err == nil {
		t.Fatal("retry yang gagal lagi harus mengembalikan error")
	}
	if got := s.Status(url); got != "failed" {
		t.Fatalf("status harus failed, dapat: %s", got)
	}
	if !rec.containsOverall(100) {
		t.Fatalf("overall harus kembali 100 setelah gagal, events: %v", rec.overallValues())
	}

	if s.Status(url) != "failed" {
		t.Fatalf("retry berikutnya harus tersedia (status failed)")
	}
	if _, err := s.Retry(context.Background(), url); err != nil {
		t.Fatalf("retry kedua harus diizinkan, dapat: %v", err)
	}
	if got := s.Status(url); got != "completed" {
		t.Fatalf("status setelah retry kedua harus completed, dapat: %s", got)
	}
}

func TestRetryDuplicateConcurrent(t *testing.T) {
	started := make(chan struct{})
	release := make(chan struct{})
	var execCalls int32

	rec := installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		atomic.AddInt32(&execCalls, 1)
		close(started)

		select {
		case <-release:
		case <-ctx.Done():
			emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "canceled"})
			return DownloadResult{}, errors.New("dibatalkan")
		}

		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "completed", Percent: 100, Message: "selesai"})
		return DownloadResult{Message: "selesai"}, nil
	})

	url := "https://example.com/a"
	s := newFailedSession(t, url)

	var wg sync.WaitGroup
	var firstErr error

	wg.Add(1)
	go func() {
		defer wg.Done()
		_, firstErr = s.Retry(context.Background(), url)
	}()

	<-started // retry pertama sedang berjalan & sudah menandai map retrying

	_, secondErr := s.Retry(context.Background(), url)
	if secondErr == nil {
		t.Fatal("retry duplikat saat in-flight harus ditolak")
	}

	close(release)
	wg.Wait()

	if firstErr != nil {
		t.Fatalf("retry pertama harus sukses, dapat: %v", firstErr)
	}
	if got := atomic.LoadInt32(&execCalls); got != 1 {
		t.Fatalf("executeItem harus dipanggil tepat 1x, dapat %d", got)
	}
	if !rec.containsOverall(100) {
		t.Fatalf("overall harus 100, events: %v", rec.overallValues())
	}
}

func TestRetryOnlyAffectsTargetItem(t *testing.T) {
	var mu sync.Mutex
	var calledURLs []string

	installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		mu.Lock()
		calledURLs = append(calledURLs, req.URL)
		mu.Unlock()

		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "completed", Percent: 100, Message: "selesai"})
		return DownloadResult{Message: "selesai"}, nil
	})

	a := "https://example.com/zza"
	b := "https://example.com/zzb"
	c := "https://example.com/zzc"
	s := newFailedSession(t, a, b, c)

	if _, err := s.Retry(context.Background(), c); err != nil {
		t.Fatalf("retry c gagal: %v", err)
	}

	mu.Lock()
	defer mu.Unlock()
	if len(calledURLs) != 1 || calledURLs[0] != c {
		t.Fatalf("hanya c yang boleh diulang, dipanggil: %v", calledURLs)
	}
	if got := s.Status(a); got != "failed" {
		t.Fatalf("a tidak boleh berubah, dapat: %s", got)
	}
	if got := s.Status(b); got != "failed" {
		t.Fatalf("b tidak boleh berubah, dapat: %s", got)
	}
}

func TestRetryOverallDipAndRecovery(t *testing.T) {
	rec := installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "downloading", Percent: 50})
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "completed", Percent: 100, Message: "selesai"})
		return DownloadResult{Message: "selesai"}, nil
	})

	a, b, c, d := "https://e.example/a", "https://e.example/b", "https://e.example/c", "https://e.example/d"
	s := newFailedSession(t, a, b, c, d)

	// 3 item selesai (100) + c failed (100 karena terminal) => 100%.
	sessCtx := context.WithValue(context.Background(), progressTrackerKey, s.tracker)
	for _, u := range []string{a, b, d} {
		emitProgress(sessCtx, ProgressEvent{URL: u, Status: "completed", Percent: 100, Message: "selesai"})
	}
	emitProgress(sessCtx, ProgressEvent{URL: c, Status: "failed", Message: "gagal"})

	if !rec.containsOverall(100) {
		t.Fatalf("awal harus 100, events: %v", rec.overallValues())
	}

	if _, err := s.Retry(context.Background(), c); err != nil {
		t.Fatalf("retry c gagal: %v", err)
	}

	values := rec.overallValues()
	// Reset c ke 0% => (100+100+0+100)/4 = 75, lalu 50% => 87.5, selesai => 100.
	if !containsFloat(values, 75) || !containsFloat(values, 87.5) || values[len(values)-1] != 100 {
		t.Fatalf("urutan overall retry salah: %v", values)
	}
}

func TestRetryEarlyErrorForcesFailed(t *testing.T) {
	rec := installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		// error di awal DownloadDefault tanpa event terminal apapun
		return DownloadResult{}, errors.New("gagal menentukan nama file output")
	})

	url := "https://example.com/a"
	s := newFailedSession(t, url)

	_, err := s.Retry(context.Background(), url)
	if err == nil {
		t.Fatal("retry harus mengembalikan error dari executeItem")
	}
	if got := s.Status(url); got != "failed" {
		t.Fatalf("status harus failed (dipaksa), dapat: %s", got)
	}
	if !rec.containsOverall(100) {
		t.Fatalf("overall harus 100 setelah dipaksa failed, events: %v", rec.overallValues())
	}

	// Event failed hasil paksaan harus ada (bukan hanya error return).
	var forced bool
	for _, ev := range rec.events {
		if ev.URL == url && ev.Status == "failed" {
			forced = true
		}
	}
	if !forced {
		t.Fatal("harus ada event failed yang dipaksakan")
	}
}

func TestRetryCanceledKeepsCanceled(t *testing.T) {
	installRetrySeams(t, func(ctx context.Context, req DownloadRequest) (DownloadResult, error) {
		emitProgress(ctx, ProgressEvent{URL: req.URL, Status: "canceled", Message: "dibatalkan"})
		return DownloadResult{}, errors.New("dibatalkan")
	})

	url := "https://example.com/a"
	s := newFailedSession(t, url)

	_, err := s.Retry(context.Background(), url)
	if err == nil {
		t.Fatal("retry harus mengembalikan error")
	}
	if got := s.Status(url); got != "canceled" {
		t.Fatalf("status canceled tidak boleh ditimpa, dapat: %s", got)
	}
}

func containsFloat(values []float64, want float64) bool {
	for _, v := range values {
		if v == want {
			return true
		}
	}
	return false
}

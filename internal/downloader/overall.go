package downloader

import (
	"context"
	"fmt"
	"sync"

	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// progressTrackerKeyType membedakan key context tracker dari key lain.
type progressTrackerKeyType struct{}

var progressTrackerKey progressTrackerKeyType

// progressTracker menghitung overall progress sebuah batch dari event progress
// per URL. Item yang berakhir (completed/failed/canceled) dianggap 100% sehingga
// overall selalu bisa mencapai 100% setelah seluruh item selesai; item yang
// masih queued/not started dihitung 0%.
type progressTracker struct {
	mu    sync.Mutex
	state map[string]float64
}

func newProgressTracker() *progressTracker {
	return &progressTracker{state: make(map[string]float64)}
}

// record memperbarui progress sebuah URL sesuai status event-nya.
func (t *progressTracker) record(ev ProgressEvent) {
	t.mu.Lock()
	defer t.mu.Unlock()

	var percent float64
	switch ev.Status {
	case "completed", "failed", "canceled":
		percent = 100
	case "downloading":
		percent = clampPercent(ev.Percent)
	default:
		percent = 0
	}

	t.state[ev.URL] = percent
}

// overall mengembalikan rata-rata progress seluruh item batch.
func (t *progressTracker) overall() float64 {
	t.mu.Lock()
	defer t.mu.Unlock()

	if len(t.state) == 0 {
		return 0
	}

	var sum float64
	for _, p := range t.state {
		sum += p
	}
	return sum / float64(len(t.state))
}

// emitOverall mengirim event progress overall ke frontend.
func (t *progressTracker) emitOverall(ctx context.Context) {
	overall := t.overall()
	runtime.EventsEmit(ctx, progressEventName, ProgressEvent{
		Status:  "overall",
		Percent: overall,
		Message: fmt.Sprintf("Overall: %.1f%%", overall),
	})
}

func clampPercent(p float64) float64 {
	if p < 0 {
		return 0
	}
	if p > 100 {
		return 100
	}
	return p
}

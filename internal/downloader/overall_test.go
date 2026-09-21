package downloader

import "testing"

type overallEvent struct {
	url     string
	status  string
	percent float64
}

func feedOverall(events ...overallEvent) float64 {
	t := newProgressTracker()
	for _, ev := range events {
		t.record(ProgressEvent{URL: ev.url, Status: ev.status, Percent: ev.percent})
	}
	return t.overall()
}

func TestOverallProgress(t *testing.T) {
	almostEqual := func(got, want float64) bool {
		diff := got - want
		if diff < 0 {
			diff = -diff
		}
		return diff < 0.001
	}

	steps := []struct {
		name string
		feed []overallEvent
		want float64
	}{
		{"1 item belum mulai", []overallEvent{{"a", "queued", 0}}, 0},
		{"1 item sedang jalan", []overallEvent{{"a", "downloading", 40}}, 40},
		{"1 item selesai", []overallEvent{{"a", "completed", 0}}, 100},
		{"2 item progress berbeda - awal", []overallEvent{{"a", "downloading", 80}, {"b", "queued", 0}}, 40},
		{"2 item progress berbeda - akhir", []overallEvent{{"a", "downloading", 80}, {"b", "downloading", 40}}, 60},
		{"item failed dihitung selesai", []overallEvent{{"a", "queued", 0}, {"b", "failed", 0}}, 50},
		{"item canceled dihitung selesai", []overallEvent{{"a", "canceled", 0}, {"b", "queued", 0}}, 50},
		{"invalid URL dari TXT", []overallEvent{{"a", "failed", 0}, {"b", "queued", 0}, {"c", "queued", 0}}, 33.333},
		{"semua item invalid", []overallEvent{{"a", "failed", 0}, {"b", "failed", 0}, {"c", "failed", 0}}, 100},
		{"batch selesai -> overall 100", []overallEvent{{"a", "completed", 0}, {"b", "failed", 0}, {"c", "canceled", 0}}, 100},
		{"contoh 4 item -> 62.5", []overallEvent{{"a", "downloading", 100}, {"b", "downloading", 50}, {"c", "queued", 0}, {"d", "completed", 0}}, 62.5},
	}

	for _, st := range steps {
		if got := feedOverall(st.feed...); !almostEqual(got, st.want) {
			t.Errorf("%s: harap %.3f, dapat %.3f", st.name, st.want, got)
		}
	}
}

func TestOverallSingleItemProgress(t *testing.T) {
	tk := newProgressTracker()

	tk.record(ProgressEvent{URL: "a", Status: "queued"})
	if got := tk.overall(); got != 0 {
		t.Fatalf("queued seharusnya 0, dapat %.1f", got)
	}

	tk.record(ProgressEvent{URL: "a", Status: "downloading", Percent: 50})
	if got := tk.overall(); got != 50 {
		t.Fatalf("downloading 50 seharusnya 50, dapat %.1f", got)
	}

	tk.record(ProgressEvent{URL: "a", Status: "completed"})
	if got := tk.overall(); got != 100 {
		t.Fatalf("completed seharusnya 100, dapat %.1f", got)
	}
}

func TestOverallEmptyTracker(t *testing.T) {
	if got := newProgressTracker().overall(); got != 0 {
		t.Errorf("tracker kosong seharusnya 0, dapat %v", got)
	}
}

func TestClampPercent(t *testing.T) {
	if got := clampPercent(-5); got != 0 {
		t.Errorf("clamp -5 seharusnya 0, dapat %v", got)
	}
	if got := clampPercent(150); got != 100 {
		t.Errorf("clamp 150 seharusnya 100, dapat %v", got)
	}
	if got := clampPercent(75.5); got != 75.5 {
		t.Errorf("clamp 75.5 seharusnya 75.5, dapat %v", got)
	}
}

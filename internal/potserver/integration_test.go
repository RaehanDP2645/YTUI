//go:build integration

package potserver

import (
	"os"
	"path/filepath"
	"testing"

	"YTUI/internal/tools"
)

// TestBgutilIntegration menguji proses nyata: ekstrak tools seperti aplikasi,
// mulai Manager (adopsi server eksternal ATAU spawn node sendiri tergantung
// keadaan port 4416), pastikan sehat, lalu Stop dan pastikan lifecycle benar.
func TestBgutilIntegration(t *testing.T) {
	root, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	// go test berjalan di direktori package; repo root adalah dua level di atas.
	root, err = filepath.Abs(filepath.Join(root, "..", ".."))
	if err != nil {
		t.Fatal(err)
	}

	// Meniru aplikasi: ekstrak folder embedded "bin" (di sini dari disk)
	// ke tools directory user.
	if _, err := tools.Extract(os.DirFS(root)); err != nil {
		t.Fatalf("tools.Extract: %v", err)
	}

	externalAtStart := pingHealthy()
	t.Logf("externalAtStart=%v", externalAtStart)

	m := New()
	if err := m.Start(); err != nil {
		t.Fatalf("Start: %v", err)
	}

	m.mu.Lock()
	st := m.status
	external := m.external
	spawned := m.cmd != nil
	m.mu.Unlock()

	t.Logf("sebelum Stop: status=%s external=%v spawned=%v", st, external, spawned)

	if st != StatusReady {
		t.Fatalf("status harus ready, dapat: %s", st)
	}
	if externalAtStart {
		if !external || spawned {
			t.Fatalf("harus mengadopsi server eksternal tanpa spawn: external=%v spawned=%v", external, spawned)
		}
	} else {
		if external || !spawned {
			t.Fatalf("harus spawn server sendiri: external=%v spawned=%v", external, spawned)
		}
	}

	if !pingHealthy() {
		t.Fatal("server tidak sehat setelah Start")
	}

	m.Stop()

	m.mu.Lock()
	st = m.status
	m.mu.Unlock()
	t.Logf("setelah Stop: status=%s", st)

	if st != StatusStopped {
		t.Fatalf("status harus stopped, dapat: %s", st)
	}

	if externalAtStart {
		if !pingHealthy() {
			t.Fatal("server eksternal seharusnya TIDAK dimatikan oleh Stop")
		}
		t.Log("OK: server eksternal tetap berjalan")
	} else {
		if pingHealthy() {
			t.Fatal("server milik kita harus sudah berhenti setelah Stop")
		}
		t.Log("OK: server milik kita berhenti setelah Stop")
	}
}

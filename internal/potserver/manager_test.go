package potserver

import "testing"

func TestManagerNoopLifecycle(t *testing.T) {
	m := New()

	if m.Status() != StatusStopped {
		t.Fatalf("manager baru harus berstatus stopped, dapat: %s", m.Status())
	}

	// Stop pada manager yang belum pernah Start tidak boleh panic dan
	// aman dipanggil berulang kali.
	m.Stop()
	m.Stop()

	if m.Status() != StatusStopped {
		t.Fatalf("setelah Stop harus stopped, dapat: %s", m.Status())
	}
}

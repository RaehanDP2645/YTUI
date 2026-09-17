package downloader

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestReplaceOutputTemplate(t *testing.T) {
	args := []string{"--newline", "-o", "out.%(ext)s", "--merge-output-format", "mp4", "url"}
	out := replaceOutputTemplate(args, `C:\videos\video (1).mp4`)
	if out[2] != `C:\videos\video (1).mp4` {
		t.Fatalf("template tidak diganti: %v", out)
	}
	if out[3] != "--merge-output-format" || out[4] != "mp4" || out[5] != "url" {
		t.Fatalf("arg lain berubah: %v", out)
	}
}

func TestUniqueExistingPath(t *testing.T) {
	dir := t.TempDir()
	p1 := filepath.Join(dir, "video.mp4")
	if err := os.WriteFile(p1, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got := uniqueExistingPath(p1)
	want := filepath.Join(dir, "video (1).mp4")
	if got != want {
		t.Fatalf("harusnya %q, dapat %q", want, got)
	}

	p2 := filepath.Join(dir, "video (1).mp4")
	if err := os.WriteFile(p2, []byte("x"), 0644); err != nil {
		t.Fatal(err)
	}

	got = uniqueExistingPath(p1)
	want = filepath.Join(dir, "video (2).mp4")
	if got != want {
		t.Fatalf("harusnya %q, dapat %q", want, got)
	}

	fresh := filepath.Join(dir, "baru.mp4")
	if got := uniqueExistingPath(fresh); got != fresh {
		t.Fatalf("file belum ada harus tetap sama, dapat %q", got)
	}
}

func TestParseExistingFileChoice(t *testing.T) {
	for _, s := range []string{"rename", "overwrite", "abort", "ask"} {
		if _, ok := ParseExistingFileChoice(s); !ok {
			t.Fatalf("gagal parse %q", s)
		}
	}
	if _, ok := ParseExistingFileChoice("bogus"); ok {
		t.Fatal("bogus harus gagal")
	}
}

func TestMusicProbePathSwap(t *testing.T) {
	path := "C:\\out\\song - 320K.webm"
	idx := strings.Index(path, ".webm")
	got := path[:idx] + ".mp3"
	if got != "C:\\out\\song - 320K.mp3" {
		t.Fatalf("swap music salah: %q", got)
	}
}

package tools

import (
	"os"
	"path/filepath"
	"testing"
	"testing/fstest"
)

// TestExtractRuntimeSubtree memastikan ekstraksi menghasilkan struktur
// runtime yang dibutuhkan aplikasi:
//   - file top-level (yt-dlp.exe, node.exe, dst) selalu diekstrak;
//   - bgutil/server/** diekstrak rekursif;
//   - yt_dlp_plugins/** diekstrak rekursif;
//   - file/sertifikasi non-runtime (bgutil non-server, repository
//     yt-dlp-plugins, .git) TIDAK diekstrak.
func TestExtractRuntimeSubtree(t *testing.T) {
	fsys := fstest.MapFS{
		"bin/yt-dlp.exe":                                     &fstest.MapFile{Data: []byte("exe")},
		"bin/node.exe":                                       &fstest.MapFile{Data: []byte("node")},
		"bin/ffmpeg.exe":                                     &fstest.MapFile{Data: []byte("ffmpeg")},
		"bin/bgutil/server/package.json":                     &fstest.MapFile{Data: []byte("{}")},
		"bin/bgutil/server/build/main.js":                    &fstest.MapFile{Data: []byte("main")},
		"bin/bgutil/server/node_modules/express/index.js":    &fstest.MapFile{Data: []byte("x")},
		"bin/bgutil/nonruntime/ignore.txt":                   &fstest.MapFile{Data: []byte("no")},
		"bin/yt_dlp_plugins/extractor/getpot_bgutil_http.py": &fstest.MapFile{Data: []byte("py")},
		"bin/yt-dlp-plugins/.git/HEAD":                       &fstest.MapFile{Data: []byte("ref")},
	}

	dest := t.TempDir()
	old := cacheDirOverride
	cacheDirOverride = dest
	defer func() { cacheDirOverride = old }()

	got, err := Extract(fsys)
	if err != nil {
		t.Fatalf("Extract: %v", err)
	}

	wantDir := filepath.Join(dest, "YTUI", "tools")
	if got != wantDir {
		t.Fatalf("dir ekstraksi salah, dapat %q want %q", got, wantDir)
	}

	mustExist := []string{
		"yt-dlp.exe",
		"node.exe",
		"bgutil/server/package.json",
		"bgutil/server/build/main.js",
		"bgutil/server/node_modules/express/index.js",
		"yt_dlp_plugins/extractor/getpot_bgutil_http.py",
	}
	for _, p := range mustExist {
		p = filepath.Join(got, filepath.FromSlash(p))
		if _, err := os.Stat(p); err != nil {
			t.Errorf("seharusnya ada: %s (%v)", p, err)
		}
	}

	mustMissing := []string{
		"bgutil/nonruntime/ignore.txt",
		"yt-dlp-plugins/.git/HEAD",
	}
	for _, p := range mustMissing {
		p = filepath.Join(got, filepath.FromSlash(p))
		if _, err := os.Stat(p); err == nil {
			t.Errorf("seharusnya TIDAK ada: %s", p)
		}
	}
}

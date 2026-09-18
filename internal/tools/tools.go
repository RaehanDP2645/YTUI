package tools

import (
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

var (
	mu        sync.RWMutex
	dir       string
	extracted bool
	fail      error
)

// cacheDirOverride untuk test agar ekstraksi bisa diarahkan ke direktori
// sementara tanpa merusak cache user yang asli. Kosong = os.UserCacheDir.
var cacheDirOverride string

const embedPrefix = "bin"

// Extract mengekstrak semua file dari folder embedded "bin" ke
// <UserCacheDir>/YTUI/tools, lalu menyimpan hasilnya untuk dipakai
// oleh Dir(). File yang sudah ada dengan ukuran sama dilewati.
func Extract(embedded fs.FS) (string, error) {
	mu.Lock()
	defer mu.Unlock()

	if extracted {
		return dir, fail
	}

	base := cacheDirOverride
	var err error
	if base == "" {
		base, err = os.UserCacheDir()
		if err != nil {
			fail = fmt.Errorf("cari cache dir gagal: %w", err)
			extracted = true
			return "", fail
		}
	}

	dest := filepath.Join(base, "YTUI", "tools")
	if err := os.MkdirAll(dest, 0755); err != nil {
		fail = fmt.Errorf("buat folder tools gagal: %w", err)
		extracted = true
		return "", fail
	}

	entries, err := fs.ReadDir(embedded, embedPrefix)
	if err != nil {
		fail = fmt.Errorf("baca embedded tools gagal: %w", err)
		extracted = true
		return "", fail
	}

	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		name := entry.Name()
		src := embedPrefix + "/" + name
		dst := filepath.Join(dest, name)

		info, err := fs.Stat(embedded, src)
		if err != nil {
			continue
		}

		if fi, err := os.Stat(dst); err == nil && fi.Size() == info.Size() {
			continue
		}

		if err := copyFile(embedded, src, dst); err != nil {
			fail = fmt.Errorf("ekstrak %s gagal: %w", name, err)
			extracted = true
			return "", fail
		}
	}

	// Subtree runtime yang butuh diekstrak secara rekursif (mis. BGUTIL
	// server dan plugin yt-dlp). Direktori lain (mis. seluruh repository
	// yt-dlp-plugins, .git, berkas development) sengaja tidak diekstrak.
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		name := entry.Name()
		if name != "bgutil" && name != "yt_dlp_plugins" {
			continue
		}
		if err := walkExtract(embedded, embedPrefix+"/"+name, name, dest); err != nil {
			fail = fmt.Errorf("ekstrak subtree %s gagal: %w", name, err)
			extracted = true
			return "", fail
		}
	}

	if len(entries) == 0 {
		fail = errors.New("tidak ada tools dalam folder embedded")
		extracted = true
		return "", fail
	}

	dir = dest
	extracted = true
	return dir, nil
}

// Dir mengembalikan folder tempat tools diekstrak.
// Mengembalikan ("", nil) jika Extract belum pernah dipanggil.
func Dir() (string, error) {
	mu.RLock()
	defer mu.RUnlock()
	return dir, fail
}

func copyFile(fsys fs.FS, src, dst string) error {
	in, err := fsys.Open(src)
	if err != nil {
		return err
	}
	defer in.Close()

	out, err := os.OpenFile(dst, os.O_CREATE|os.O_TRUNC|os.O_WRONLY, 0755)
	if err != nil {
		return err
	}

	if _, err := io.Copy(out, in); err != nil {
		out.Close()
		return err
	}

	if err := out.Close(); err != nil {
		return err
	}

	return os.Chmod(dst, 0755)
}

// shouldExtractRel menentukan apakah satu path (relatif terhadap folder
// embedded "bin") perlu diekstrak secara rekursif. Hanya runtime yang
// dibutuhkan yang diizinkan:
//   - bgutil/server/**        (BGUTIL HTTP server beserta node_modules)
//   - yt_dlp_plugins/**        (plugin POT provider yang dipakai yt-dlp)
//
// Sisanya (mis. seluruh repository yt-dlp-plugins, .git, berkas dev)
// tidak diekstrak.
func shouldExtractRel(rel string) bool {
	if rel == "bgutil/server" || strings.HasPrefix(rel, "bgutil/server/") {
		return true
	}
	if rel == "yt_dlp_plugins" || strings.HasPrefix(rel, "yt_dlp_plugins/") {
		return true
	}
	return false
}

// walkExtract menyalin subtree embedded ke dest secara rekursif,
// mengikuti aturan shouldExtractRel.
func walkExtract(fsys fs.FS, src, rel, dest string) error {
	entries, err := fs.ReadDir(fsys, src)
	if err != nil {
		return err
	}

	for _, entry := range entries {
		childRel := rel + "/" + entry.Name()
		if !shouldExtractRel(childRel) {
			continue
		}

		childSrc := src + "/" + entry.Name()

		if entry.IsDir() {
			if err := walkExtract(fsys, childSrc, childRel, dest); err != nil {
				return err
			}
			continue
		}

		info, err := fs.Stat(fsys, childSrc)
		if err != nil {
			continue
		}

		dst := filepath.Join(dest, filepath.FromSlash(childRel))
		if fi, err := os.Stat(dst); err == nil && fi.Size() == info.Size() {
			continue
		}

		if err := os.MkdirAll(filepath.Dir(dst), 0755); err != nil {
			return err
		}

		if err := copyFile(fsys, childSrc, dst); err != nil {
			return err
		}
	}

	return nil
}

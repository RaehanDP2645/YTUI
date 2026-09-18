package ytdlp

import (
	"strings"
	"testing"
)

func TestBuildArgsUsesQuickJS(t *testing.T) {
	args, err := BuildArgs(Options{
		URL:         "https://example.com/video",
		Kind:        KindVideo,
		Mode:        ModeDefault,
		Quality:     "720p",
		OutputDir:   "C:\\out",
		FFmpegPath:  "C:\\tools\\ffmpeg.exe",
		QuickJSPath: "C:\\tools\\qjs.exe",
	})
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	for i, a := range args {
		if a == "--js-runtimes" {
			if i+1 >= len(args) || args[i+1] != "quickjs:C:\\tools\\qjs.exe" {
				t.Fatalf("flag --js-runtimes salah: %v", safeAt(args, i+1))
			}
			return
		}
	}
	t.Fatalf("--js-runtimes tidak ditemukan di args: %v", args)
}

func TestBuildArgsWithoutQuickJS(t *testing.T) {
	args, err := BuildArgs(Options{
		URL:        "https://example.com/video",
		Kind:       KindVideo,
		Mode:       ModeDefault,
		Quality:    "720p",
		OutputDir:  "C:\\out",
		FFmpegPath: "C:\\tools\\ffmpeg.exe",
	})
	if err != nil {
		t.Fatalf("BuildArgs: %v", err)
	}

	for _, a := range args {
		if a == "--js-runtimes" || strings.Contains(a, "deno") {
			t.Fatalf("args tidak boleh memuat js-runtimes/deno: %v", args)
		}
	}
}

func safeAt(args []string, i int) string {
	if i < 0 || i >= len(args) {
		return "<kosong>"
	}
	return args[i]
}

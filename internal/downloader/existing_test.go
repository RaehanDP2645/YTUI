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

func TestWindowsSanitizeFilename(t *testing.T) {
	// Harapan mengikuti sanitize_filename yt-dlp (restricted=false, is_id=NO_DEFAULT).
	cases := []struct {
		in   string
		want string
	}{
		{"", ""},
		{"Me at the zoo", "Me at the zoo"},
		{"可惜夜", "可惜夜"},
		{"A: B", "A： B"},                     // colon   -> full-width U+FF1A
		{"x?y", "x？y"},                       // ?       -> full-width U+FF1F
		{`x"y`, "x＂y"},                       // "       -> full-width U+FF02
		{"A*B", "A＊B"},                       // *       -> full-width U+FF0A
		{"a<b>c", "a＜b＞c"},                   // < >     -> full-width U+FF1C / U+FF1E
		{"a|b", "a｜b"},                       // |       -> full-width U+FF5C
		{"a/b", "a⧸b"},                       // /       -> U+29F8
		{`a\b`, "a⧹b"},                       // \       -> U+29F9
		{"x\ny", "x y"},                      // newline -> spasi
		{"x\n\n\ny", "x y"},                  // banyak newline -> satu spasi
		{"\nx", "x"},                         // newline awal dibuang
		{"x\n", "x"},                         // newline akhir dibuang
		{"\n", "_"},                          // jadi kosong -> "_"
		{"12:30:00 title", "12_30_00 title"}, // timestamp kolon -> underscore
		{"a__b", "a__b"},                     // is_id=NO_DEFAULT: underscore ganda TIDAK dipangkas
		{"-abc", "-abc"},                     // is_id=NO_DEFAULT: dash awal TIDAK diproses
		{" a ", " a "},                       // spasi luar dipertahankan
	}

	for _, c := range cases {
		if got := windowsSanitizeFilename(c.in); got != c.want {
			t.Errorf("windowsSanitizeFilename(%q) = %q, harap %q", c.in, got, c.want)
		}
	}
}

func TestTitleEmptyInPath(t *testing.T) {
	cases := []struct {
		path string
		want bool
	}{
		{`C:\videos\ - 1080p.mp4`, true},
		{`C:\videos\ - 320K.mp3`, true},
		{`C:\videos\Me at the zoo - 1080p.mp4`, false},
		{`C:\videos\可惜夜 - 1080p.mp4`, false},
		{`C:\videos\ - x.mp4`, true},
	}
	for _, c := range cases {
		if got := titleEmptyInPath(c.path); got != c.want {
			t.Errorf("titleEmptyInPath(%q) = %v, harap %v", c.path, got, c.want)
		}
	}
}

func TestPathReconstruction(t *testing.T) {
	dir := `C:\videos`
	base := ` - 1080p.mp4`
	got := okReconstruct(dir, base, "可惜夜")
	want := `C:\videos\可惜夜 - 1080p.mp4`
	if got != want {
		t.Fatalf("rekonstruksi path = %q, harap %q", got, want)
	}
}

func okReconstruct(dir, base, title string) string {
	return dir + `\` + windowsSanitizeFilename(title) + base
}

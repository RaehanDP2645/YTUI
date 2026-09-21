package downloader

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestValidateYoutubeURL(t *testing.T) {
	valid := []string{
		"https://www.youtube.com/watch?v=VALID1",
		"https://youtu.be/VALID2",
		"https://music.youtube.com/watch?v=VALID3",
		"https://m.youtube.com/watch?v=VALID4",
		"http://youtube.com/watch?v=VALID5",
	}

	for _, url := range valid {
		if err := validateYoutubeURL(url); err != nil {
			t.Errorf("URL harus valid: %q - err: %v", url, err)
		}
	}

	invalid := []string{
		"https://example.com/test",
		"ini-bukan-url",
		"ftp://example.com/test",
		"https://notyoutube.com/watch?v=x",
		"youtube.com/watch?v=x",
		"https:///path-only",
		"",
	}

	for _, url := range invalid {
		if err := validateYoutubeURL(url); err == nil {
			t.Errorf("URL harus invalid: %q", url)
		}
	}
}

func TestReadURLsFromFile(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "batch.txt")

	content := strings.Join([]string{
		"https://www.youtube.com/watch?v=VALID1",
		"",
		"   https://youtu.be/VALID2  ",
		"# komentar",
		"  # komentar dengan spasi",
		"https://example.com/test",
	}, "\n")

	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}

	urls, err := readURLsFromFile(path)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"https://www.youtube.com/watch?v=VALID1",
		"https://youtu.be/VALID2",
		"https://example.com/test",
	}

	if !reflect.DeepEqual(urls, want) {
		t.Fatalf("harusnya %v, dapat %v", want, urls)
	}
}

package downloader

import (
	"fmt"
	"net/url"
	"strings"
)

// youtubeHosts adalah daftar host yang dikenali sebagai milik YouTube.
var youtubeHosts = map[string]struct{}{
	"youtube.com":       {},
	"www.youtube.com":   {},
	"m.youtube.com":     {},
	"music.youtube.com": {},
	"youtu.be":          {},
}

// validateYoutubeURL memastikan sebuah baris adalah URL YouTube yang valid
// secara syntactic/domain. Tidak melakukan network request atau resolusi
// lebih lanjut; cukup untuk memisahkan baris yang layak dikirim ke yt-dlp
// dari baris yang jelas-jelas bukan URL YouTube.
func validateYoutubeURL(raw string) error {
	if strings.TrimSpace(raw) == "" {
		return fmt.Errorf("URL kosong")
	}

	u, err := url.Parse(raw)
	if err != nil {
		return fmt.Errorf("URL tidak valid: %v", err)
	}

	if u.Scheme != "http" && u.Scheme != "https" {
		return fmt.Errorf("URL wajib memakai scheme http/https, dapat: %q", u.Scheme)
	}

	if u.Host == "" {
		return fmt.Errorf("URL tidak memiliki host")
	}

	hostname := strings.ToLower(u.Hostname())
	if _, ok := youtubeHosts[hostname]; !ok {
		return fmt.Errorf("domain bukan YouTube (youtube.com/youtu.be), dapat: %s", hostname)
	}

	return nil
}

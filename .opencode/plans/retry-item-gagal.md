# Rencana: Retry Item Gagal (Queue & Batch) — YTD-UI

Status: menunggu persetujuan eksekusi (harness masih plan mode/read-only).
Fitur tunggal: retry SATU item yang berstatus `failed`. Batch cancel TIDAK diimplementasikan.

## Konfirmasi desain (disetujui user)
- Seam `var executeItem = DownloadDefault` dipakai HANYA oleh Retry; worker batch & DownloadDefault tidak diubah. Untuk test sukses deterministik.
- E2E deterministik memakai ID video fiktif (`?v=NOTREAL...` — selalu gagal: "This video is unavailable" atau gagal jaringan). Jalur "retry sukses" di-cover unit test seam.

## Analisis singkat
- `DownloadBatch` saat ini single-shot: baca TXT -> validasi -> worker pool -> hasil; tanpa state bertahan.
- `progressTracker` (overall.go) via context key; `emitProgress` (service.go:418) satu pintu event.
- Identity item = URL (sudah dipakai frontend `downloadItems`, tracker `state[url]`).
- Gap: ada jalur error `DownloadDefault` yang tidak mengirim event terminal (mis. "Gagal menentukan nama file output") -> retry wajib emit `failed` sendiri.

## Perubahan Backend

### 1. `internal/downloader/session.go` (BARU) — `BatchSession`
- `BatchItem{URL, Status, Percent, Message, Type, Quality, OutputDir}`; status vocabulary sama dgn event existing.
- `BatchSession{mu, base context (app ctx, tidak di-cancel), tracker *progressTracker, items map[string]*BatchItem, retrying map[string]bool}`.
- `batchSessionKey` context key; `applyEvent(ev)` sinkron dari aliran event (URL key).
- `Status(url) string` (exported, untuk E2E hook).
- `Retry(ctx, url) (DownloadResult, error)` (exported):
  1. guard atomik (satu lock): item ada && `Status=="failed"` && tidak di map retrying; selain itu error `errItemUnknown / errRetryNotFailed / errRetryInProgress`.
  2. tandai retrying, reset item -> queued/0%/msg kosong; req dibangun dari item (Type/Quality/OutputDir asli item batch).
  3. `retryCtx = context.WithValue(ctx, progressTrackerKey, s.tracker)` + `batchSessionKey`; emit `queued` (item 0%, overall turun sementara).
  4. `result, err := executeItem(retryCtx, req)` -> DownloadDefault (default), event retry otomatis update overall+state via emitProgress.
  5. selesai: hapus retrying; jika `err != nil` dan belum ada status terminal (completed/failed/canceled) -> emit `failed` (memastikan terminal + overall kembali 100; retry tetap tersedia). Konteks batch lama TIDAK dipakai.
- Seam: `var executeItem = DownloadDefault` (test-only swap).

### 2. `internal/downloader/overall.go`
- Tambah hook emit saja: `emitOverall` memakai `emitEvent` (var seam) agar jalur emit bisa di-stub di unit test. Logika `record/overall` TIDAK berubah (retry reset dilakukan lewat event `queued` -> `record` default 0; tidak perlu method baru).

### 3. `internal/downloader/service.go` — `emitProgress` (baris 418)
- Tambah var seam `var emitEvent = runtime.EventsEmit`; ganti pemanggilan `runtime.EventsEmit` di `emitProgress` (dan `emitOverall`) menjadi `emitEvent`.
- Tambah cabang: bila session ada di ctx -> `session.applyEvent(event)`.
- Jalur existing (tracker record + emitOverall) tidak diubah.

### 4. `internal/downloader/batch.go`
- Ganti `newProgressTracker()` -> `session := newBatchSession(ctx, req, urls)`; attach `batchSessionKey` + `progressTrackerKey (session.tracker)` ke batchCtx SEBELUM loop validasi (agar event queued/failed ikut update session).
- Worker & tracker existing TIDAK diubah. Hasil akhir: `BatchDownloadResult{..., Session: session}`.

### 5. `internal/downloader/request.go`
- `BatchDownloadResult` + `Session *BatchSession` tag `json:"-"` (tidak sampai JS).

### 6. `internal/downloader/app.go` (App binding)
- Field `batchSession *downloader.BatchSession` + mu kecil (pola `fileExistsMu`).
- `DownloadBatch` binding: simpan `a.batchSession = res.Session`.
- Binding baru `RetryDownload(url string) (DownloadResult, error)`:
  - session nil -> error "Tidak ada batch aktif".
  - `retryCtx, cancel := context.WithCancel(a.ctx)`; simpan cancel ke slot `a.cancel` yang ADA (CancelDownload existing ikut membatalkan retry; tanpa mengubah CancelDownload); defer clear.
  - `return a.batchSession.Retry(retryCtx, url)`.

## Perubahan Frontend (`frontend/src/main.js`, `style.css`)
- Import `RetryDownload` dari wailsjs (digenapkan `wails generate module`).
- Variabel `let retryActive = false; const retryingUrls = new Set();`; reset saat batch/single baru.
- Guard `updateProgress`: `if ((batchActive || retryActive) && event.status !== 'overall') return;` (generic; item event tidak menyentuh main bar saat retry, overall yang menggerakkan main bar).
- `updateDownloadItemDom`: tambah elemen `.download-item-actions > button.retry-button`; tampil hanya `status === 'failed'`, disabled saat sedang retry; listener sekali saat item dibuat -> `retryItem(url)`.
- `retryItem(url)`: guard `retryingUrls.has(url)` + item.status !== 'failed'; set retrying/retryActive; matikan batchDownloadBtn/downloadBtn; `await RetryDownload(url)`; `setStatus` dari result/catch; finally reset + aktifkan tombol. Status card diperbarui otomatis lewat aliran event (URL = key).
- Guard `batchDownloadBtn` handler: `if (retryActive) return action`.
- `style.css`: 1 rule kecil `.retry-button` (reuse pola tombol existing; `button:disabled` existing sudah ada).

## Duplicate retry prevention (2 lapis)
1. Backend: guard atomik — status harus `failed`; set `queued` + tandai `retrying[url]` dalam satu lock. Item queued/downloading otomatis tertolak.
2. Frontend: `retryingUrls` + tombol disabled.

## Overall progress & retry
- Tidak mengubah logika tracker. Retry: event `queued` -> item 0% (overall turun) -> `downloading` mengikuti percent -> sukses `completed` 100% / gagal lagi `failed` 100% (item selesai), tombol retry tetap ada. Cancel: `canceled` 100%.
- Contoh 4 item (2 succ, 1 fail, 1 succ): 100% -> reset -> 75% -> download 50% -> 87.5% -> selesai -> 100%.

## Test (deterministik, tanpa ketergantungan YouTube)
- `internal/downloader/retry_test.go` (BARU): seam `executeItem` + seam `emitEvent` (stub recorder). Fake script misal url -> fail, fail, ok (emit via emitProgress). Kasus:
  1. hanya item failed bisa retry (completed -> tolak; tanpa ubah state).
  2. item tak dikenal -> tolak.
  3. retry sukses -> completed, overall 100.
  4. retry gagal lagi -> failed; retry berikutnya tetap boleh (dianggak "tombol tersedia").
  5. duplicate/rapid click: executeItem di-block channel; 2 call konkuren -> kedua tolak; executeItem dipanggil 1×.
  6. hanya item tsb diulang: item lain status & call-count tidak berubah.
  7. overall: event pipeline queued/reset -> 75 -> downloading 87.5 -> completed 100; dan retry gagal -> kembali 100.
  8. existing test (overall/validate/ytdlp...) tidak diubah, harus tetap hijau.
- E2E env hook `YTUI_TEST_RETRY` (sementara di app.go startup, direvert setelah): batch 2 video fiktif -> total=2 failed=2 -> `session.Retry(url1)` x2 (fail lagi, status tetap failed, retry tersedia) -> cek overall (100 -> turun saat reset -> 100) via logger; `session.Status` dicek. Tidak memakai YouTube asli.

## Verifikasi build
- gofmt (hanya file diubah; jangan sentuh hide_window_*.go / progress.go yang sudah pre-existing unformatted), `go build ./...`, `go vet ./...`, `go test ./...`
- `wails generate module` (sinkronkan wailsjs) -> `npm run build` -> `wails build`
- E2E: build exe + hook -> run env -> baca log -> revert hook + log (`git checkout -- app.go logs/...`) -> build final.

## Batasan yang dipatuhi
- Tidak menyentuh BGUTIL/Node/QuickJS/FFmpeg, command/build-args yt-dlp, single URL download, import TXT, validasi URL.
- Tidak refactor besar; API existing (`DownloadBatch`, `DownloadDefault`, `CancelDownload`, file-exists) tidak berubah.
package downloader

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"sync"

	"YTUI/internal/logger"
)

type batchJob struct {
	URL string
}

type batchJobResult struct {
	URL string
	Err error
}

func DownloadBatch(ctx context.Context, req BatchDownloadRequest) (BatchDownloadResult, error) {
	urls, err := readURLsFromFile(req.FilePath)
	if err != nil {
		return BatchDownloadResult{}, err
	}

	if len(urls) == 0 {
		return BatchDownloadResult{}, errors.New("file batch tidak berisi URL")
	}

	parallel := req.Parallel
	if parallel <= 0 {
		parallel = 2
	}

	if parallel > 5 {
		parallel = 5
	}

	logger.L.Runtime("Batch download dimulai: total=%d, parallel=%d", len(urls), parallel)

	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	// BatchSession menyimpan state batch yang bertahan setelah fungsi ini selesai,
	// sehingga item gagal bisa di-retry satu per satu. Seluruh event progress item
	// (dari validasi, worker, maupun batcher) disinkronkan ke session via emitProgress.
	session := newBatchSession(ctx, req, urls)
	batchCtx = context.WithValue(batchCtx, batchSessionKey, session)

	// progressTracker menempel di batchCtx sehingga seluruh event progress item
	// (dari validasi, worker, maupun batcher) tercatat dan overall dipancarkan.
	// Tracker milik session dipakai supaya retry berbagi state yang sama.
	batchCtx = context.WithValue(batchCtx, progressTrackerKey, session.tracker)

	// Validasi URL: hanya baris yang valid masuk queue dan diproses worker.
	// Baris invalid ditandai failed per-item tanpa dikirim ke yt-dlp dan
	// tidak menghentikan batch URL lainnya.
	validURLs := make([]string, 0, len(urls))
	invalidCount := 0

	for _, url := range urls {
		if err := validateYoutubeURL(url); err != nil {
			invalidCount++
			emitProgress(batchCtx, ProgressEvent{
				URL:     url,
				Status:  "failed",
				Message: err.Error(),
			})
			logger.L.Error("Batch item URL tidak valid: %s - err: %v", url, err)
			continue
		}

		validURLs = append(validURLs, url)
		emitProgress(batchCtx, ProgressEvent{
			URL:     url,
			Status:  "queued",
			Message: "Queued",
		})
	}

	if invalidCount > 0 {
		logger.L.Runtime("Batch: %d URL valid (queued), %d URL tidak valid", len(validURLs), invalidCount)
	}

	jobs := make(chan batchJob)
	results := make(chan batchJobResult)

	var wg sync.WaitGroup
	for workerID := 0; workerID < parallel; workerID++ {
		wg.Add(1)

		go func() {
			defer wg.Done()

			for job := range jobs {
				err := batchCtx.Err()
				if err == nil {
					_, err = DownloadDefault(batchCtx, DownloadRequest{
						URL:       job.URL,
						Type:      req.Type,
						Quality:   req.Quality,
						OutputDir: req.OutputDir,
					})
				}

				results <- batchJobResult{
					URL: job.URL,
					Err: err,
				}

				if err != nil && !req.SkipErrors {
					cancel()
				}
			}
		}()
	}

	go func() {
		defer close(jobs)

		for _, url := range validURLs {
			if batchCtx.Err() != nil {
				return
			}

			jobs <- batchJob{URL: url}
		}
	}()

	go func() {
		wg.Wait()
		close(results)
	}()

	completed := 0
	failed := invalidCount

	for result := range results {
		if result.Err != nil {
			failed++
			// Jalur error yang tidak sempat mengirim event terminal (mis. gagal
			// menentukan nama file output di awal DownloadDefault) dipastikan
			// tetap tercatat selesai/gagal sehingga overall batch bisa mencapai 100%.
			emitProgress(batchCtx, ProgressEvent{
				URL:     result.URL,
				Status:  "failed",
				Message: result.Err.Error(),
			})
			logger.L.Error("Batch item gagal: %s - err:%v", result.URL, result.Err)
			continue
		}

		completed++
		logger.L.Runtime("Batch item selesai: %s", result.URL)
	}

	// Pastikan event overall final terkirim (seluruh item sudah berakhir).
	if tracker, ok := batchCtx.Value(progressTrackerKey).(*progressTracker); ok {
		tracker.emitOverall(batchCtx)
	}

	logger.L.Runtime("Batch selesai: total=%d, sukses=%d, gagal=%d", len(urls), completed, failed)
	return BatchDownloadResult{
		Message:   "Batch download selesai",
		Total:     len(urls),
		Completed: completed,
		Failed:    failed,
		OutputDir: req.OutputDir,
		Session:   session,
	}, nil
}

func readURLsFromFile(filePath string) ([]string, error) {
	filePath = strings.TrimSpace(filePath)
	if filePath == "" {
		return nil, errors.New("file path belum dipilih")
	}

	file, err := os.Open(filePath)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var urls []string
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		urls = append(urls, line)
	}

	if err := scanner.Err(); err != nil {
		return nil, err
	}

	return urls, nil
}

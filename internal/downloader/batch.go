package downloader

import (
	"bufio"
	"context"
	"errors"
	"os"
	"strings"
	"sync"
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

	batchCtx, cancel := context.WithCancel(ctx)
	defer cancel()

	jobs := make(chan batchJob)
	results := make(chan batchJobResult)

	for _, url := range urls {
		emitProgress(batchCtx, ProgressEvent{
			URL:     url,
			Status:  "queued",
			Message: "Queued",
		})
	}

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

		for _, url := range urls {
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
	failed := 0

	for result := range results {
		if result.Err != nil {
			failed++
			continue
		}

		completed++
	}

	return BatchDownloadResult{
		Message:   "Batch download selesai",
		Total:     len(urls),
		Completed: completed,
		Failed:    failed,
		OutputDir: req.OutputDir,
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

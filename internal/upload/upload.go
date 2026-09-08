// Package upload transfers file parts to presigned storage URLs.
package upload

import (
	"bytes"
	"context"
	"crypto/md5"
	"encoding/base64"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dolthub/cli/internal/dolthub"
)

const PartSize int64 = 5 << 20
const MaxFileSize int64 = 1 << 30

// NumParts is the number of fixed-size parts needed for a supported table file.
func NumParts(size int64) (int, error) {
	if size <= 0 {
		return 0, errors.New("file must not be empty")
	}
	if size > MaxFileSize {
		return 0, errors.New("table import files must not exceed 1 GiB")
	}
	return int((size + PartSize - 1) / PartSize), nil
}

type Result struct {
	Parts []dolthub.CompletedPart
	MD5   string
}

// File uploads with four workers and at most 20 MiB of part buffers. Progress
// reports successfully transferred bytes and is called serially.
func File(ctx context.Context, file io.ReaderAt, size int64, session dolthub.ImportUpload, progress func(int64)) (Result, error) {
	count, err := NumParts(size)
	if err != nil {
		return Result{}, err
	}
	if session.Token == "" || session.ContentsKey == "" || session.HTTPMethod != http.MethodPut || len(session.Parts) != count {
		return Result{}, errors.New("invalid upload session")
	}
	parts := append([]dolthub.ImportUploadPart(nil), session.Parts...)
	sort.Slice(parts, func(i, j int) bool { return parts[i].PartNumber < parts[j].PartNumber })
	for i, part := range parts {
		u, e := url.Parse(part.URL)
		if part.PartNumber != i+1 || e != nil || u.Scheme != "https" || u.Host == "" || u.User != nil || u.Fragment != "" {
			return Result{}, errors.New("invalid upload part URL or numbering")
		}
	}
	// This client deliberately has no OAuth transport, cookies, or redirects.
	client := &http.Client{Timeout: 2 * time.Minute, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	result := Result{Parts: make([]dolthub.CompletedPart, count)}
	hashes := make([]string, count)
	var mu sync.Mutex
	var firstErr error
	var transferred int64
	var wg sync.WaitGroup
	for worker := 0; worker < 4; worker++ {
		wg.Add(1)
		go func(start int) {
			defer wg.Done()
			for i := start; i < count; i += 4 {
				if ctx.Err() != nil {
					return
				}
				offset := int64(i) * PartSize
				length := min(PartSize, size-offset)
				data := make([]byte, int(length))
				_, e := io.ReadFull(io.NewSectionReader(file, offset, length), data)
				var etag string
				if e == nil {
					etag, e = transfer(ctx, client, parts[i].URL, session.Headers, data)
				}
				mu.Lock()
				if e != nil {
					if firstErr == nil {
						firstErr = fmt.Errorf("upload part %d: %w", i+1, e)
						cancel()
					}
					mu.Unlock()
					return
				}
				sum := md5.Sum(data)
				hashes[i] = base64.StdEncoding.EncodeToString(sum[:])
				result.Parts[i] = dolthub.CompletedPart{PartNumber: i + 1, ETag: etag}
				transferred += length
				if progress != nil {
					progress(transferred)
				}
				mu.Unlock()
			}
		}(worker)
	}
	wg.Wait()
	if firstErr != nil {
		return Result{}, firstErr
	}
	if err := ctx.Err(); err != nil {
		return Result{}, err
	}
	// Match DoltHub's web uploader: hash concatenated base64 part digests.
	sum := md5.Sum([]byte(strings.Join(hashes, "")))
	result.MD5 = base64.StdEncoding.EncodeToString(sum[:])
	return result, nil
}

func transfer(ctx context.Context, client *http.Client, target string, headers map[string][]string, data []byte) (string, error) {
	for attempt := 0; attempt < 4; attempt++ {
		if attempt > 0 {
			timer := time.NewTimer(time.Duration(1<<(attempt-1)) * time.Second)
			select {
			case <-ctx.Done():
				timer.Stop()
				return "", ctx.Err()
			case <-timer.C:
			}
		}
		req, err := http.NewRequestWithContext(ctx, http.MethodPut, target, bytes.NewReader(data))
		if err != nil {
			return "", errors.New("invalid upload request")
		}
		for key, values := range headers {
			for _, value := range values {
				req.Header.Add(key, value)
			}
		}
		resp, err := client.Do(req)
		if err != nil {
			if ctx.Err() != nil {
				return "", ctx.Err()
			}
			// net/http errors contain the signed URL; do not return them to callers.
			if attempt == 3 {
				return "", errors.New("transfer failed after 4 attempts; rerun the command")
			}
			continue
		}
		_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
		_ = resp.Body.Close()
		if resp.StatusCode >= 200 && resp.StatusCode < 300 {
			etag := resp.Header.Get("ETag")
			if strings.TrimSpace(etag) == "" {
				return "", errors.New("storage response is missing an ETag")
			}
			return etag, nil
		}
		if resp.StatusCode == 400 || resp.StatusCode == 403 {
			return "", fmt.Errorf("storage returned HTTP %d; the upload URL may have expired (10-minute lifetime); rerun the command", resp.StatusCode)
		}
		if attempt == 3 || (resp.StatusCode != 408 && resp.StatusCode != 429 && resp.StatusCode < 500) {
			return "", fmt.Errorf("storage returned HTTP %d", resp.StatusCode)
		}
	}
	return "", errors.New("transfer failed")
}

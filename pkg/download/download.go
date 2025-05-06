package download

import (
	"context"
	"crypto/sha256"
	"crypto/tls"
	"encoding/hex"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

type Manager struct {
	downloadDir string
}

func NewManager(downloadDir string) *Manager {
	// Ensure download directory exists
	if _, err := os.Stat(downloadDir); os.IsNotExist(err) {
		log.Printf("Download directory %s does not exist, creating it...", downloadDir)
		if err := os.MkdirAll(downloadDir, 0755); err != nil {
			log.Printf("Error creating download directory: %v", err)
		}
	}
	
	return &Manager{
		downloadDir: downloadDir,
	}
}

func (m *Manager) Download(ctx context.Context, url string) (string, error) {
	filename := filepath.Base(url)
	if filename == "" || filename == "." {
		filename = "update.mender"
	}

	downloadPath := filepath.Join(m.downloadDir, filename)

	fileInfo, err := os.Stat(downloadPath)
	var fileSize int64
	if err == nil {
		fileSize = fileInfo.Size()
		log.Printf("File already exists with size %d bytes, resuming download", fileSize)
	} else if !os.IsNotExist(err) {
		return "", fmt.Errorf("error checking file: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("error creating request: %w", err)
	}

	if fileSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", fileSize))
	}

	// Create a custom transport with separate timeouts
	transport := &http.Transport{
		TLSClientConfig: &tls.Config{
			VerifyConnection: func(cs tls.ConnectionState) error {
				// Skip certificate time validation
				return nil
			},
		},
		// Timeout for establishing TCP connections
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		// Timeout for TLS handshake
		TLSHandshakeTimeout: 30 * time.Second,
		// Increase idle connections
		MaxIdleConns:        100,
		IdleConnTimeout:     90 * time.Second,
	}

	client := &http.Client{
		Transport: transport,
		// No timeout here - we'll handle timeouts through context
		Timeout: 0,
	}

	var resp *http.Response
	maxRetries := 5
	for i := 0; i < maxRetries; i++ {
		log.Printf("Starting download attempt %d/%d", i+1, maxRetries)
		resp, err = client.Do(req)
		if err == nil {
			break
		}
		log.Printf("Error downloading file (attempt %d/%d): %v", i+1, maxRetries, err)
		if i < maxRetries-1 {
			sleepTime := time.Duration(1<<uint(i)) * time.Second
			log.Printf("Waiting %v before retry...", sleepTime)
			time.Sleep(sleepTime)
		}
	}
	if err != nil {
		return "", fmt.Errorf("error downloading file after %d attempts: %w", maxRetries, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusPartialContent {
		return "", fmt.Errorf("unexpected status code: %d", resp.StatusCode)
	}

	var file *os.File
	if fileSize > 0 && resp.StatusCode == http.StatusPartialContent {
		file, err = os.OpenFile(downloadPath, os.O_APPEND|os.O_WRONLY, 0644)
		log.Printf("Opened file for append at offset %d", fileSize)
	} else {
		file, err = os.Create(downloadPath)
		fileSize = 0
		log.Printf("Created new file for download")
	}
	if err != nil {
		return "", fmt.Errorf("error opening file: %w", err)
	}
	defer file.Close()

	// Increase buffer size to 1MB for faster downloads
	buffer := make([]byte, 1024*1024)
	totalRead := fileSize
	lastProgressReport := time.Now()
	start := time.Now()
	
	for {
		select {
		case <-ctx.Done():
			return "", ctx.Err()
		default:
			n, err := resp.Body.Read(buffer)
			if n > 0 {
				_, writeErr := file.Write(buffer[:n])
				if writeErr != nil {
					return "", fmt.Errorf("error writing to file: %w", writeErr)
				}
				totalRead += int64(n)

				if time.Since(lastProgressReport) > 5*time.Second {
					elapsed := time.Since(start)
					speed := float64(totalRead) / elapsed.Seconds() / 1024 / 1024 // MB/s
					log.Printf("Downloaded %d bytes (%.2f MB/s)", totalRead, speed)
					lastProgressReport = time.Now()
				}
			}
			if err != nil {
				if err == io.EOF {
					elapsed := time.Since(start)
					speed := float64(totalRead) / elapsed.Seconds() / 1024 / 1024 // MB/s
					log.Printf("Download complete, total size: %d bytes, average speed: %.2f MB/s", totalRead, speed)
					return downloadPath, nil
				}
				return "", fmt.Errorf("error reading response: %w", err)
			}
		}
	}
}

func (m *Manager) VerifyChecksum(filePath, checksumStr string) error {
	parts := strings.SplitN(checksumStr, ":", 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid checksum format, expected 'algorithm:hash', got '%s'", checksumStr)
	}

	algorithm := strings.ToLower(parts[0])
	expectedHash := parts[1]

	if algorithm != "sha256" {
		return fmt.Errorf("unsupported checksum algorithm: %s", algorithm)
	}

	file, err := os.Open(filePath)
	if err != nil {
		return fmt.Errorf("error opening file for checksum verification: %w", err)
	}
	defer file.Close()

	hash := sha256.New()
	if _, err := io.Copy(hash, file); err != nil {
		return fmt.Errorf("error calculating checksum: %w", err)
	}

	actualHash := hex.EncodeToString(hash.Sum(nil))
	if actualHash != expectedHash {
		return fmt.Errorf("checksum mismatch: expected %s, got %s", expectedHash, actualHash)
	}

	return nil
}

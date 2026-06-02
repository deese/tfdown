package main

import (
	"archive/zip"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"time"
)

// hashicorpCheckResponse holds the fields we care about from the checkpoint API.
type hashicorpCheckResponse struct {
	Product        string `json:"product"`
	CurrentVersion string `json:"current_version"`
}

// Downloader manages downloading HashiCorp tools.
type Downloader struct {
	targetOS   string
	targetArch string
	versions   map[string]string // tool name -> requested version (empty = latest)
	quiet      bool
	httpClient *http.Client
}

// NewDownloader creates a Downloader.
// versions maps tool names to pinned versions; missing or empty entries resolve to latest.
func NewDownloader(targetOS, targetArch string, versions map[string]string, quiet bool) *Downloader {
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}
	if versions == nil {
		versions = make(map[string]string)
	}

	client := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				for _, env := range []string{"https_proxy", "HTTPS_PROXY", "http_proxy", "HTTP_PROXY"} {
					if proxyURL := os.Getenv(env); proxyURL != "" {
						return url.Parse(proxyURL)
					}
				}
				return nil, nil
			},
		},
	}

	return &Downloader{
		targetOS:   targetOS,
		targetArch: targetArch,
		versions:   versions,
		quiet:      quiet,
		httpClient: client,
	}
}

// GetLatestVersion queries the checkpoint API and returns the latest version of toolName.
func (d *Downloader) GetLatestVersion(toolName string) (string, error) {
	checkURL := fmt.Sprintf("https://checkpoint-api.hashicorp.com/v1/check/%s", toolName)
	resp, err := d.httpClient.Get(checkURL)
	if err != nil {
		return "", fmt.Errorf("fetching latest version of %s: %w", toolName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("fetching latest version of %s: HTTP %d", toolName, resp.StatusCode)
	}

	var check hashicorpCheckResponse
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		return "", fmt.Errorf("decoding version response for %s: %w", toolName, err)
	}

	if check.CurrentVersion == "" {
		return "", fmt.Errorf("empty version in response for %s", toolName)
	}

	return check.CurrentVersion, nil
}

// GetVersion returns the version to download for toolName:
// the user-pinned version if set, otherwise the latest available.
func (d *Downloader) GetVersion(toolName string) (string, error) {
	if ver := d.versions[toolName]; ver != "" {
		return strings.TrimPrefix(ver, "v"), nil
	}
	return d.GetLatestVersion(toolName)
}

// Download downloads toolName at the configured (or latest) version.
// Returns the path of the downloaded .zip file.
func (d *Downloader) Download(toolName string) (string, error) {
	ver, err := d.GetVersion(toolName)
	if err != nil {
		return "", err
	}
	ver = strings.TrimPrefix(ver, "v")

	downloadURL := fmt.Sprintf(
		"https://releases.hashicorp.com/%s/%s/%s_%s_%s_%s.zip",
		toolName, ver, toolName, ver, d.targetOS, d.targetArch,
	)
	zipFile := fmt.Sprintf("%s_%s_%s_%s.zip", toolName, ver, d.targetOS, d.targetArch)

	fmt.Printf("Downloading %s %s for %s/%s...\n", toolName, ver, d.targetOS, d.targetArch)
	if !d.quiet {
		fmt.Printf("URL: %s\n", downloadURL)
	}

	resp, err := d.httpClient.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("downloading %s: %w", toolName, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("downloading %s: HTTP %d", toolName, resp.StatusCode)
	}

	out, err := os.Create(zipFile)
	if err != nil {
		return "", fmt.Errorf("creating output file: %w", err)
	}
	defer out.Close()

	totalSize := resp.ContentLength
	if d.quiet || totalSize <= 0 {
		_, err = io.Copy(out, resp.Body)
	} else {
		progress := &ProgressReader{
			Reader:     resp.Body,
			Total:      totalSize,
			onProgress: d.printProgress,
		}
		_, err = io.Copy(out, progress)
		fmt.Println()
	}

	if err != nil {
		return "", fmt.Errorf("writing download to disk: %w", err)
	}

	fmt.Printf("Downloaded: %s\n", zipFile)
	return zipFile, nil
}

// printProgress renders a progress bar to stdout.
func (d *Downloader) printProgress(current, total int64) {
	percent := float64(current) / float64(total) * 100
	barWidth := 50
	completed := int(float64(barWidth) * float64(current) / float64(total))

	bar := strings.Repeat("=", completed)
	if completed < barWidth {
		bar += ">"
		bar += strings.Repeat(" ", barWidth-completed-1)
	}

	currentMB := float64(current) / 1024 / 1024
	totalMB := float64(total) / 1024 / 1024

	fmt.Printf("\r[%s] %.1f%% (%.2f MB / %.2f MB)", bar, percent, currentMB, totalMB)
}

// Unzip extracts the contents of zipPath into destPath.
func Unzip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("opening zip: %w", err)
	}
	defer r.Close()

	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("creating destination directory: %w", err)
	}

	for _, f := range r.File {
		fpath := filepath.Join(destPath, f.Name)

		// Guard against ZipSlip path traversal.
		if !strings.HasPrefix(fpath, filepath.Clean(destPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path in zip: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		if err := os.MkdirAll(filepath.Dir(fpath), os.ModePerm); err != nil {
			return err
		}

		outFile, err := os.OpenFile(fpath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		outFile.Close()
		rc.Close()

		if err != nil {
			return err
		}
	}

	return nil
}

// ProgressReader wraps an io.Reader and reports download progress.
type ProgressReader struct {
	Reader     io.Reader
	Total      int64
	Current    int64
	onProgress func(current, total int64)
}

// Read implements io.Reader.
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	if pr.onProgress != nil {
		pr.onProgress(pr.Current, pr.Total)
	}
	return n, err
}

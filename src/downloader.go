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

const (
	terraformCheckURL    = "https://checkpoint-api.hashicorp.com/v1/check/terraform"
	terraformDownloadURL = "https://releases.hashicorp.com/terraform/%s/terraform_%s_%s_%s.zip"
)

// TerraformCheck represents the response from HashiCorp's checkpoint API
type TerraformCheck struct {
	Product            string `json:"product"`
	CurrentVersion     string `json:"current_version"`
	CurrentRelease     int64  `json:"current_release"`
	CurrentDownloadURL string `json:"current_download_url"`
	CurrentChangelogURL string `json:"current_changelog_url"`
	ProjectWebsite     string `json:"project_website"`
	Alerts             []interface{} `json:"alerts"`
}

// Downloader handles downloading Terraform
type Downloader struct {
	targetOS   string
	targetArch string
	targetVer  string
	quiet      bool
	httpClient *http.Client
}

// NewDownloader creates a new Downloader
func NewDownloader(targetOS, targetArch, targetVer string, quiet bool) *Downloader {
	if targetOS == "" {
		targetOS = runtime.GOOS
	}
	if targetArch == "" {
		targetArch = runtime.GOARCH
	}

	// Configure HTTP client with proxy support
	client := &http.Client{
		Timeout: 30 * time.Minute,
		Transport: &http.Transport{
			Proxy: func(req *http.Request) (*url.URL, error) {
				// Check for https_proxy environment variable
				if proxyURL := os.Getenv("https_proxy"); proxyURL != "" {
					return url.Parse(proxyURL)
				}
				if proxyURL := os.Getenv("HTTPS_PROXY"); proxyURL != "" {
					return url.Parse(proxyURL)
				}
				// Also check http_proxy as fallback
				if proxyURL := os.Getenv("http_proxy"); proxyURL != "" {
					return url.Parse(proxyURL)
				}
				if proxyURL := os.Getenv("HTTP_PROXY"); proxyURL != "" {
					return url.Parse(proxyURL)
				}
				return nil, nil
			},
		},
	}

	return &Downloader{
		targetOS:   targetOS,
		targetArch: targetArch,
		targetVer:  targetVer,
		quiet:      quiet,
		httpClient: client,
	}
}

// GetLatestVersion fetches the latest version of Terraform
func (d *Downloader) GetLatestVersion() (string, error) {
	resp, err := d.httpClient.Get(terraformCheckURL)
	if err != nil {
		return "", fmt.Errorf("error fetching latest version: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error fetching latest version: status %d", resp.StatusCode)
	}

	var check TerraformCheck
	if err := json.NewDecoder(resp.Body).Decode(&check); err != nil {
		return "", fmt.Errorf("error decoding version info: %w", err)
	}

	if check.CurrentVersion == "" {
		return "", fmt.Errorf("no version found in response")
	}

	return check.CurrentVersion, nil
}

// Download downloads the specified version of Terraform
func (d *Downloader) Download() (string, error) {
	ver := d.targetVer
	if ver == "" {
		latest, err := d.GetLatestVersion()
		if err != nil {
			return "", err
		}
		ver = latest
	}

	// Remove 'v' prefix if present
	ver = strings.TrimPrefix(ver, "v")

	downloadURL := fmt.Sprintf(terraformDownloadURL, ver, ver, d.targetOS, d.targetArch)
	zipFile := fmt.Sprintf("terraform_%s_%s_%s.zip", ver, d.targetOS, d.targetArch)

	fmt.Printf("Downloading Terraform %s for %s/%s...\n", ver, d.targetOS, d.targetArch)
	if !d.quiet {
		fmt.Printf("URL: %s\n", downloadURL)
	}

	// Download the file
	resp, err := d.httpClient.Get(downloadURL)
	if err != nil {
		return "", fmt.Errorf("error downloading: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("error downloading: status %d", resp.StatusCode)
	}

	// Create the zip file
	out, err := os.Create(zipFile)
	if err != nil {
		return "", fmt.Errorf("error creating file: %w", err)
	}
	defer out.Close()

	// Get total size for progress bar
	totalSize := resp.ContentLength

	// Write the body to file with progress bar
	if d.quiet || totalSize <= 0 {
		// No progress bar in quiet mode or if size is unknown
		_, err = io.Copy(out, resp.Body)
	} else {
		// Use progress bar
		progress := &ProgressReader{
			Reader:    resp.Body,
			Total:     totalSize,
			onProgress: d.printProgress,
		}
		_, err = io.Copy(out, progress)
		fmt.Println() // New line after progress bar
	}

	if err != nil {
		return "", fmt.Errorf("error writing file: %w", err)
	}

	fmt.Printf("Downloaded to: %s\n", zipFile)
	return zipFile, nil
}

// printProgress prints a progress bar
func (d *Downloader) printProgress(current, total int64) {
	percent := float64(current) / float64(total) * 100
	barWidth := 50
	completed := int(float64(barWidth) * float64(current) / float64(total))
	
	bar := strings.Repeat("=", completed)
	if completed < barWidth {
		bar += ">"
		bar += strings.Repeat(" ", barWidth-completed-1)
	}
	
	// Format sizes
	currentMB := float64(current) / 1024 / 1024
	totalMB := float64(total) / 1024 / 1024
	
	fmt.Printf("\r[%s] %.1f%% (%.2f MB / %.2f MB)", bar, percent, currentMB, totalMB)
}

// Unzip extracts the terraform binary from the zip file
func Unzip(zipPath, destPath string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return fmt.Errorf("error opening zip: %w", err)
	}
	defer r.Close()

	// Ensure destination directory exists
	if err := os.MkdirAll(destPath, 0755); err != nil {
		return fmt.Errorf("error creating destination directory: %w", err)
	}

	for _, f := range r.File {
		// Construct the full path for the file
		fpath := filepath.Join(destPath, f.Name)

		// Check for ZipSlip vulnerability
		if !strings.HasPrefix(fpath, filepath.Clean(destPath)+string(os.PathSeparator)) {
			return fmt.Errorf("illegal file path: %s", fpath)
		}

		if f.FileInfo().IsDir() {
			os.MkdirAll(fpath, os.ModePerm)
			continue
		}

		// Create the file
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

// GetVersion returns the target version (or latest if not specified)
func (d *Downloader) GetVersion() (string, error) {
	if d.targetVer != "" {
		return strings.TrimPrefix(d.targetVer, "v"), nil
	}
	return d.GetLatestVersion()
}

// ProgressReader wraps an io.Reader to track download progress
type ProgressReader struct {
	Reader     io.Reader
	Total      int64
	Current    int64
	onProgress func(current, total int64)
}

// Read implements io.Reader interface with progress tracking
func (pr *ProgressReader) Read(p []byte) (int, error) {
	n, err := pr.Reader.Read(p)
	pr.Current += int64(n)
	
	if pr.onProgress != nil {
		pr.onProgress(pr.Current, pr.Total)
	}
	
	return n, err
}

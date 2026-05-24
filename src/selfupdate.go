package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"runtime"
	"strings"
	"time"
)

const githubRepo = "deese/tfdown"

type githubRelease struct {
	TagName string `json:"tag_name"`
}

// selfUpdate downloads and installs the latest tfdown release from GitHub,
// replacing the current executable using the rename trick (safe on Windows).
func selfUpdate(currentVersion string, quiet bool) error {
	client := &http.Client{Timeout: 5 * time.Minute}

	latestVersion, err := getLatestTfdownVersion(client)
	if err != nil {
		return fmt.Errorf("checking latest version: %w", err)
	}

	if latestVersion == currentVersion {
		fmt.Printf("tfdown is already up to date (v%s)\n", currentVersion)
		return nil
	}

	fmt.Printf("Updating tfdown %s → %s\n", currentVersion, latestVersion)

	exePath, err := os.Executable()
	if err != nil {
		return fmt.Errorf("getting executable path: %w", err)
	}

	ext := ""
	if runtime.GOOS == "windows" {
		ext = ".exe"
	}

	downloadURL := fmt.Sprintf(
		"https://github.com/%s/releases/download/v%s/tfdown-%s-%s%s",
		githubRepo, latestVersion, runtime.GOOS, runtime.GOARCH, ext,
	)

	if !quiet {
		fmt.Printf("URL: %s\n", downloadURL)
	}

	tmpFile, err := os.CreateTemp("", "tfdown-update-*")
	if err != nil {
		return fmt.Errorf("creating temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	defer os.Remove(tmpPath)

	resp, err := client.Get(downloadURL)
	if err != nil {
		tmpFile.Close()
		return fmt.Errorf("downloading update: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		tmpFile.Close()
		return fmt.Errorf("downloading update: HTTP %d", resp.StatusCode)
	}

	totalSize := resp.ContentLength
	if quiet || totalSize <= 0 {
		_, err = io.Copy(tmpFile, resp.Body)
	} else {
		d := &Downloader{quiet: false}
		progress := &ProgressReader{
			Reader:     resp.Body,
			Total:      totalSize,
			onProgress: d.printProgress,
		}
		_, err = io.Copy(tmpFile, progress)
		fmt.Println()
	}
	tmpFile.Close()
	if err != nil {
		return fmt.Errorf("writing update: %w", err)
	}

	// Rename trick: rename the running exe so we can write the new binary in its place.
	// On Windows, you cannot overwrite a running executable, but you CAN rename it.
	// The current process keeps running from the renamed copy until it exits.
	oldPath := exePath + ".old"
	os.Remove(oldPath) // clean up any leftover from a previous update
	if err := os.Rename(exePath, oldPath); err != nil {
		return fmt.Errorf("renaming current binary: %w", err)
	}

	if err := copyFile(tmpPath, exePath); err != nil {
		os.Rename(oldPath, exePath) // rollback
		return fmt.Errorf("installing update: %w", err)
	}

	if runtime.GOOS != "windows" {
		os.Chmod(exePath, 0755)
	}

	// Try to remove the old binary immediately; on Windows this may fail if
	// another process still has it open — cleanupOldBinary() will retry on next run.
	os.Remove(oldPath)

	fmt.Printf("tfdown updated to v%s\n", latestVersion)
	return nil
}

// cleanupOldBinary removes the .old file left by a previous self-update.
// Called at startup so the cleanup happens silently on the next invocation.
func cleanupOldBinary() {
	exe, err := os.Executable()
	if err != nil {
		return
	}
	os.Remove(exe + ".old")
}

// getLatestTfdownVersion queries the GitHub Releases API for the latest tag.
func getLatestTfdownVersion(client *http.Client) (string, error) {
	apiURL := fmt.Sprintf("https://api.github.com/repos/%s/releases/latest", githubRepo)

	resp, err := client.Get(apiURL)
	if err != nil {
		return "", fmt.Errorf("fetching release info: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("GitHub API returned HTTP %d", resp.StatusCode)
	}

	var release githubRelease
	if err := json.NewDecoder(resp.Body).Decode(&release); err != nil {
		return "", fmt.Errorf("decoding release info: %w", err)
	}

	if release.TagName == "" {
		return "", fmt.Errorf("no releases found for %s", githubRepo)
	}

	return strings.TrimPrefix(release.TagName, "v"), nil
}

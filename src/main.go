package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "1.0.0"

func main() {
	// Define command line flags
	var (
		targetOS      = flag.String("os", "", "Target OS (linux, windows, darwin, openbsd, solaris)")
		targetArch    = flag.String("arch", "", "Target architecture (amd64, 386, arm64, arm)")
		targetVersion = flag.String("ver", "", "Target Terraform version (e.g., 1.7.0)")
		showVersion   = flag.Bool("version", false, "Show tfdown version")
		installFlag   = flag.Bool("install", false, "Enable automatic installation")
		installPath   = flag.String("install-path", "", "Path to install Terraform")
		quietMode     = flag.Bool("quiet", false, "Quiet mode, no progress bar")
		forceFlag     = flag.Bool("force", false, "Force download and install even if already up to date")
		showHelp      = flag.Bool("help", false, "Show help message")
	)

	flag.BoolVar(quietMode, "q", false, "Quiet mode (shorthand)")
	flag.BoolVar(forceFlag, "f", false, "Force download (shorthand)")

	flag.Parse()

	if *showHelp {
		printHelp()
		return
	}

	if *showVersion {
		fmt.Printf("tfdown version %s\n", version)
		return
	}

	// Load configuration
	config := NewConfig()
	if err := config.Load(); err != nil {
		fmt.Printf("Warning: Could not load config: %v\n", err)
	}

	// Determine if this is an auto-update run (no arguments provided)
	autoUpdate := flag.NFlag() == 0

	// If auto-update and install is configured, use config settings
	if autoUpdate && config.Install && config.InstallPath != "" {
		*installFlag = true
		*installPath = config.InstallPath
	}

	// Create downloader
	downloader := NewDownloader(*targetOS, *targetArch, *targetVersion, *quietMode)

	// Get the version we're going to download
	ver, err := downloader.GetVersion()
	if err != nil {
		fmt.Printf("Error getting version: %v\n", err)
		os.Exit(1)
	}

	// Check if we already have this version in auto-update mode
	if autoUpdate && config.Version == ver && !*installFlag && !*forceFlag {
		fmt.Printf("Already up to date (version %s)\n", ver)
		return
	}

	// If force flag is set and auto-install is configured, enable installation
	if *forceFlag && config.Install && config.InstallPath != "" {
		*installFlag = true
		*installPath = config.InstallPath
	}

	// Download Terraform
	zipFile, err := downloader.Download()
	if err != nil {
		fmt.Printf("Error downloading: %v\n", err)
		os.Exit(1)
	}

	// Update configuration
	if err := config.Update(ver, *installFlag, *installPath); err != nil {
		fmt.Printf("Warning: Could not save config: %v\n", err)
	}

	// Install if requested
	if *installFlag && *installPath != "" {
		if err := installTerraform(zipFile, *installPath); err != nil {
			fmt.Printf("Error installing: %v\n", err)
			os.Exit(1)
		}
		fmt.Printf("Successfully installed Terraform %s to %s\n", ver, *installPath)
	} else {
		fmt.Printf("\nDownload complete! Terraform %s is ready.\n", ver)
		fmt.Printf("To install automatically next time, use:\n")
		fmt.Printf("  tfdown --install --install-path /path/to/install\n")
	}
}

func installTerraform(zipFile, installPath string) error {
	// Check if install path exists
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("install path does not exist: %s", installPath)
	}

	// Create a temporary directory for extraction
	tempDir, err := os.MkdirTemp("", "tfdown-*")
	if err != nil {
		return fmt.Errorf("error creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	// Extract the zip file
	fmt.Printf("Extracting %s...\n", zipFile)
	if err := Unzip(zipFile, tempDir); err != nil {
		return fmt.Errorf("error extracting zip: %w", err)
	}

	// Determine which binary exists in the extracted files
	// Check the zip filename to determine if it's Windows or Unix binary
	var srcPath, binaryName string

	binaryName = "terraform"
	if strings.Contains(zipFile, "windows") {
		binaryName = "terraform.exe"
	}

	srcPath = filepath.Join(tempDir, binaryName)

	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("terraform binary not found in zip: %s", srcPath)
	}

	// Use the same binary name for destination
	dstPath := filepath.Join(installPath, binaryName)

	// Copy the binary
	fmt.Printf("Installing to %s...\n", dstPath)
	if err := copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("error copying binary: %w", err)
	}

	// Make it executable on Unix systems
	if runtime.GOOS != "windows" {
		if err := os.Chmod(dstPath, 0755); err != nil {
			return fmt.Errorf("error making binary executable: %w", err)
		}
	}

	// Clean up the zip file
	os.Remove(zipFile)

	return nil
}

func copyFile(src, dst string) error {
	// Read the source file
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}

	// Write to destination
	return os.WriteFile(dst, data, 0755)
}

func printHelp() {
	fmt.Printf("tfdown v%s - Terraform downloader\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  tfdown [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --os string           Target OS (linux, windows, darwin, openbsd, solaris)")
	fmt.Println("                        Default: current OS")
	fmt.Println("  --arch string         Target architecture (amd64, 386, arm64, arm)")
	fmt.Println("                        Default: current architecture")
	fmt.Println("  --ver string          Target Terraform version (e.g., 1.7.0)")
	fmt.Println("                        Default: latest stable version")
	fmt.Println("  --install             Enable automatic installation")
	fmt.Println("  --install-path string Path to install Terraform")
	fmt.Println("  -f, --force           Force download and install even if already up to date")
	fmt.Println("  -q, --quiet           Quiet mode, disable progress bar")
	fmt.Println("  --version             Show tfdown version")
	fmt.Println("  --help                Show this help message")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Download latest version for current platform")
	fmt.Println("  tfdown")
	fmt.Println()
	fmt.Println("  # Download and install to /usr/local/bin")
	fmt.Println("  tfdown --install --install-path /usr/local/bin")
	fmt.Println()
	fmt.Println("  # Force re-download and install even if up to date")
	fmt.Println("  tfdown -f")
	fmt.Println()
	fmt.Println("  # Download specific version for Linux ARM64")
	fmt.Println("  tfdown --ver 1.7.0 --os linux --arch arm64")
	fmt.Println()
	fmt.Println("Configuration:")
	fmt.Println("  Config file: ~/.tfdown.conf")
	fmt.Println("  The tool saves the last downloaded version and install settings.")
	fmt.Println("  When run without arguments, it will check for updates and install")
	fmt.Println("  automatically if configured.")
}

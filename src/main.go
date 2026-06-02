package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "1.2.1"

// toolRequest describes a tool to download in this run.
type toolRequest struct {
	name    string // must match a key in KnownTools
	version string // user-pinned version; empty means latest
	enabled bool
}

func main() {
	var (
		targetOS      = flag.String("os", "", "Target OS (linux, windows, darwin, openbsd, solaris)")
		targetArch    = flag.String("arch", "", "Target architecture (amd64, 386, arm64, arm)")
		targetVersion = flag.String("ver", "", "Terraform version (e.g. 1.7.0)")
		targetLSVer   = flag.String("ls-ver", "", "terraform-ls version (e.g. 0.38.0)")
		targetPackerV = flag.String("packer-ver", "", "Packer version (e.g. 1.15.3); implies --packer")
		noLS          = flag.Bool("no-ls", false, "Skip terraform-ls")
		withPacker    = flag.Bool("packer", false, "Also download Packer")
		showVersion   = flag.Bool("version", false, "Print tfdown version")
		installFlag   = flag.Bool("install", false, "Install binaries after downloading")
		installPath   = flag.String("install-path", "", "Directory to install binaries into")
		quietMode     = flag.Bool("quiet", false, "Quiet mode (no progress bar)")
		forceFlag     = flag.Bool("force", false, "Force download even if already up to date")
		showHelp      = flag.Bool("help", false, "Show help")
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

	config := NewConfig()
	if err := config.Load(); err != nil {
		fmt.Printf("Warning: could not load config: %v\n", err)
	}

	autoUpdate := flag.NFlag() == 0
	if autoUpdate && config.Install && config.InstallPath != "" {
		*installFlag = true
		*installPath = config.InstallPath
	}
	if *forceFlag && config.Install && config.InstallPath != "" {
		*installFlag = true
		*installPath = config.InstallPath
	}

	// To add a new tool: one flag above + one line here.
	tools := []toolRequest{
		{name: "terraform", version: *targetVersion, enabled: true},
		{name: "terraform-ls", version: *targetLSVer, enabled: !*noLS},
		{name: "packer", version: *targetPackerV, enabled: *withPacker || *targetPackerV != ""},
	}

	// Build the version map for the downloader (pinned versions only).
	pinnedVersions := make(map[string]string)
	for _, t := range tools {
		if t.version != "" {
			pinnedVersions[t.name] = t.version
		}
	}

	downloader := NewDownloader(*targetOS, *targetArch, pinnedVersions, *quietMode)

	// Resolve final versions for all enabled tools.
	resolved := make(map[string]string)
	for _, t := range tools {
		if !t.enabled {
			continue
		}
		ver, err := downloader.GetVersion(t.name)
		if err != nil {
			fmt.Printf("Error getting version for %s: %v\n", t.name, err)
			os.Exit(1)
		}
		resolved[t.name] = ver
	}

	// In auto-update mode, exit early if everything is current.
	if autoUpdate && !*forceFlag && !*installFlag {
		allCurrent := true
		for _, t := range tools {
			if t.enabled && config.GetVersion(t.name) != resolved[t.name] {
				allCurrent = false
				break
			}
		}
		if allCurrent {
			parts := make([]string, 0, len(tools))
			for _, t := range tools {
				if t.enabled {
					parts = append(parts, fmt.Sprintf("%s %s", t.name, resolved[t.name]))
				}
			}
			fmt.Printf("Already up to date (%s)\n", strings.Join(parts, ", "))
			return
		}
	}

	// Download tools that need updating.
	downloaded := make(map[string]string) // tool name -> zip path

	for _, t := range tools {
		if !t.enabled {
			continue
		}
		needsDownload := !autoUpdate || config.GetVersion(t.name) != resolved[t.name] || *forceFlag
		if !needsDownload {
			continue
		}

		zipFile, err := downloader.Download(t.name)
		if err != nil {
			fmt.Printf("Error downloading %s: %v\n", t.name, err)
			os.Exit(1)
		}
		downloaded[t.name] = zipFile
		config.SetVersion(t.name, resolved[t.name])
	}

	// Persist updated versions and install settings.
	config.Install = *installFlag
	if *installPath != "" {
		config.InstallPath = *installPath
	}
	if err := config.Save(); err != nil {
		fmt.Printf("Warning: could not save config: %v\n", err)
	}

	// Install or report.
	if *installFlag && *installPath != "" {
		for _, t := range tools {
			zipFile, ok := downloaded[t.name]
			if !ok {
				continue
			}
			if err := installBinary(zipFile, t.name, *installPath); err != nil {
				fmt.Printf("Error installing %s: %v\n", t.name, err)
				os.Exit(1)
			}
			fmt.Printf("Installed %s %s to %s\n", t.name, resolved[t.name], *installPath)
		}
	} else {
		for _, t := range tools {
			if _, ok := downloaded[t.name]; ok {
				fmt.Printf("Download complete: %s %s\n", t.name, resolved[t.name])
			}
		}
		if len(downloaded) > 0 {
			fmt.Println("\nTo install automatically next time, run:")
			fmt.Println("  tfdown --install --install-path /path/to/bin")
		}
	}
}

func installBinary(zipFile, toolName, installPath string) error {
	if _, err := os.Stat(installPath); os.IsNotExist(err) {
		return fmt.Errorf("install path does not exist: %s", installPath)
	}

	tempDir, err := os.MkdirTemp("", "tfdown-*")
	if err != nil {
		return fmt.Errorf("creating temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	fmt.Printf("Extracting %s...\n", zipFile)
	if err := Unzip(zipFile, tempDir); err != nil {
		return fmt.Errorf("extracting zip: %w", err)
	}

	binaryName := toolName
	if strings.Contains(zipFile, "windows") {
		binaryName = toolName + ".exe"
	}

	srcPath := filepath.Join(tempDir, binaryName)
	if _, err := os.Stat(srcPath); err != nil {
		return fmt.Errorf("binary not found in zip: %s", srcPath)
	}

	dstPath := filepath.Join(installPath, binaryName)
	fmt.Printf("Installing to %s...\n", dstPath)

	if err := copyFile(srcPath, dstPath); err != nil {
		return fmt.Errorf("copying binary: %w", err)
	}

	if runtime.GOOS != "windows" {
		if err := os.Chmod(dstPath, 0755); err != nil {
			return fmt.Errorf("setting executable permission: %w", err)
		}
	}

	os.Remove(zipFile)
	return nil
}

func copyFile(src, dst string) error {
	data, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, data, 0755)
}

func printHelp() {
	fmt.Printf("tfdown v%s - HashiCorp tools downloader\n\n", version)
	fmt.Println("Usage:")
	fmt.Println("  tfdown [flags]")
	fmt.Println()
	fmt.Println("Flags:")
	fmt.Println("  --os string           Target OS (linux, windows, darwin, openbsd, solaris)")
	fmt.Println("                        Default: current OS")
	fmt.Println("  --arch string         Target architecture (amd64, 386, arm64, arm)")
	fmt.Println("                        Default: current architecture")
	fmt.Println("  --ver string          Terraform version (e.g. 1.7.0)")
	fmt.Println("  --ls-ver string       terraform-ls version (e.g. 0.38.0)")
	fmt.Println("  --packer-ver string   Packer version (e.g. 1.15.3); implies --packer")
	fmt.Println("  --no-ls               Skip terraform-ls")
	fmt.Println("  --packer              Also download Packer")
	fmt.Println("  --install             Install binaries after downloading")
	fmt.Println("  --install-path string Directory to install binaries into")
	fmt.Println("  -f, --force           Force download even if already up to date")
	fmt.Println("  -q, --quiet           Quiet mode (no progress bar)")
	fmt.Println("  --version             Print tfdown version")
	fmt.Println("  --help                Show this help")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Download latest versions for the current platform")
	fmt.Println("  tfdown")
	fmt.Println()
	fmt.Println("  # Download and install to /usr/local/bin")
	fmt.Println("  tfdown --install --install-path /usr/local/bin")
	fmt.Println()
	fmt.Println("  # Terraform only (skip terraform-ls)")
	fmt.Println("  tfdown --no-ls")
	fmt.Println()
	fmt.Println("  # Terraform + Packer")
	fmt.Println("  tfdown --packer")
	fmt.Println()
	fmt.Println("  # Force re-download and install")
	fmt.Println("  tfdown -f")
	fmt.Println()
	fmt.Println("  # Specific versions for Linux ARM64")
	fmt.Println("  tfdown --ver 1.7.0 --ls-ver 0.38.0 --os linux --arch arm64")
	fmt.Println()
	fmt.Println("Config file: ~/.tfdown.conf")
}

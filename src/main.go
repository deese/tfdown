package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
)

const version = "1.3.0"

func main() {
	var (
		targetOS    = flag.String("os", "", "Target OS (linux, windows, darwin, openbsd, solaris)")
		targetArch  = flag.String("arch", "", "Target architecture (amd64, 386, arm64, arm)")
		showVersion = flag.Bool("version", false, "Print tfdown version")
		installFlag = flag.Bool("install", false, "Install binaries after downloading")
		installPath = flag.String("install-path", "", "Directory to install binaries into")
		quietMode   = flag.Bool("quiet", false, "Quiet mode (no progress bar)")
		forceFlag   = flag.Bool("force", false, "Force download even if already up to date")
		showHelp    = flag.Bool("help", false, "Show help")
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
	if (autoUpdate || *forceFlag) && config.Install && config.InstallPath != "" {
		*installFlag = true
		*installPath = config.InstallPath
	}

	apps := config.GetApps()

	downloader := NewDownloader(*targetOS, *targetArch, nil, *quietMode)

	// Resolve final versions for all apps.
	resolved := make(map[string]string)
	for _, app := range apps {
		ver, err := downloader.GetVersion(app)
		if err != nil {
			fmt.Printf("Error getting version for %s: %v\n", app, err)
			os.Exit(1)
		}
		resolved[app] = ver
	}

	// In auto-update mode, exit early if everything is current.
	if autoUpdate && !*forceFlag && !*installFlag {
		allCurrent := true
		for _, app := range apps {
			if config.GetVersion(app) != resolved[app] {
				allCurrent = false
				break
			}
		}
		if allCurrent {
			parts := make([]string, 0, len(apps))
			for _, app := range apps {
				parts = append(parts, fmt.Sprintf("%s %s", app, resolved[app]))
			}
			fmt.Printf("Already up to date (%s)\n", strings.Join(parts, ", "))
			return
		}
	}

	// Download apps that need updating.
	downloaded := make(map[string]string) // app name -> zip path

	for _, app := range apps {
		needsDownload := !autoUpdate || config.GetVersion(app) != resolved[app] || *forceFlag
		if !needsDownload {
			continue
		}

		zipFile, err := downloader.Download(app)
		if err != nil {
			fmt.Printf("Error downloading %s: %v\n", app, err)
			os.Exit(1)
		}
		downloaded[app] = zipFile
		config.SetVersion(app, resolved[app])
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
		for _, app := range apps {
			zipFile, ok := downloaded[app]
			if !ok {
				continue
			}
			if err := installBinary(zipFile, app, *installPath); err != nil {
				fmt.Printf("Error installing %s: %v\n", app, err)
				os.Exit(1)
			}
			fmt.Printf("Installed %s %s to %s\n", app, resolved[app], *installPath)
		}
	} else {
		for _, app := range apps {
			if _, ok := downloaded[app]; ok {
				fmt.Printf("Download complete: %s %s\n", app, resolved[app])
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
	fmt.Println("  --install             Install binaries after downloading")
	fmt.Println("  --install-path string Directory to install binaries into")
	fmt.Println("  -f, --force           Force download even if already up to date")
	fmt.Println("  -q, --quiet           Quiet mode (no progress bar)")
	fmt.Println("  --version             Print tfdown version")
	fmt.Println("  --help                Show this help")
	fmt.Println()
	fmt.Println("Apps are configured via the 'apps' key in ~/.tfdown.conf.")
	fmt.Println("If the key is absent, only terraform is downloaded.")
	fmt.Println()
	fmt.Println("Config file (~/.tfdown.conf):")
	fmt.Println("  apps=terraform,terraform-ls,packer")
	fmt.Println("  install=true")
	fmt.Println("  install_path=/usr/local/bin")
	fmt.Println()
	fmt.Println("Examples:")
	fmt.Println("  # Download latest versions (apps from config, default: terraform)")
	fmt.Println("  tfdown")
	fmt.Println()
	fmt.Println("  # Download and install to /usr/local/bin")
	fmt.Println("  tfdown --install --install-path /usr/local/bin")
	fmt.Println()
	fmt.Println("  # Force re-download and install")
	fmt.Println("  tfdown -f")
	fmt.Println()
	fmt.Println("  # Download for Linux ARM64")
	fmt.Println("  tfdown --os linux --arch arm64")
}

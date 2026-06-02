package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

// Config holds the persistent configuration for tfdown.
type Config struct {
	Apps        []string          // tools to download; empty defaults to ["terraform"]
	Versions    map[string]string // tool name -> last downloaded version
	Install     bool
	InstallPath string
	configPath  string
}

// NewConfig returns a Config with default values.
func NewConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	return &Config{
		Apps:       []string{},
		Versions:   make(map[string]string),
		configPath: filepath.Join(homeDir, ".tfdown.conf"),
	}
}

// Load reads configuration from ~/.tfdown.conf.
// Accepts both the new format (version.TOOL=x.y.z) and the legacy format
// (version=x.y.z / ls_version=x.y.z) for backward compatibility.
func (c *Config) Load() error {
	file, err := os.Open(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("opening config file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		parts := strings.SplitN(line, "=", 2)
		if len(parts) != 2 {
			continue
		}

		key := strings.TrimSpace(parts[0])
		value := strings.TrimSpace(parts[1])

		switch {
		case key == "apps":
			for _, app := range strings.Split(value, ",") {
				app = strings.TrimSpace(app)
				if app != "" {
					c.Apps = append(c.Apps, app)
				}
			}
		case strings.HasPrefix(key, "version."):
			// New format: version.terraform=1.7.0
			c.Versions[strings.TrimPrefix(key, "version.")] = value
		case key == "version":
			// Legacy: terraform
			c.Versions["terraform"] = value
		case key == "ls_version":
			// Legacy: terraform-ls
			c.Versions["terraform-ls"] = value
		case key == "install":
			c.Install, _ = strconv.ParseBool(value)
		case key == "install_path":
			c.InstallPath = value
		}
	}

	return scanner.Err()
}

// Save writes the current configuration to ~/.tfdown.conf.
func (c *Config) Save() error {
	var sb strings.Builder
	sb.WriteString("# tfdown configuration file\n")
	fmt.Fprintf(&sb, "# Last updated: %s\n\n", time.Now().Format("2006-01-02"))

	if len(c.Apps) > 0 {
		fmt.Fprintf(&sb, "apps=%s\n", strings.Join(c.Apps, ","))
	}

	for toolName, ver := range c.Versions {
		fmt.Fprintf(&sb, "version.%s=%s\n", toolName, ver)
	}

	fmt.Fprintf(&sb, "install=%t\n", c.Install)
	fmt.Fprintf(&sb, "install_path=%s\n", c.InstallPath)

	return os.WriteFile(c.configPath, []byte(sb.String()), 0644)
}

// GetApps returns the configured apps, defaulting to ["terraform"] if none are set.
func (c *Config) GetApps() []string {
	if len(c.Apps) == 0 {
		return []string{"terraform"}
	}
	return c.Apps
}

// SetVersion stores the downloaded version for a tool.
func (c *Config) SetVersion(toolName, ver string) {
	c.Versions[toolName] = ver
}

// GetVersion returns the stored version for a tool, or empty string if unknown.
func (c *Config) GetVersion(toolName string) string {
	return c.Versions[toolName]
}

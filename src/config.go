package main

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// Config holds the configuration for tfdown
type Config struct {
	Version     string
	Install     bool
	InstallPath string
	configPath  string
}

// NewConfig creates a new Config instance with default values
func NewConfig() *Config {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		homeDir = "."
	}

	configPath := filepath.Join(homeDir, ".tfdown.conf")
	return &Config{
		Version:     "",
		Install:     false,
		InstallPath: "",
		configPath:  configPath,
	}
}

// Load reads the configuration from the config file
func (c *Config) Load() error {
	file, err := os.Open(c.configPath)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return default config
			return nil
		}
		return fmt.Errorf("error opening config file: %w", err)
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

		switch key {
		case "version":
			c.Version = value
		case "install":
			c.Install, _ = strconv.ParseBool(value)
		case "install_path":
			c.InstallPath = value
		}
	}

	if err := scanner.Err(); err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	return nil
}

// Save writes the configuration to the config file
func (c *Config) Save() error {
	content := fmt.Sprintf("# tfdown configuration file\n")
	content += fmt.Sprintf("# Last updated: %s\n\n", getCurrentDate())
	content += fmt.Sprintf("version=%s\n", c.Version)
	content += fmt.Sprintf("install=%t\n", c.Install)
	content += fmt.Sprintf("install_path=%s\n", c.InstallPath)

	err := os.WriteFile(c.configPath, []byte(content), 0644)
	if err != nil {
		return fmt.Errorf("error writing config file: %w", err)
	}

	return nil
}

// Update updates the configuration with new values and saves it
func (c *Config) Update(version string, install bool, installPath string) error {
	if version != "" {
		c.Version = version
	}
	c.Install = install
	c.InstallPath = installPath
	return c.Save()
}

func getCurrentDate() string {
	return "2026-02-05" // Placeholder, in real implementation use time.Now()
}

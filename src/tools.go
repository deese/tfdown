package main

import "fmt"

// Tool represents a HashiCorp product that can be downloaded.
// URLs are derived from the name; all HashiCorp tools follow the same pattern.
type Tool struct {
	Name string
}

// CheckURL returns the checkpoint API URL used to query the latest version.
func (t *Tool) CheckURL() string {
	return fmt.Sprintf("https://checkpoint-api.hashicorp.com/v1/check/%s", t.Name)
}

// DownloadURLFmt returns the format string for the release download URL.
// Format specifiers: ver, ver, os, arch.
func (t *Tool) DownloadURLFmt() string {
	return fmt.Sprintf(
		"https://releases.hashicorp.com/%s/%%s/%s_%%s_%%s_%%s.zip",
		t.Name, t.Name,
	)
}

// KnownTools is the registry of downloadable tools.
// To add a new tool, add one line here.
var KnownTools = map[string]*Tool{
	"terraform":    {Name: "terraform"},
	"terraform-ls": {Name: "terraform-ls"},
	"packer":       {Name: "packer"},
}

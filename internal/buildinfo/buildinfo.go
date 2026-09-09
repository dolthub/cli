// Package buildinfo records executable provenance without accessing the checkout.
package buildinfo

import (
	"regexp"
	"runtime/debug"
)

// Release packaging stamps these values. Ordinary builds use Go VCS metadata.
var Commit string
var Release string
var Dirty string

type Info struct {
	Version          string  `json:"cli_version"`
	Commit           *string `json:"source_commit"`
	Dirty            *bool   `json:"source_dirty"`
	PublicationReady bool    `json:"publication_ready"`
}

var fullRevision = regexp.MustCompile(`^[0-9a-f]{40}$`)
var releaseVersion = regexp.MustCompile(`^v?[0-9]+\.[0-9]+\.[0-9]+(?:[-+][0-9A-Za-z.-]+)?$`)

func Current(version string) Info {
	commit, dirty := Commit, Dirty
	if info, ok := debug.ReadBuildInfo(); ok {
		for _, setting := range info.Settings {
			switch setting.Key {
			case "vcs.revision":
				if Commit == "" {
					commit = setting.Value
				}
			case "vcs.modified":
				if Dirty == "" {
					dirty = setting.Value
				}
			}
		}
	}
	return New(version, commit, dirty, Release == "true")
}

// New normalizes unknown provenance; a version string alone is not a release.
func New(version, commit, dirty string, release bool) Info {
	info := Info{Version: version}
	if fullRevision.MatchString(commit) {
		info.Commit = &commit
	}
	if dirty == "true" || dirty == "false" {
		value := dirty == "true"
		info.Dirty = &value
	}
	info.PublicationReady = release && releaseVersion.MatchString(version) && info.Commit != nil && info.Dirty != nil && !*info.Dirty
	return info
}

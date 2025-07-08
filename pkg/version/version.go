package version

import (
	"fmt"
	"runtime"
	"strconv"
	"time"
)

// Build information set via ldflags during build
var (
	// Major version number
	Major = "1"
	
	// Minor version number (auto-incremented by commit count)
	Minor = "0"
	
	// Patch version number
	Patch = "0"
	
	// PreRelease identifier (e.g., "alpha", "beta", "rc1")
	PreRelease = ""
	
	// BuildDate is the date the binary was built
	BuildDate = "unknown"
	
	// CommitHash is the git commit hash
	CommitHash = "unknown"
	
	// CommitCount is the number of commits (used for minor version)
	CommitCount = "0"
	
	// GoVersion is the Go version used to build
	GoVersion = runtime.Version()
	
	// BuildUser is the user who built the binary
	BuildUser = "unknown"
)

// Info represents version information
type Info struct {
	Version    string `json:"version"`
	Major      string `json:"major"`
	Minor      string `json:"minor"`
	Patch      string `json:"patch"`
	PreRelease string `json:"pre_release,omitempty"`
	BuildDate  string `json:"build_date"`
	CommitHash string `json:"commit_hash"`
	GoVersion  string `json:"go_version"`
	BuildUser  string `json:"build_user,omitempty"`
}

// GetSemanticVersion returns the semantic version string
func GetSemanticVersion() string {
	// Use commit count as minor version if available
	minor := Minor
	if CommitCount != "0" && CommitCount != "unknown" {
		if count, err := strconv.Atoi(CommitCount); err == nil && count > 0 {
			minor = CommitCount
		}
	}
	
	version := fmt.Sprintf("%s.%s.%s", Major, minor, Patch)
	if PreRelease != "" {
		version += "-" + PreRelease
	}
	return version
}

// GetInfo returns version information
func GetInfo() Info {
	return Info{
		Version:    GetSemanticVersion(),
		Major:      Major,
		Minor:      Minor,
		Patch:      Patch,
		PreRelease: PreRelease,
		BuildDate:  BuildDate,
		CommitHash: CommitHash,
		GoVersion:  GoVersion,
		BuildUser:  BuildUser,
	}
}

// GetInfoWithoutBuildUser returns version information without build user
func GetInfoWithoutBuildUser() Info {
	info := GetInfo()
	info.BuildUser = ""
	return info
}

// GetVersionString returns a formatted version string for --version flag
func GetVersionString() string {
	return GetSemanticVersion()
}

// GetVersionWithBuildInfo returns version with build date and commit hash on next line
func GetVersionWithBuildInfo() string {
	version := GetSemanticVersion()
	if BuildDate != "unknown" && CommitHash != "unknown" {
		buildInfo := fmt.Sprintf("Built %s from commit %s", FormatBuildDate(), CommitHash)
		return fmt.Sprintf("%s\n%s", version, buildInfo)
	}
	return version
}

// GetDetailedVersionString returns a detailed version string
func GetDetailedVersionString() string {
	info := GetInfo()
	
	result := fmt.Sprintf("Version:      %s\n", info.Version)
	result += fmt.Sprintf("Build Date:   %s\n", info.BuildDate)
	result += fmt.Sprintf("Commit Hash:  %s\n", info.CommitHash)
	result += fmt.Sprintf("Go Version:   %s\n", info.GoVersion)
	result += fmt.Sprintf("Build User:   %s\n", info.BuildUser)
	
	return result
}

// FormatBuildDate parses the build date and returns a formatted string
func FormatBuildDate() string {
	if BuildDate == "unknown" {
		return BuildDate
	}
	
	// Try to parse the build date and format it nicely
	if t, err := time.Parse(time.RFC3339, BuildDate); err == nil {
		return t.Format("2006-01-02 15:04:05 MST")
	}
	
	// If RFC3339 fails, try Unix timestamp
	if t, err := time.Parse("1136214245", BuildDate); err == nil {
		return t.Format("2006-01-02 15:04:05 MST")
	}
	
	// Return as-is if parsing fails
	return BuildDate
}
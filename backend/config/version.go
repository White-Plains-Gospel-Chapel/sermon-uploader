package config

// Build-time variables (set via -ldflags)
var (
	Version   = "1.1.0"           // Default version
	BuildTime = "unknown"         // Set at build time
	GitCommit = "unknown"         // Set at build time
)

// GetVersion returns the full version string
func GetVersion() string {
	return Version
}

// GetFullVersion returns version with component suffix
func GetFullVersion(component string) string {
	return Version + "-" + component
}
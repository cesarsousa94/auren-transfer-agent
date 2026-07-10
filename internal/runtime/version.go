package runtime

import "fmt"

const (
	// AppName is the canonical executable name used by scripts and packaging.
	AppName = "auren-transfer-agent"

	// Version identifies this complete project delivery.
	Version = "v1.9.1"

	// Status describes the maturity of the current delivery.
	Status = "production/ready"
)

// VersionInfo is the public version payload exposed by the runtime package.
type VersionInfo struct {
	Name    string
	Version string
	Status  string
}

// Info returns immutable build metadata for the current binary.
func Info() VersionInfo {
	return VersionInfo{
		Name:    AppName,
		Version: Version,
		Status:  Status,
	}
}

// String renders version metadata for CLI output and diagnostics.
func (info VersionInfo) String() string {
	return fmt.Sprintf("%s %s (%s)", info.Name, info.Version, info.Status)
}

package adapter

import (
	"fmt"
	"os"
	"runtime"
	"strings"
)

// DetectPlatform determines the current platform and returns a platform identifier.
// Returns "darwin" on macOS, "ubuntu" on Ubuntu/Debian-family Linux, or
// ErrUnsupportedPlatform for all other platforms.
func DetectPlatform() (string, error) {
	return detectPlatformWith(runtime.GOOS, readOSRelease)
}

// detectPlatformWith is the testable core of DetectPlatform. It accepts the
// GOOS value and a function to read /etc/os-release contents.
func detectPlatformWith(goos string, osReleaseReader func() (string, error)) (string, error) {
	switch goos {
	case "darwin":
		return "darwin", nil
	case "linux":
		content, err := osReleaseReader()
		if err != nil {
			return "", fmt.Errorf("%w: cannot read /etc/os-release: %v", ErrUnsupportedPlatform, err)
		}
		if isDebianFamily(content) {
			return "ubuntu", nil
		}
		return "", fmt.Errorf("%w: unsupported Linux distribution", ErrUnsupportedPlatform)
	default:
		return "", fmt.Errorf("%w: %s", ErrUnsupportedPlatform, goos)
	}
}

// readOSRelease reads the contents of /etc/os-release.
func readOSRelease() (string, error) {
	data, err := os.ReadFile("/etc/os-release")
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// isDebianFamily checks whether the /etc/os-release content indicates a
// Debian-family distribution (ID=ubuntu, ID=debian, or ID_LIKE contains "debian").
func isDebianFamily(content string) bool {
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)

		if strings.HasPrefix(line, "ID=") {
			id := strings.Trim(strings.TrimPrefix(line, "ID="), "\"")
			if id == "ubuntu" || id == "debian" {
				return true
			}
		}

		if strings.HasPrefix(line, "ID_LIKE=") {
			idLike := strings.Trim(strings.TrimPrefix(line, "ID_LIKE="), "\"")
			if strings.Contains(idLike, "debian") {
				return true
			}
		}
	}
	return false
}

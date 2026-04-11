package adapter

import (
	"errors"
	"runtime"
	"testing"
)

func TestDetectPlatform_CurrentPlatform(t *testing.T) {
	platform, err := DetectPlatform()
	switch runtime.GOOS {
	case "darwin":
		if err != nil {
			t.Fatalf("expected no error on darwin, got %v", err)
		}
		if platform != "darwin" {
			t.Fatalf("expected darwin, got %s", platform)
		}
	case "linux":
		// On Linux, result depends on the distro. Just verify no panic.
		_ = err
		_ = platform
	default:
		if !errors.Is(err, ErrUnsupportedPlatform) {
			t.Fatalf("expected ErrUnsupportedPlatform on %s, got %v", runtime.GOOS, err)
		}
	}
}

func TestDetectPlatformWith_Darwin(t *testing.T) {
	platform, err := detectPlatformWith("darwin", nil)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if platform != "darwin" {
		t.Fatalf("expected darwin, got %s", platform)
	}
}

func TestDetectPlatformWith_Ubuntu(t *testing.T) {
	reader := func() (string, error) {
		return `NAME="Ubuntu"
VERSION="22.04 LTS (Jammy Jellyfish)"
ID=ubuntu
ID_LIKE=debian
PRETTY_NAME="Ubuntu 22.04 LTS"`, nil
	}
	platform, err := detectPlatformWith("linux", reader)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if platform != "ubuntu" {
		t.Fatalf("expected ubuntu, got %s", platform)
	}
}

func TestDetectPlatformWith_Debian(t *testing.T) {
	reader := func() (string, error) {
		return `NAME="Debian GNU/Linux"
ID=debian
PRETTY_NAME="Debian GNU/Linux 12 (bookworm)"`, nil
	}
	platform, err := detectPlatformWith("linux", reader)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if platform != "ubuntu" {
		t.Fatalf("expected ubuntu, got %s", platform)
	}
}

func TestDetectPlatformWith_DebianLike(t *testing.T) {
	reader := func() (string, error) {
		return `NAME="Linux Mint"
ID=linuxmint
ID_LIKE="ubuntu debian"
PRETTY_NAME="Linux Mint 21"`, nil
	}
	platform, err := detectPlatformWith("linux", reader)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if platform != "ubuntu" {
		t.Fatalf("expected ubuntu, got %s", platform)
	}
}

func TestDetectPlatformWith_UnsupportedLinux(t *testing.T) {
	reader := func() (string, error) {
		return `NAME="Fedora"
ID=fedora
PRETTY_NAME="Fedora 38"`, nil
	}
	_, err := detectPlatformWith("linux", reader)
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("expected ErrUnsupportedPlatform, got %v", err)
	}
}

func TestDetectPlatformWith_LinuxNoOSRelease(t *testing.T) {
	reader := func() (string, error) {
		return "", errors.New("file not found")
	}
	_, err := detectPlatformWith("linux", reader)
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("expected ErrUnsupportedPlatform, got %v", err)
	}
}

func TestDetectPlatformWith_Windows(t *testing.T) {
	_, err := detectPlatformWith("windows", nil)
	if !errors.Is(err, ErrUnsupportedPlatform) {
		t.Fatalf("expected ErrUnsupportedPlatform, got %v", err)
	}
}

func TestIsDebianFamily(t *testing.T) {
	tests := []struct {
		name    string
		content string
		want    bool
	}{
		{"ubuntu", "ID=ubuntu\n", true},
		{"debian", "ID=debian\n", true},
		{"debian-quoted", "ID=\"debian\"\n", true},
		{"id_like_debian", "ID=mint\nID_LIKE=debian\n", true},
		{"id_like_multi", "ID=pop\nID_LIKE=\"ubuntu debian\"\n", true},
		{"fedora", "ID=fedora\n", false},
		{"arch", "ID=arch\nID_LIKE=archlinux\n", false},
		{"empty", "", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := isDebianFamily(tt.content)
			if got != tt.want {
				t.Errorf("isDebianFamily(%q) = %v, want %v", tt.content, got, tt.want)
			}
		})
	}
}

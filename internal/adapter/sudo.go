package adapter

import (
	"fmt"
	"os"
	"os/exec"
	"sync"
	"syscall"
	"time"
)

// sudoRefreshInterval is the interval for the sudo keepalive loop.
// macOS defaults to 5-minute cache TTL; Linux defaults to 15 minutes.
// 50 seconds is well within both TTLs.
const sudoRefreshInterval = 50 * time.Second

// SudoManager manages privilege escalation and keepalive for operations
// requiring sudo.
type SudoManager struct {
	mu            sync.Mutex
	keepalivePID  int
	keepaliveProc *os.Process
	acquired      bool
}

// NewSudoManager creates a new SudoManager.
func NewSudoManager() *SudoManager {
	return &SudoManager{}
}

// AcquirePrivileges acquires sudo credentials and starts a keepalive background
// process. The operations parameter lists human-readable operation names for
// informational purposes.
func (m *SudoManager) AcquirePrivileges(operations []string) error {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.acquired {
		return nil
	}

	// Validate credentials with sudo -v.
	cmd := exec.Command("sudo", "-v")
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("%w: sudo -v failed: %v", ErrPrivilegeRequired, err)
	}

	// Start the keepalive process.
	keepalive := exec.Command("sh", "-c",
		fmt.Sprintf("while true; do sudo -n true; sleep %d; done", int(sudoRefreshInterval.Seconds())))
	keepalive.SysProcAttr = &syscall.SysProcAttr{
		Setpgid: true,
	}
	if err := keepalive.Start(); err != nil {
		return fmt.Errorf("failed to start sudo keepalive: %w", err)
	}

	m.keepalivePID = keepalive.Process.Pid
	m.keepaliveProc = keepalive.Process
	m.acquired = true

	return nil
}

// Release terminates the keepalive background process.
func (m *SudoManager) Release() {
	m.mu.Lock()
	defer m.mu.Unlock()

	if m.keepaliveProc != nil {
		// Kill the keepalive process and its children (process group).
		_ = syscall.Kill(-m.keepalivePID, syscall.SIGTERM)
		_, _ = m.keepaliveProc.Wait()
		m.keepaliveProc = nil
		m.keepalivePID = 0
	}
	m.acquired = false
}

// NeedsPrivilege returns true if the given operation requires privilege escalation.
func (m *SudoManager) NeedsPrivilege(op string) bool {
	privilegedOps := map[string]bool{
		"apt-install":       true,
		"apt-remove":        true,
		"apt-mark-hold":     true,
		"apt-mark-unhold":   true,
		"systemctl-enable":  true,
		"systemctl-disable": true,
		"systemctl-start":   true,
		"systemctl-stop":    true,
		"hostnamectl":       true,
		"scutil-set":        true,
	}
	return privilegedOps[op]
}

// IsAcquired returns whether privileges have been acquired.
func (m *SudoManager) IsAcquired() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.acquired
}

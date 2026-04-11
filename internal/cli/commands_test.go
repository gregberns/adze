package cli

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/gregberns/adze/internal/adapter"
	"github.com/gregberns/adze/internal/config"
	"github.com/spf13/cobra"
)

// mockAdapter is a test double for adapter.Adapter.
type mockAdapter struct {
	leaves        []string
	leavesErr     error
	allInstalled  []adapter.InstalledPackage
	allInstalledE error
	installed     map[string]bool // track installs
	removed       map[string]bool // track removals
	upgraded      map[string]bool // track upgrades
	installErr    error
	removeErr     error
	upgradeErr    error
	upgradeErrs   map[string]error // per-package upgrade errors
	defaults      map[string]map[string]adapter.DefaultsValue
	defaultsErr   error
}

func newMockAdapter() *mockAdapter {
	return &mockAdapter{
		installed:   make(map[string]bool),
		removed:     make(map[string]bool),
		upgraded:    make(map[string]bool),
		upgradeErrs: make(map[string]error),
		defaults:    make(map[string]map[string]adapter.DefaultsValue),
	}
}

func (m *mockAdapter) PackageInstall(pkg adapter.Package) error {
	if m.installErr != nil {
		return m.installErr
	}
	m.installed[pkg.Name] = true
	return nil
}

func (m *mockAdapter) PackageCheck(pkg adapter.Package) (bool, error) {
	return false, nil
}

func (m *mockAdapter) PackageList() ([]adapter.InstalledPackage, error) {
	return m.allInstalled, m.allInstalledE
}

func (m *mockAdapter) PackageUpgrade(pkg adapter.Package) error {
	if err, ok := m.upgradeErrs[pkg.Name]; ok {
		return err
	}
	if m.upgradeErr != nil {
		return m.upgradeErr
	}
	m.upgraded[pkg.Name] = true
	return nil
}

func (m *mockAdapter) PackageRemove(pkg adapter.Package) error {
	if m.removeErr != nil {
		return m.removeErr
	}
	m.removed[pkg.Name] = true
	return nil
}

func (m *mockAdapter) DefaultsRead(domain, key string) (adapter.DefaultsValue, error) {
	if m.defaultsErr != nil {
		return adapter.DefaultsValue{}, m.defaultsErr
	}
	domainMap, ok := m.defaults[domain]
	if !ok {
		return adapter.DefaultsValue{}, fmt.Errorf("domain %s not found", domain)
	}
	val, ok := domainMap[key]
	if !ok {
		return adapter.DefaultsValue{}, fmt.Errorf("key %s.%s not found", domain, key)
	}
	return val, nil
}

func (m *mockAdapter) DefaultsWrite(domain, key string, value adapter.DefaultsValue) error {
	return nil
}

func (m *mockAdapter) ServiceEnable(name string) error  { return nil }
func (m *mockAdapter) ServiceDisable(name string) error { return nil }
func (m *mockAdapter) SetHostname(hostname string) error { return nil }
func (m *mockAdapter) SetDefaultShell(shell string) error { return nil }

func (m *mockAdapter) ListLeaves() ([]string, error) {
	if m.leavesErr != nil {
		return nil, m.leavesErr
	}
	return m.leaves, nil
}

func (m *mockAdapter) ListAllInstalled() ([]adapter.InstalledPackage, error) {
	return m.allInstalled, m.allInstalledE
}

// writeTestConfig writes a config YAML file to the given directory and returns its path.
func writeTestConfig(t *testing.T, dir string, content string) string {
	t.Helper()
	path := filepath.Join(dir, "config.yaml")
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatal(err)
	}
	return path
}

// loadTestConfig is a helper that wraps loadConfigFromFile for tests.
func loadTestConfig(path string) (*config.Config, error) {
	return loadConfigFromFile(path)
}

// --- Status tests ---

func TestStatusInSync(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - curl
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "curl"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeStatus(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if !result.InSync {
		t.Error("expected InSync=true when all packages match")
	}
	if len(result.Drift) != 0 {
		t.Errorf("expected 0 drift entries, got %d", len(result.Drift))
	}
}

func TestStatusPackageOnMachineNotInConfig(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "lazygit", "htop"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeStatus(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if result.InSync {
		t.Error("expected InSync=false when drift exists")
	}

	// Should have 2 "+" entries.
	added := 0
	for _, d := range result.Drift {
		if d.Kind == DriftAdded {
			added++
			if d.Name != "htop" && d.Name != "lazygit" {
				t.Errorf("unexpected added package: %s", d.Name)
			}
		}
	}
	if added != 2 {
		t.Errorf("expected 2 added entries, got %d", added)
	}
}

func TestStatusPackageInConfigNotOnMachine(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - hugo
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeStatus(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if result.InSync {
		t.Error("expected InSync=false")
	}

	missing := 0
	for _, d := range result.Drift {
		if d.Kind == DriftMissing && d.Category == "packages" {
			missing++
			if d.Name != "hugo" {
				t.Errorf("unexpected missing package: %s", d.Name)
			}
		}
	}
	if missing != 1 {
		t.Errorf("expected 1 missing entry, got %d", missing)
	}
}

func TestStatusDefaultsDrift(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew: []
defaults:
  com.apple.dock:
    tilesize: 36
`)

	mock := newMockAdapter()
	mock.leaves = []string{}
	mock.defaults["com.apple.dock"] = map[string]adapter.DefaultsValue{
		"tilesize": {Type: "integer", Raw: "48"},
	}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeStatus(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if result.InSync {
		t.Error("expected InSync=false for defaults drift")
	}

	found := false
	for _, d := range result.Drift {
		if d.Kind == DriftChanged && d.Category == "defaults" {
			found = true
			if d.Config != "36" || d.Actual != "48" {
				t.Errorf("expected config=36 actual=48, got config=%s actual=%s", d.Config, d.Actual)
			}
		}
	}
	if !found {
		t.Error("expected a defaults drift entry with kind '~'")
	}
}

func TestStatusExitCode7OnDrift(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "extra"}

	deps := &statusDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	// Replace status command with our injected deps.
	for i, cmd := range root.Commands() {
		if cmd.Use == "status" {
			root.RemoveCommand(cmd)
			statusCmd := &cobra.Command{
				Use:  "status",
				RunE: runStatus(deps),
			}
			root.AddCommand(statusCmd)
			_ = i
			break
		}
	}

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"status", "--config", cfgPath})

	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for drift")
	}

	exitErr, ok := err.(*cmdExitError)
	if !ok {
		t.Fatalf("expected *cmdExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitDriftDetected {
		t.Errorf("expected exit code %d, got %d", ExitDriftDetected, exitErr.Code)
	}
}

func TestStatusJSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "extra"}

	deps := &statusDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "status" {
			root.RemoveCommand(cmd)
			break
		}
	}
	statusCmd := &cobra.Command{
		Use:  "status",
		RunE: runStatus(deps),
	}
	root.AddCommand(statusCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"status", "--config", cfgPath, "--json"})
	root.Execute()

	var result StatusResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON output: %v\nraw: %s", err, buf.String())
	}

	if result.InSync {
		t.Error("expected InSync=false in JSON")
	}
	if len(result.Drift) != 1 {
		t.Errorf("expected 1 drift entry, got %d", len(result.Drift))
	}
}

func TestStatusCaskPackages(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  cask:
    - firefox
  brew:
    - git
`)

	mock := newMockAdapter()
	// ListLeaves returns both brew leaves and casks.
	mock.leaves = []string{"git", "firefox"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeStatus(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if !result.InSync {
		t.Errorf("expected InSync=true, drift: %+v", result.Drift)
	}
}

// --- Capture tests ---

func TestCaptureFindsDrift(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "lazygit", "htop"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeCapture(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Extras) != 2 {
		t.Errorf("expected 2 extras, got %d: %v", len(result.Extras), result.Extras)
	}

	// Should be sorted.
	if result.Extras[0] != "htop" || result.Extras[1] != "lazygit" {
		t.Errorf("expected [htop lazygit], got %v", result.Extras)
	}
}

func TestCaptureNoDrift(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - curl
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "curl"}

	cfg, err := loadTestConfig(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	result, err := computeCapture(cfg, cfgPath, mock)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Extras) != 0 {
		t.Errorf("expected 0 extras, got %d", len(result.Extras))
	}
}

func TestCaptureWriteAll(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "lazygit"}

	deps := &captureDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "capture" {
			root.RemoveCommand(cmd)
			break
		}
	}
	captureCmd := &cobra.Command{
		Use:  "capture",
		RunE: runCapture(deps),
	}
	captureCmd.Flags().Bool("all", false, "write all")
	root.AddCommand(captureCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capture", "--config", cfgPath, "--all"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("capture --all failed: %v", err)
	}

	// Read the config back and verify lazygit was added.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}

	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatalf("parse error: %v", parseErr)
	}

	found := false
	for _, p := range cfg.Packages.Brew {
		if p.Name == "lazygit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected lazygit to be added to config after capture --all")
	}
}

// --- Install tests ---

func TestInstallSuccess(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()

	deps := &installDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "install <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	installCmd := &cobra.Command{
		Use:  "install <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runInstall(deps),
	}
	installCmd.Flags().Bool("cask", false, "cask")
	root.AddCommand(installCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"install", "lazygit", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("install failed: %v", err)
	}

	// Verify adapter was called.
	if !mock.installed["lazygit"] {
		t.Error("expected adapter.PackageInstall to be called with lazygit")
	}

	// Verify config was updated.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}

	found := false
	for _, p := range cfg.Packages.Brew {
		if p.Name == "lazygit" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected lazygit in config after install")
	}
}

func TestInstallCask(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  cask: []
  brew:
    - git
`)

	mock := newMockAdapter()

	deps := &installDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "install <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	installCmd := &cobra.Command{
		Use:  "install <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runInstall(deps),
	}
	installCmd.Flags().Bool("cask", false, "cask")
	root.AddCommand(installCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"install", "firefox", "--cask", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("install --cask failed: %v", err)
	}

	// Verify config was updated in cask list.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}

	found := false
	for _, p := range cfg.Packages.Cask {
		if p.Name == "firefox" {
			found = true
			break
		}
	}
	if !found {
		t.Error("expected firefox in cask list after install --cask")
	}
}

func TestInstallFailure(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.installErr = fmt.Errorf("brew install failed")

	deps := &installDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "install <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	installCmd := &cobra.Command{
		Use:  "install <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runInstall(deps),
	}
	installCmd.Flags().Bool("cask", false, "cask")
	root.AddCommand(installCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"install", "badpkg", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for install failure")
	}

	exitErr, ok := err.(*cmdExitError)
	if !ok {
		t.Fatalf("expected *cmdExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitExecFailure {
		t.Errorf("expected exit code %d, got %d", ExitExecFailure, exitErr.Code)
	}
}

func TestInstallDuplicateNoOp(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()

	deps := &installDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "install <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	installCmd := &cobra.Command{
		Use:  "install <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runInstall(deps),
	}
	installCmd.Flags().Bool("cask", false, "cask")
	root.AddCommand(installCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	// Install git which is already in config.
	root.SetArgs([]string{"install", "git", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("install of existing package should succeed: %v", err)
	}

	// Verify config still has exactly one git.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}

	count := 0
	for _, p := range cfg.Packages.Brew {
		if p.Name == "git" {
			count++
		}
	}
	if count != 1 {
		t.Errorf("expected 1 git entry, got %d", count)
	}
}

// --- Remove tests ---

func TestRemoveSuccess(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - lazygit
`)

	mock := newMockAdapter()

	deps := &removeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "remove <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	removeCmd := &cobra.Command{
		Use:  "remove <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runRemove(deps),
	}
	root.AddCommand(removeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"remove", "lazygit", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	if !mock.removed["lazygit"] {
		t.Error("expected adapter.PackageRemove to be called with lazygit")
	}

	// Verify config was updated.
	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}

	for _, p := range cfg.Packages.Brew {
		if p.Name == "lazygit" {
			t.Error("lazygit should have been removed from config")
		}
	}

	// git should still be there.
	found := false
	for _, p := range cfg.Packages.Brew {
		if p.Name == "git" {
			found = true
			break
		}
	}
	if !found {
		t.Error("git should still be in config after removing lazygit")
	}
}

func TestRemoveFailure(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.removeErr = fmt.Errorf("brew uninstall failed")

	deps := &removeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "remove <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	removeCmd := &cobra.Command{
		Use:  "remove <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runRemove(deps),
	}
	root.AddCommand(removeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"remove", "git", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for remove failure")
	}

	exitErr, ok := err.(*cmdExitError)
	if !ok {
		t.Fatalf("expected *cmdExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitExecFailure {
		t.Errorf("expected exit code %d, got %d", ExitExecFailure, exitErr.Code)
	}
}

func TestRemoveFromCaskList(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
  cask:
    - firefox
`)

	mock := newMockAdapter()

	deps := &removeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "remove <pkg>" {
			root.RemoveCommand(cmd)
			break
		}
	}
	removeCmd := &cobra.Command{
		Use:  "remove <pkg>",
		Args: cobra.ExactArgs(1),
		RunE: runRemove(deps),
	}
	root.AddCommand(removeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"remove", "firefox", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("remove failed: %v", err)
	}

	data, err := os.ReadFile(cfgPath)
	if err != nil {
		t.Fatal(err)
	}
	cfg, _, _, parseErr := config.Parse(data)
	if parseErr != nil {
		t.Fatal(parseErr)
	}

	for _, p := range cfg.Packages.Cask {
		if p.Name == "firefox" {
			t.Error("firefox should have been removed from cask list")
		}
	}
}

// --- Upgrade tests ---

func TestUpgradeSkipsPinned(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - name: git
      pinned: true
    - curl
`)

	mock := newMockAdapter()

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	if mock.upgraded["git"] {
		t.Error("pinned package git should NOT have been upgraded")
	}
	if !mock.upgraded["curl"] {
		t.Error("non-pinned package curl should have been upgraded")
	}
}

func TestUpgradeSkipsCasksWithoutAll(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - curl
  cask:
    - firefox
`)

	mock := newMockAdapter()

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	if mock.upgraded["firefox"] {
		t.Error("cask firefox should NOT be upgraded without --all")
	}
	if !mock.upgraded["curl"] {
		t.Error("brew curl should have been upgraded")
	}
}

func TestUpgradeAllIncludesCasks(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - curl
  cask:
    - firefox
`)

	mock := newMockAdapter()

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade", "--all", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("upgrade --all failed: %v", err)
	}

	if !mock.upgraded["firefox"] {
		t.Error("cask firefox should be upgraded with --all")
	}
	if !mock.upgraded["curl"] {
		t.Error("brew curl should have been upgraded")
	}
}

func TestUpgradePartialFailure(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - curl
`)

	mock := newMockAdapter()
	mock.upgradeErrs["git"] = fmt.Errorf("upgrade failed")

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error for partial failure")
	}

	exitErr, ok := err.(*cmdExitError)
	if !ok {
		t.Fatalf("expected *cmdExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitPartialSuccess {
		t.Errorf("expected exit code %d, got %d", ExitPartialSuccess, exitErr.Code)
	}
}

func TestUpgradeAllFailed(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
    - curl
`)

	mock := newMockAdapter()
	mock.upgradeErr = fmt.Errorf("upgrade failed")

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	errBuf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetErr(errBuf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath})
	err := root.Execute()
	if err == nil {
		t.Fatal("expected error when all upgrades fail")
	}

	exitErr, ok := err.(*cmdExitError)
	if !ok {
		t.Fatalf("expected *cmdExitError, got %T: %v", err, err)
	}
	if exitErr.Code != ExitExecFailure {
		t.Errorf("expected exit code %d, got %d", ExitExecFailure, exitErr.Code)
	}
}

func TestUpgradeVersionConstraint(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - name: python
      version: "3.11"
`)

	mock := newMockAdapter()

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("upgrade failed: %v", err)
	}

	// The version-constrained package should still be upgraded (adapter handles constraints).
	if !mock.upgraded["python"] {
		t.Error("version-constrained package should be upgraded (adapter handles version)")
	}
}

func TestUpgradeJSONOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - name: git
      pinned: true
    - curl
`)

	mock := newMockAdapter()

	deps := &upgradeDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "upgrade" {
			root.RemoveCommand(cmd)
			break
		}
	}
	upgradeCmd := &cobra.Command{
		Use:  "upgrade",
		RunE: runUpgrade(deps),
	}
	upgradeCmd.Flags().Bool("all", false, "include casks")
	root.AddCommand(upgradeCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"upgrade", "--config", cfgPath, "--json"})
	err := root.Execute()
	if err != nil {
		t.Fatalf("upgrade --json failed: %v", err)
	}

	var result UpgradeResult
	if err := json.Unmarshal(buf.Bytes(), &result); err != nil {
		t.Fatalf("failed to parse JSON: %v\nraw: %s", err, buf.String())
	}

	if len(result.Upgraded) != 1 || result.Upgraded[0] != "curl" {
		t.Errorf("expected [curl] in upgraded, got %v", result.Upgraded)
	}
	if len(result.Skipped) != 1 || result.Skipped[0] != "git" {
		t.Errorf("expected [git] in skipped, got %v", result.Skipped)
	}
}

// --- Human output tests ---

func TestStatusHumanOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "lazygit"}

	deps := &statusDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "status" {
			root.RemoveCommand(cmd)
			break
		}
	}
	statusCmd := &cobra.Command{
		Use:  "status",
		RunE: runStatus(deps),
	}
	root.AddCommand(statusCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"status", "--config", cfgPath})
	root.Execute()

	output := buf.String()
	if !strings.Contains(output, "Comparing") {
		t.Error("expected 'Comparing' in human output")
	}
	if !strings.Contains(output, "+ lazygit") {
		t.Errorf("expected '+ lazygit' in output, got: %s", output)
	}
	if !strings.Contains(output, "Everything else:") {
		t.Errorf("expected 'Everything else:' in output, got: %s", output)
	}
}

func TestCaptureHumanOutput(t *testing.T) {
	dir := t.TempDir()
	cfgPath := writeTestConfig(t, dir, `
name: test
platform: darwin
packages:
  brew:
    - git
`)

	mock := newMockAdapter()
	mock.leaves = []string{"git", "lazygit"}

	deps := &captureDeps{
		adapter:    mock,
		loadConfig: loadTestConfig,
	}

	root := NewRootCmd("dev", "none", "unknown")
	for _, cmd := range root.Commands() {
		if cmd.Use == "capture" {
			root.RemoveCommand(cmd)
			break
		}
	}
	captureCmd := &cobra.Command{
		Use:  "capture",
		RunE: runCapture(deps),
	}
	captureCmd.Flags().Bool("all", false, "write all")
	root.AddCommand(captureCmd)

	buf := new(bytes.Buffer)
	root.SetOut(buf)
	root.SetArgs([]string{"capture", "--config", cfgPath})
	err := root.Execute()
	if err != nil {
		t.Fatalf("capture failed: %v", err)
	}

	output := buf.String()
	if !strings.Contains(output, "+ lazygit") {
		t.Errorf("expected '+ lazygit' in output, got: %s", output)
	}
	if !strings.Contains(output, "--all") {
		t.Errorf("expected '--all' suggestion in output, got: %s", output)
	}
}


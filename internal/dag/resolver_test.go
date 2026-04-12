package dag

import (
	"strings"
	"testing"
)

func TestLinearChain(t *testing.T) {
	// A → B → C (A provides x, B requires x and provides y, C requires y)
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}},
		{Name: "step-b", Provides: []string{"y"}, Requires: []string{"x"}},
		{Name: "step-c", Requires: []string{"y"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(g.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(g.Steps))
	}

	assertOrder(t, g, "step-a", "step-b")
	assertOrder(t, g, "step-b", "step-c")
}

func TestDiamondDependency(t *testing.T) {
	// A provides X, B provides Y, C requires X+Y
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}},
		{Name: "step-b", Provides: []string{"y"}},
		{Name: "step-c", Requires: []string{"x", "y"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(g.Steps) != 3 {
		t.Fatalf("expected 3 steps, got %d", len(g.Steps))
	}

	assertOrder(t, g, "step-a", "step-c")
	assertOrder(t, g, "step-b", "step-c")

	// C should have both deps in DependsOn.
	cStep := findStep(g, "step-c")
	if cStep == nil {
		t.Fatal("step-c not found")
	}
	if cStep.DependsOn["x"] != "step-a" {
		t.Errorf("step-c DependsOn[x] = %q, want %q", cStep.DependsOn["x"], "step-a")
	}
	if cStep.DependsOn["y"] != "step-b" {
		t.Errorf("step-c DependsOn[y] = %q, want %q", cStep.DependsOn["y"], "step-b")
	}
}

func TestCycleDetection(t *testing.T) {
	// A requires y (provided by B), B requires x (provided by A) — cycle.
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}, Requires: []string{"y"}},
		{Name: "step-b", Provides: []string{"y"}, Requires: []string{"x"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected cycle error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, "dependency cycle detected") {
		t.Errorf("expected cycle error message, got: %s", errStr)
	}
	if !strings.Contains(errStr, "Cycle:") {
		t.Errorf("expected Cycle: path in error, got: %s", errStr)
	}
}

func TestDuplicateProvides(t *testing.T) {
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}, BuiltIn: true},
		{Name: "step-b", Provides: []string{"x"}, BuiltIn: false},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected duplicate provides error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, `duplicate provider for "x"`) {
		t.Errorf("expected duplicate provider error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "step-a (built-in)") {
		t.Errorf("expected step-a (built-in) in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "step-b (custom)") {
		t.Errorf("expected step-b (custom) in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "Fix: remove one of these steps or rename the capability") {
		t.Errorf("expected fix suggestion in error, got: %s", errStr)
	}
}

func TestUnresolvedRequiresWithBuiltInSuggestion(t *testing.T) {
	steps := []StepInput{
		{Name: "step-a", Requires: []string{"homebrew"}},
	}
	known := KnownBuiltIn{"homebrew": "brew-install"}

	g, errs := Resolve(steps, "darwin", known)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected unresolved dependency error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, `unresolved dependency "homebrew"`) {
		t.Errorf("expected unresolved dependency error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "Required by: step-a") {
		t.Errorf("expected 'Required by: step-a' in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, `Suggestion: add the built-in step "brew-install" which provides "homebrew"`) {
		t.Errorf("expected suggestion in error, got: %s", errStr)
	}
}

func TestUnresolvedRequiresWithoutBuiltIn(t *testing.T) {
	steps := []StepInput{
		{Name: "step-a", Requires: []string{"something-unknown"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected unresolved dependency error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, `unresolved dependency "something-unknown"`) {
		t.Errorf("expected unresolved dependency error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "No built-in step provides this capability") {
		t.Errorf("expected 'No built-in step provides this capability' in error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "provides: [something-unknown]") {
		t.Errorf("expected provides suggestion in error, got: %s", errStr)
	}
}

func TestPlatformFiltering(t *testing.T) {
	// step-a is darwin-only, step-b is ubuntu-only; resolve on darwin.
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}, Platforms: []string{"darwin"}},
		{Name: "step-b", Provides: []string{"y"}, Platforms: []string{"ubuntu"}},
		{Name: "step-c", Requires: []string{"x"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	// step-b should be filtered out.
	if len(g.Steps) != 2 {
		t.Fatalf("expected 2 steps, got %d: %v", len(g.Steps), stepNames(g))
	}

	names := stepNames(g)
	if contains(names, "step-b") {
		t.Error("step-b should have been filtered out by platform")
	}
}

func TestPlatformFilteredBreakage(t *testing.T) {
	// step-a is linux-only but step-b (darwin) requires capability from step-a.
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}, Platforms: []string{"linux"}},
		{Name: "step-b", Requires: []string{"x"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected platform-filtered breakage error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, `step "step-a" is required by "step-b" but does not support platform "darwin"`) {
		t.Errorf("expected platform-filtered breakage error, got: %s", errStr)
	}
	if !strings.Contains(errStr, `"step-a" only supports: linux`) {
		t.Errorf("expected platform list in error, got: %s", errStr)
	}
}

func TestAlphabeticalOrdering(t *testing.T) {
	// Multiple independent steps — should be sorted alphabetically.
	steps := []StepInput{
		{Name: "delta"},
		{Name: "alpha"},
		{Name: "charlie"},
		{Name: "bravo"},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(g.Steps) != 4 {
		t.Fatalf("expected 4 steps, got %d", len(g.Steps))
	}

	expected := []string{"alpha", "bravo", "charlie", "delta"}
	for i, s := range g.Steps {
		if s.Name != expected[i] {
			t.Errorf("step[%d] = %q, want %q", i, s.Name, expected[i])
		}
	}
}

func TestEmptyInput(t *testing.T) {
	g, errs := Resolve(nil, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if g == nil {
		t.Fatal("expected non-nil graph for empty input")
	}
	if len(g.Steps) != 0 {
		t.Errorf("expected 0 steps, got %d", len(g.Steps))
	}
}

func TestStepWithNoProvidesNoRequires(t *testing.T) {
	steps := []StepInput{
		{Name: "standalone"},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(g.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(g.Steps))
	}
	if g.Steps[0].Name != "standalone" {
		t.Errorf("expected 'standalone', got %q", g.Steps[0].Name)
	}
	if g.Steps[0].Depth != 0 {
		t.Errorf("expected depth 0, got %d", g.Steps[0].Depth)
	}
}

func TestMultipleErrorsCollected(t *testing.T) {
	// Both duplicate provides AND unresolved requires in one call.
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"x"}},
		{Name: "step-b", Provides: []string{"x"}},
		{Name: "step-c", Requires: []string{"missing"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) < 2 {
		t.Fatalf("expected at least 2 errors, got %d: %v", len(errs), errs)
	}

	// Check that we got both types of errors.
	var hasDuplicate, hasUnresolved bool
	for _, e := range errs {
		s := e.Error()
		if strings.Contains(s, "duplicate provider") {
			hasDuplicate = true
		}
		if strings.Contains(s, "unresolved dependency") {
			hasUnresolved = true
		}
	}
	if !hasDuplicate {
		t.Error("expected duplicate provider error")
	}
	if !hasUnresolved {
		t.Error("expected unresolved dependency error")
	}
}

func TestDepthCalculation(t *testing.T) {
	// root=0, child=1, grandchild=2
	steps := []StepInput{
		{Name: "root", Provides: []string{"x"}},
		{Name: "child", Provides: []string{"y"}, Requires: []string{"x"}},
		{Name: "grandchild", Requires: []string{"y"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	root := findStep(g, "root")
	child := findStep(g, "child")
	grandchild := findStep(g, "grandchild")

	if root == nil || child == nil || grandchild == nil {
		t.Fatal("missing steps in result")
	}
	if root.Depth != 0 {
		t.Errorf("root depth = %d, want 0", root.Depth)
	}
	if child.Depth != 1 {
		t.Errorf("child depth = %d, want 1", child.Depth)
	}
	if grandchild.Depth != 2 {
		t.Errorf("grandchild depth = %d, want 2", grandchild.Depth)
	}
}

func TestDependsOnAndRequiredByMaps(t *testing.T) {
	steps := []StepInput{
		{Name: "provider", Provides: []string{"cap-a", "cap-b"}},
		{Name: "consumer", Requires: []string{"cap-a"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	provider := findStep(g, "provider")
	consumer := findStep(g, "consumer")
	if provider == nil || consumer == nil {
		t.Fatal("missing steps")
	}

	// Consumer's DependsOn should map cap-a → provider.
	if consumer.DependsOn["cap-a"] != "provider" {
		t.Errorf("consumer DependsOn[cap-a] = %q, want %q", consumer.DependsOn["cap-a"], "provider")
	}

	// Provider's RequiredBy should map cap-a → consumer.
	if provider.RequiredBy["cap-a"] != "consumer" {
		t.Errorf("provider RequiredBy[cap-a] = %q, want %q", provider.RequiredBy["cap-a"], "consumer")
	}
}

func TestBuiltInFlag(t *testing.T) {
	steps := []StepInput{
		{Name: "built-in-step", Provides: []string{"x"}, BuiltIn: true},
		{Name: "custom-step", Requires: []string{"x"}, BuiltIn: false},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	bis := findStep(g, "built-in-step")
	cs := findStep(g, "custom-step")
	if bis == nil || cs == nil {
		t.Fatal("missing steps")
	}
	if !bis.BuiltIn {
		t.Error("built-in-step should have BuiltIn=true")
	}
	if cs.BuiltIn {
		t.Error("custom-step should have BuiltIn=false")
	}
}

func TestPlatformEmptyMatchesAll(t *testing.T) {
	// Step with no platforms should match any platform.
	steps := []StepInput{
		{Name: "any-platform", Provides: []string{"x"}},
	}

	g, errs := Resolve(steps, "some-exotic-os", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}
	if len(g.Steps) != 1 {
		t.Fatalf("expected 1 step, got %d", len(g.Steps))
	}
}

func TestDeterminismMultipleRuns(t *testing.T) {
	// Run the same resolve multiple times, order must be identical.
	steps := []StepInput{
		{Name: "zulu"},
		{Name: "alpha"},
		{Name: "mike"},
		{Name: "echo"},
	}

	var firstOrder []string
	for i := 0; i < 10; i++ {
		g, errs := Resolve(steps, "darwin", nil)
		if len(errs) > 0 {
			t.Fatalf("unexpected errors on run %d: %v", i, errs)
		}
		names := stepNames(g)
		if i == 0 {
			firstOrder = names
		} else {
			for j, name := range names {
				if name != firstOrder[j] {
					t.Fatalf("non-deterministic order on run %d: got %v, expected %v", i, names, firstOrder)
				}
			}
		}
	}
}

func TestCycleDetectionThreeNodes(t *testing.T) {
	// A → B → C → A (three-node cycle)
	steps := []StepInput{
		{Name: "step-a", Provides: []string{"a-cap"}, Requires: []string{"c-cap"}},
		{Name: "step-b", Provides: []string{"b-cap"}, Requires: []string{"a-cap"}},
		{Name: "step-c", Provides: []string{"c-cap"}, Requires: []string{"b-cap"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if g != nil {
		t.Fatalf("expected nil graph, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected cycle error, got none")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, "dependency cycle detected") {
		t.Errorf("expected cycle error, got: %s", errStr)
	}
	if !strings.Contains(errStr, "Cycle:") {
		t.Errorf("expected Cycle: path, got: %s", errStr)
	}
}

func TestDiamondWithDepths(t *testing.T) {
	//     A (depth 0)
	//    / \
	//   B   C (depth 1)
	//    \ /
	//     D (depth 2)
	steps := []StepInput{
		{Name: "A", Provides: []string{"a1", "a2"}},
		{Name: "B", Provides: []string{"b1"}, Requires: []string{"a1"}},
		{Name: "C", Provides: []string{"c1"}, Requires: []string{"a2"}},
		{Name: "D", Requires: []string{"b1", "c1"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	a := findStep(g, "A")
	b := findStep(g, "B")
	c := findStep(g, "C")
	d := findStep(g, "D")

	if a.Depth != 0 {
		t.Errorf("A depth = %d, want 0", a.Depth)
	}
	if b.Depth != 1 {
		t.Errorf("B depth = %d, want 1", b.Depth)
	}
	if c.Depth != 1 {
		t.Errorf("C depth = %d, want 1", c.Depth)
	}
	if d.Depth != 2 {
		t.Errorf("D depth = %d, want 2", d.Depth)
	}

	// D must come after both B and C.
	assertOrder(t, g, "B", "D")
	assertOrder(t, g, "C", "D")
}

// --- helpers ---

func findStep(g *ResolvedGraph, name string) *ResolvedStep {
	for i := range g.Steps {
		if g.Steps[i].Name == name {
			return &g.Steps[i]
		}
	}
	return nil
}

func stepNames(g *ResolvedGraph) []string {
	names := make([]string, len(g.Steps))
	for i, s := range g.Steps {
		names[i] = s.Name
	}
	return names
}

func assertOrder(t *testing.T, g *ResolvedGraph, before, after string) {
	t.Helper()
	bi, ai := -1, -1
	for i, s := range g.Steps {
		if s.Name == before {
			bi = i
		}
		if s.Name == after {
			ai = i
		}
	}
	if bi == -1 {
		t.Errorf("step %q not found in graph", before)
		return
	}
	if ai == -1 {
		t.Errorf("step %q not found in graph", after)
		return
	}
	if bi >= ai {
		t.Errorf("expected %q (index %d) before %q (index %d)", before, bi, after, ai)
	}
}

func contains(ss []string, s string) bool {
	for _, v := range ss {
		if v == s {
			return true
		}
	}
	return false
}

// TestCrossPlatformRequires verifies that steps with platform-conditional
// requires resolve correctly on both darwin and ubuntu. This is the pattern
// used by node-fnm, python, go steps which require homebrew on darwin
// but apt-essentials on ubuntu.
func TestCrossPlatformRequires_Darwin(t *testing.T) {
	steps := []StepInput{
		{Name: "xcode-cli-tools", Provides: []string{"xcode-cli-tools"}, Platforms: []string{"darwin"}},
		{Name: "homebrew", Provides: []string{"homebrew"}, Requires: []string{"xcode-cli-tools"}, Platforms: []string{"darwin"}},
		{Name: "apt-essentials", Provides: []string{"apt-essentials"}, Platforms: []string{"ubuntu"}},
		// On darwin, node-fnm requires homebrew
		{Name: "node-fnm", Provides: []string{"node", "fnm"}, Requires: []string{"homebrew"}, Platforms: []string{"darwin", "ubuntu"}},
	}

	g, errs := Resolve(steps, "darwin", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors on darwin: %v", errs)
	}

	// Should have xcode-cli-tools, homebrew, node-fnm (apt-essentials filtered out)
	if len(g.Steps) != 3 {
		t.Fatalf("expected 3 steps on darwin, got %d: %v", len(g.Steps), stepNames(g))
	}

	assertOrder(t, g, "xcode-cli-tools", "homebrew")
	assertOrder(t, g, "homebrew", "node-fnm")

	nodeFnm := findStep(g, "node-fnm")
	if nodeFnm == nil {
		t.Fatal("node-fnm not found")
	}
	if nodeFnm.DependsOn["homebrew"] != "homebrew" {
		t.Errorf("node-fnm DependsOn[homebrew] = %q, want %q", nodeFnm.DependsOn["homebrew"], "homebrew")
	}
}

func TestCrossPlatformRequires_Ubuntu(t *testing.T) {
	steps := []StepInput{
		{Name: "xcode-cli-tools", Provides: []string{"xcode-cli-tools"}, Platforms: []string{"darwin"}},
		{Name: "homebrew", Provides: []string{"homebrew"}, Requires: []string{"xcode-cli-tools"}, Platforms: []string{"darwin"}},
		{Name: "apt-essentials", Provides: []string{"apt-essentials"}, Platforms: []string{"ubuntu"}},
		// On ubuntu, node-fnm requires apt-essentials
		{Name: "node-fnm", Provides: []string{"node", "fnm"}, Requires: []string{"apt-essentials"}, Platforms: []string{"darwin", "ubuntu"}},
	}

	g, errs := Resolve(steps, "ubuntu", nil)
	if len(errs) > 0 {
		t.Fatalf("unexpected errors on ubuntu: %v", errs)
	}

	// Should have apt-essentials, node-fnm (xcode-cli-tools and homebrew filtered out)
	if len(g.Steps) != 2 {
		t.Fatalf("expected 2 steps on ubuntu, got %d: %v", len(g.Steps), stepNames(g))
	}

	assertOrder(t, g, "apt-essentials", "node-fnm")

	nodeFnm := findStep(g, "node-fnm")
	if nodeFnm == nil {
		t.Fatal("node-fnm not found")
	}
	if nodeFnm.DependsOn["apt-essentials"] != "apt-essentials" {
		t.Errorf("node-fnm DependsOn[apt-essentials] = %q, want %q", nodeFnm.DependsOn["apt-essentials"], "apt-essentials")
	}
}

// TestCrossPlatformRequires_UbuntuFailsWithHomebrew verifies that if a step
// requires "homebrew" on ubuntu (the bug scenario), DAG resolution fails.
func TestCrossPlatformRequires_UbuntuFailsWithHomebrew(t *testing.T) {
	steps := []StepInput{
		{Name: "homebrew", Provides: []string{"homebrew"}, Platforms: []string{"darwin"}},
		{Name: "apt-essentials", Provides: []string{"apt-essentials"}, Platforms: []string{"ubuntu"}},
		// Bug: node-fnm requires homebrew even on ubuntu
		{Name: "node-fnm", Provides: []string{"node"}, Requires: []string{"homebrew"}, Platforms: []string{"darwin", "ubuntu"}},
	}

	g, errs := Resolve(steps, "ubuntu", nil)
	if g != nil {
		t.Fatalf("expected nil graph on ubuntu with homebrew requirement, got %+v", g)
	}
	if len(errs) == 0 {
		t.Fatal("expected error for unresolvable homebrew dependency on ubuntu")
	}

	errStr := errs[0].Error()
	if !strings.Contains(errStr, "homebrew") {
		t.Errorf("expected error mentioning homebrew, got: %s", errStr)
	}
}

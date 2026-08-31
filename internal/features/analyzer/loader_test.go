package analyzer

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadPackagesDeterministicTargetAwareSnapshot(t *testing.T) {
	goModPath := filepath.Join(contractFixtureRoot(), "go.mod")
	before, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	config := LoadConfig{ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetShared}
	first, err := LoadPackages(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	second, err := LoadPackages(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(first.Packages, second.Packages) {
		t.Fatalf("package snapshots differ:\n%+v\n%+v", first.Packages, second.Packages)
	}
	after, err := os.ReadFile(goModPath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("package loading mutated the analyzed module")
	}
	if len(first.Packages) != 1 || first.Packages[0].Path != "example.com/analyzer-contracts/compliant" {
		t.Fatalf("packages = %+v", first.Packages)
	}
	for _, file := range first.Packages[0].Files {
		if !hasRole(file.Roles, RoleShared) {
			t.Errorf("file %q has no shared role: %v", file.Path, file.Roles)
		}
		if strings.HasSuffix(file.Path, "generated.go") && !hasRole(file.Roles, RoleGenerated) {
			t.Errorf("generated file roles = %v", file.Roles)
		}
	}
}

func TestLoadPackagesHonorsBuildTagsAndTests(t *testing.T) {
	snapshot, err := LoadPackages(context.Background(), LoadConfig{
		ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, BuildTags: []string{"analyzer_extra"}, Tests: true, Target: TargetDevice,
	})
	if err != nil {
		t.Fatal(err)
	}
	var tagged, test bool
	for _, pkg := range snapshot.Packages {
		for _, file := range pkg.Files {
			tagged = tagged || strings.HasSuffix(file.Path, "tagged.go")
			test = test || strings.HasSuffix(file.Path, "game_test.go") && hasRole(file.Roles, RoleTest)
			if !hasRole(file.Roles, RoleDevice) {
				t.Errorf("file %q has no device role: %v", file.Path, file.Roles)
			}
		}
	}
	if !tagged || !test {
		t.Fatalf("tagged=%v test=%v packages=%+v", tagged, test, snapshot.Packages)
	}
}

func TestLoadPackagesReturnsPartialPackageForOverlaySyntaxError(t *testing.T) {
	snapshot, err := LoadPackages(context.Background(), LoadConfig{
		ModuleRoot: contractFixtureRoot(), Patterns: []string{"./compliant"}, Target: TargetSimulator,
		Overlay: map[string][]byte{"compliant/game.go": []byte("package compliant\nfunc broken(")},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.Packages) != 1 || !snapshot.Packages[0].Partial || len(snapshot.Packages[0].Errors) == 0 {
		t.Fatalf("partial snapshot = %+v", snapshot.Packages)
	}
	if position := snapshot.Packages[0].Errors[0].Position; strings.Contains(position, "\\") || strings.Contains(strings.ToLower(position), strings.ToLower(snapshot.ModuleRoot)) {
		t.Fatalf("error position is not module-relative: %q", position)
	}
}

func TestLoadPackagesStopsForCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, err := LoadPackages(ctx, LoadConfig{ModuleRoot: contractFixtureRoot(), Target: TargetShared})
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("LoadPackages() = %v, want context cancellation", err)
	}
}

func TestLoadPackagesValidatesBoundary(t *testing.T) {
	for name, config := range map[string]LoadConfig{
		"missing root":   {},
		"invalid target": {ModuleRoot: contractFixtureRoot(), Target: "other"},
	} {
		t.Run(name, func(t *testing.T) {
			if _, err := LoadPackages(context.Background(), config); err == nil {
				t.Fatal("LoadPackages() succeeded")
			}
		})
	}
}

func contractFixtureRoot() string { return "testdata/contracts" }

func hasRole(roles []SourceRole, wanted SourceRole) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

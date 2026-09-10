package analyzer

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestWorkspaceRulesAcceptPortableApplication(t *testing.T) {
	root := workspaceFixture(t, `func load(g playdate.Graphics) { _, _ = g.LoadBitmap("images/player") }`)
	writeAnalyzerFixture(t, root, "resources/images/player.png", "png")
	writeAnalyzerFixture(t, root, "resources/images/launcher/card.png", "png")
	writeAnalyzerFixture(t, root, "resources/images/launcher/icon.png", "png")
	writeAnalyzerFixture(t, root, "resources/images/launcher/launchImage.png", "png")
	findings := runWorkspaceFixture(t, root)
	if len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func TestWorkspaceRulesReportManifestResourcesAndVersion(t *testing.T) {
	root := workspaceFixture(t, `func load(g playdate.Graphics) { _, _ = g.LoadBitmap("images/PLAYER") }`)
	writeAnalyzerFixture(t, root, "go.mod", strings.ReplaceAll(readFixture(t, filepath.Join(root, "go.mod")), "v1.0.0", "v0.0.0"))
	writeAnalyzerFixture(t, root, "pdxinfo", "name=Fixture\nname=Duplicate\nauthor=Author\nbundleID=unsafe id\nversion=1.0.0\nbuildNumber=zero\n")
	writeAnalyzerFixture(t, root, "resources/images/player.png", "png")
	findings := runWorkspaceFixture(t, root)
	for _, rule := range []RuleID{"workspace-module-version", "workspace-manifest", "workspace-missing-resource"} {
		if !hasWorkspaceRule(findings, rule) {
			t.Fatalf("missing %s in %+v", rule, findings)
		}
	}
}

func TestWorkspaceResourcePathValidation(t *testing.T) {
	for _, value := range []string{"../secret", "/absolute", `windows\path`, "C:drive"} {
		if validResourcePath(value) {
			t.Fatalf("validResourcePath(%q) = true", value)
		}
	}
	for _, value := range []string{"images/player", "data/level.json"} {
		if !validResourcePath(value) {
			t.Fatalf("validResourcePath(%q) = false", value)
		}
	}
	if name, exact := matchResource("images/characters", "LoadBitmapTable", map[string]string{"images/characters-table-32-32.png": ""}); name == "" || !exact {
		t.Fatalf("bitmap table source match = %q, %t", name, exact)
	}
}

func TestWorkspaceRulesIgnoreLibraryWithoutManifest(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", "module example.com/library\n\ngo 1.26.5\n")
	writeAnalyzerFixture(t, root, "library.go", "package library\n")
	if findings := runWorkspaceFixture(t, root); len(findings) != 0 {
		t.Fatalf("findings = %+v", findings)
	}
}

func workspaceFixture(t *testing.T, body string) string {
	t.Helper()
	root := t.TempDir()
	repository := repositoryRoot(t)
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/workspacefixture\n\ngo 1.26.5\n\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repository)))
	writeAnalyzerFixture(t, root, "game.go", "package game\nimport \"github.com/ivan-gromov-dev/gopdsdk/playdate\"\n"+body+"\n")
	writeAnalyzerFixture(t, root, "pdxinfo", "name=Fixture\nauthor=Author\nbundleID=com.example.fixture\nversion=1.0.0\nbuildNumber=1\nimagePath=images/launcher\n")
	return root
}

func runWorkspaceFixture(t *testing.T, root string) []Finding {
	t.Helper()
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Patterns: []string{"."}, Target: TargetSimulator})
	if err != nil {
		t.Fatal(err)
	}
	active := map[RuleID]bool{"workspace-module-version": true, "workspace-manifest": true, "workspace-resource-path": true, "workspace-missing-resource": true}
	findings, err := workspaceFindings(context.Background(), snapshot, active)
	if err != nil {
		t.Fatal(err)
	}
	return findings
}

func hasWorkspaceRule(findings []Finding, rule RuleID) bool {
	for _, finding := range findings {
		if finding.RuleID == rule {
			return true
		}
	}
	return false
}

func readFixture(t *testing.T, name string) string {
	t.Helper()
	data, err := os.ReadFile(name)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

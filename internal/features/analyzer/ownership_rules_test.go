package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestOwnershipRulesReportLocalViolations(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func leak(g playdate.Graphics) error { b, err := g.NewBitmap(8, 8); if err != nil { return err }; _ = b; return nil }
func double(g playdate.Graphics) error { b, err := g.NewBitmap(8, 8); if err != nil { return err }; if err = b.Close(); err != nil { return err }; return b.Close() }
func use(g playdate.Graphics) error { b, err := g.NewBitmap(8, 8); if err != nil { return err }; if err = b.Close(); err != nil { return err }; return b.Fill(playdate.ColorBlack) }
func borrowed(g playdate.Graphics) error { table, err := g.LoadBitmapTable("tiles"); if err != nil { return err }; frame, err := table.Frame(0); if err != nil { _ = table.Close(); return err }; _ = table.Close(); return frame.Close() }
func retained(c playdate.SystemControls, g playdate.Graphics) error { b, err := g.NewBitmap(400, 240); if err != nil { return err }; _ = c.SetMenuImage(b, 0); return b.Close() }
func ordered(c playdate.SystemControls, g playdate.Graphics) error { b, err := g.NewBitmap(400, 240); if err != nil { return err }; _ = c.SetMenuImage(b, 0); c.ClearMenuImage(); return b.Close() }
`)
	output := runOwnershipFixture(t, root)
	for _, id := range []RuleID{"ownership-resource-leak", "ownership-double-close", "ownership-bitmap-use-after-close", "ownership-borrowed-close", "ownership-retained-close"} {
		if !strings.Contains(output, string(id)) {
			t.Errorf("missing %s:\n%s", id, output)
		}
	}
	if strings.Contains(output, "game.go:8:") {
		t.Fatalf("ordered cleanup produced a finding:\n%s", output)
	}
}

func TestOwnershipRulesStaySilentForEscapeAndBranches(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func transfer(playdate.Bitmap) {}
func good(g playdate.Graphics, branch bool) error {
 b, err := g.NewBitmap(8, 8); if err != nil { return err }
 if branch { transfer(b); return nil }
 return b.Close()
}
func deferred(g playdate.Graphics) error {
 b, err := g.NewBitmap(8, 8); if err != nil { return err }
 defer b.Close()
 return b.Fill(playdate.ColorBlack)
}
func loop(g playdate.Graphics, count int) error {
 for i := 0; i < count; i++ {
  b, err := g.NewBitmap(8, 8); if err != nil { return err }
  if err := b.Close(); err != nil { return err }
 }
 return nil
}
`)
	if output := runOwnershipFixture(t, root); output != "No findings.\n" {
		t.Fatalf("unexpected findings:\n%s", output)
	}
}

func TestOwnershipRulesTrackAggregateAndAliasOrder(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func bad(g playdate.Graphics, maps playdate.SpriteTileMaps) error {
 table, err := g.LoadBitmapTable("tiles"); if err != nil { return err }
 alias := table
 tilemap, err := maps.NewSpriteTileMap(alias, 1, 1, []uint16{1}); if err != nil { _ = table.Close(); return err }
 _ = tilemap
 return table.Close()
}
`)
	output := runOwnershipFixture(t, root)
	if !strings.Contains(output, "ownership-close-order") || !strings.Contains(output, "retaining operation is here") {
		t.Fatalf("aggregate close order was not reported with related information:\n%s", output)
	}
}

func TestOwnershipRulesUseBoundedSummariesOnlyInDeepProfile(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func closeBitmap(b playdate.Bitmap) error { return b.Close() }
func use(g playdate.Graphics) error {
 b, err := g.NewBitmap(8, 8); if err != nil { return err }
 if err := closeBitmap(b); err != nil { return err }
 return b.Fill(playdate.ColorBlack)
}
func makeBitmap(g playdate.Graphics) (playdate.Bitmap, error) { return g.NewBitmap(8, 8) }
func leak(g playdate.Graphics) error {
 b, err := makeBitmap(g); if err != nil { return err }; _ = b; return nil
}
`)
	if output := runOwnershipFixture(t, root); output != "No findings.\n" {
		t.Fatalf("default profile used deep summaries:\n%s", output)
	}
	output := runOwnershipFixtureWithProfile(t, root, "deep")
	if !strings.Contains(output, "ownership-bitmap-use-after-close") || !strings.Contains(output, "ownership-resource-leak") || !strings.Contains(output, "closed through") {
		t.Fatalf("deep ownership summaries were not applied with call-path context:\n%s", output)
	}
}

func TestOwnershipRulesImportDependencySummaries(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "helpers/helpers.go", `package helpers
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func Close(b playdate.Bitmap) error { return b.Close() }
func New(g playdate.Graphics) (playdate.Bitmap, error) { return g.NewBitmap(8, 8) }
`)
	writeAnalyzerFixture(t, root, "game.go", `package game
import (
 "example.com/ownership/helpers"
 "github.com/ivan-gromov-dev/gopdsdk/playdate"
)
func bad(g playdate.Graphics) error {
 b, err := helpers.New(g); if err != nil { return err }
 if err := helpers.Close(b); err != nil { return err }
 return b.Fill(playdate.ColorBlack)
}
`)
	output := runOwnershipFixtureWithProfile(t, root, "deep")
	if !strings.Contains(output, "ownership-bitmap-use-after-close") || !strings.Contains(output, "helpers.Close") {
		t.Fatalf("dependency summary was not imported:\n%s", output)
	}
}

func TestOwnershipRulesPropagateHelperRetention(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/ownership\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
func retain(c playdate.SystemControls, b playdate.Bitmap) { _ = c.SetMenuImage(b, 0) }
func bad(c playdate.SystemControls, g playdate.Graphics) error {
 b, err := g.NewBitmap(400, 240); if err != nil { return err }
 retain(c, b)
 return b.Close()
}
`)
	output := runOwnershipFixtureWithProfile(t, root, "deep")
	if !strings.Contains(output, "ownership-retained-close") || !strings.Contains(output, "retaining operation is here") {
		t.Fatalf("helper retention was not propagated:\n%s", output)
	}
}

func runOwnershipFixture(t *testing.T, root string) string {
	return runOwnershipFixtureWithProfile(t, root, "default")
}

func runOwnershipFixtureWithProfile(t *testing.T, root, profile string) string {
	t.Helper()
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	var output bytes.Buffer
	err = RunCheck(context.Background(), []string{"check", "--format", "text", "--target", "shared", "--profile", profile, "--categories", "ownership", "."}, &output, &bytes.Buffer{}, options)
	if err != nil {
		var command *CommandError
		if !errorsAs(err, &command) || command.Code != ExitFindings {
			t.Fatal(err)
		}
	}
	return output.String()
}

package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
)

func TestResultRulesReportSDKSpecificContracts(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/results\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import (
 "github.com/ivan-gromov-dev/gopdsdk/playdate"
 "github.com/ivan-gromov-dev/gopdsdk/playdate/diagnostics"
 "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
)
func bad(c playdate.SystemControls, b playdate.Bitmap, f playdate.File, q *schedule.Queue[int], tm *playdate.TileMap, err error, branch bool) {
 _ = c.SetMenuImage(b, 201)
 _, _ = diagnostics.New(0)
 _, _ = schedule.NewQueue[int](0)
 _ = f.Close()
 defer f.Close()
 q.TrySend(1)
 _, _ = tm.TileAt(0, 0)
 if err == playdate.ErrFileIO {}
 if _, ok := err.(playdate.FileOperationError); ok {}
 offset := 0
 if branch { offset = 201 }
 _ = c.SetMenuImage(b, offset)
}
`)
	findings := runResultFixture(t, root)
	for id, minimum := range map[RuleID]int{
		"result-menu-image-offset":    2,
		"result-invalid-argument":     2,
		"result-sdk-error-discarded":  4,
		"result-sdk-value-discarded":  1,
		"result-sdk-error-comparison": 1,
		"result-sdk-typed-diagnostic": 1,
	} {
		if count := strings.Count(findings, string(id)); count < minimum {
			t.Errorf("%s findings = %d, want at least %d\n%s", id, count, minimum, findings)
		}
	}
}

func TestResultRulesAcceptObservedContractsAndUnrelatedAPIs(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/results\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import (
 "errors"
 "github.com/ivan-gromov-dev/gopdsdk/playdate"
 "github.com/ivan-gromov-dev/gopdsdk/playdate/diagnostics"
 "github.com/ivan-gromov-dev/gopdsdk/playdate/schedule"
)
type local struct{}
func (local) Close() error { return nil }
func good(c playdate.SystemControls, b playdate.Bitmap, f playdate.File, q *schedule.Queue[int], tm *playdate.TileMap, err error) error {
 if err := c.SetMenuImage(b, 200); err != nil { return err }
 if _, err := diagnostics.New(36000); err != nil { return err }
 if _, err := schedule.NewQueue[int](1); err != nil { return err }
 if !q.TrySend(1) { return errors.New("full") }
 if _, ok := tm.TileAt(0, 0); !ok { return errors.New("missing") }
 if errors.Is(err, playdate.ErrFileIO) { return err }
 var detail playdate.FileOperationError
 if errors.As(err, &detail) { return detail }
 _ = local{}.Close()
 return f.Close()
}
`)
	if findings := runResultFixture(t, root); findings != "" && findings != "No findings.\n" {
		t.Fatalf("unexpected findings:\n%s", findings)
	}
}

func runResultFixture(t *testing.T, root string) string {
	t.Helper()
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	options.ModuleRoot = root
	var output bytes.Buffer
	err = RunCheck(context.Background(), []string{"check", "--format", "text", "--target", "shared", "--rules", "result-menu-image-offset,result-invalid-argument,result-sdk-error-discarded,result-sdk-value-discarded,result-sdk-error-comparison,result-sdk-typed-diagnostic", "."}, &output, &bytes.Buffer{}, options)
	if err != nil {
		var command *CommandError
		if !errorsAs(err, &command) || command.Code != ExitFindings {
			t.Fatal(err)
		}
	}
	return output.String()
}

func errorsAs(err error, target any) bool {
	// Kept local so fixture source can exercise errors.As without making test
	// assertions depend on the same spelling.
	command, ok := err.(*CommandError)
	if !ok {
		return false
	}
	pointer, ok := target.(**CommandError)
	if ok {
		*pointer = command
	}
	return ok
}

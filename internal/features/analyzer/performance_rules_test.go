package analyzer

import (
	"context"
	"fmt"
	"path/filepath"
	"testing"
)

func TestPerformanceRulesClassifyFrameAndAudioPaths(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/performance\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
type game struct{}
func (game) Update(playdate.Context)(bool,error){ frameHelper(); return true,nil }
func frameHelper(){ _ = make([]byte, 4); for { break } }
func install(a playdate.CallbackAudio, c playdate.AudioChannel){ _,_ = a.NewPCMCallbackSource(c,false,render) }
func render(left,right []int16) int { _ = make([]byte, 4); return len(left) }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetShared})
	if err != nil || snapshotHasLoadErrors(snapshot) {
		t.Fatalf("load: %v %+v", err, snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyPerformance}, IncludeExperimental: true})
	if err != nil {
		t.Fatal(err)
	}
	want := map[RuleID]bool{"performance-frame-allocation": false, "performance-frame-unbounded-work": false, "performance-audio-callback-risk": false}
	for _, finding := range result.Findings {
		if _, ok := want[finding.RuleID]; ok {
			want[finding.RuleID] = true
			if len(finding.Related) == 0 {
				t.Errorf("%s has no hot-path root", finding.RuleID)
			}
		}
	}
	for rule, found := range want {
		if !found {
			t.Errorf("missing %s in %+v", rule, result.Findings)
		}
	}
}

func TestPerformanceRulesIgnoreOneTimeAndFixedStorage(t *testing.T) {
	root := t.TempDir()
	writeAnalyzerFixture(t, root, "go.mod", fmt.Sprintf("module example.com/performanceok\n\ngo 1.26.5\nrequire github.com/ivan-gromov-dev/gopdsdk v1.0.0\nreplace github.com/ivan-gromov-dev/gopdsdk => %s\n", filepath.ToSlash(repositoryRoot(t))))
	writeAnalyzerFixture(t, root, "game.go", `package game
import "github.com/ivan-gromov-dev/gopdsdk/playdate"
type game struct{ pool [8]byte }
func (g *game) Init(playdate.Context) error { _ = make([]byte, 4); return nil }
func (g *game) Update(playdate.Context)(bool,error){ for i:=range g.pool { g.pool[i]=0 }; return true,nil }
`)
	options, err := DefaultCheckOptions()
	if err != nil {
		t.Fatal(err)
	}
	snapshot, err := LoadPackages(context.Background(), LoadConfig{ModuleRoot: root, Target: TargetShared})
	if err != nil || snapshotHasLoadErrors(snapshot) {
		t.Fatalf("load: %v %+v", err, snapshot.Packages)
	}
	result, err := options.Registry.Run(context.Background(), snapshot, RuleSelection{Families: []RuleFamily{FamilyPerformance}, IncludeExperimental: true})
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Findings) != 0 {
		t.Fatalf("fixed storage or initialization diagnosed: %+v", result.Findings)
	}
}

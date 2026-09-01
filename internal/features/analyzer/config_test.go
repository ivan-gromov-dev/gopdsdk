package analyzer

import (
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

func TestLoadRepositoryConfigValidatesAndNormalizes(t *testing.T) {
	root := t.TempDir()
	writeCheckConfig(t, root, `{
  "schema": "gopdsdk-check-config/v1",
  "format": "json",
  "target": "device",
  "buildTags": ["zeta", "alpha", "zeta"],
  "tests": true,
  "patterns": ["./game", "./tools/..."]
}`)
	config, err := loadRepositoryConfig(root, "", false)
	if err != nil {
		t.Fatal(err)
	}
	if config.Format == nil || *config.Format != "json" || config.Target == nil || *config.Target != "device" || config.Tests == nil || !*config.Tests {
		t.Fatalf("decoded config = %+v", config)
	}
	if !reflect.DeepEqual(*config.BuildTags, []string{"alpha", "zeta"}) || !reflect.DeepEqual(*config.Patterns, []string{"./game", "./tools/..."}) {
		t.Fatalf("normalized config = %+v", config)
	}
	if _, err := loadRepositoryConfig(t.TempDir(), "", false); err != nil {
		t.Fatalf("absent implicit config = %v", err)
	}
	if _, err := loadRepositoryConfig(t.TempDir(), "missing.json", true); err == nil {
		t.Fatal("absent explicit config succeeded")
	}
}

func TestLoadRepositoryConfigRejectsMalformedValues(t *testing.T) {
	for name, content := range map[string]string{
		"unknown schema": `{"schema":"gopdsdk-check-config/v2"}`,
		"unknown field":  `{"schema":"gopdsdk-check-config/v1","targte":"device"}`,
		"invalid target": `{"schema":"gopdsdk-check-config/v1","target":"host"}`,
		"empty patterns": `{"schema":"gopdsdk-check-config/v1","patterns":[]}`,
		"invalid tag":    `{"schema":"gopdsdk-check-config/v1","buildTags":["two words"]}`,
		"trailing value": `{"schema":"gopdsdk-check-config/v1"} {}`,
	} {
		t.Run(name, func(t *testing.T) {
			root := t.TempDir()
			writeCheckConfig(t, root, content)
			if _, err := loadRepositoryConfig(root, "", false); err == nil {
				t.Fatal("invalid configuration succeeded")
			}
		})
	}
}

func writeCheckConfig(t *testing.T, root, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(root, RepositoryConfigName), []byte(strings.TrimSpace(content)+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}
}

// Package initproject creates independent gopdsdk application modules.
package initproject

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/ivan-gromov-dev/gopdsdk/internal/shared/gomodule"
)

const sdkModule = "github.com/ivan-gromov-dev/gopdsdk"

var releaseVersionPattern = regexp.MustCompile(`^v(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)

// Config describes a new application project.
type Config struct {
	Path     string
	Module   string
	Name     string
	Author   string
	BundleID string
	SDKDir   string
	// SDKVersion selects a published gopdsdk module. SDKDir selects a local
	// development checkout. Exactly one must be set.
	SDKVersion string
	GoVersion  string
}

// Result identifies the created project.
type Result struct {
	Path   string
	Module string
}

// Create writes a new project without modifying an existing path.
func Create(config Config) (result Result, err error) {
	if config.Path == "" {
		return Result{}, fmt.Errorf("project path is required")
	}
	path, err := filepath.Abs(filepath.Clean(config.Path))
	if err != nil {
		return Result{}, fmt.Errorf("resolve project path: %w", err)
	}
	if _, statErr := os.Stat(path); statErr == nil {
		return Result{}, fmt.Errorf("project path already exists: %s", path)
	} else if !os.IsNotExist(statErr) {
		return Result{}, fmt.Errorf("inspect project path: %w", statErr)
	}

	slug := projectSlug(filepath.Base(path))
	if slug == "" {
		return Result{}, fmt.Errorf("project directory name must contain a letter or digit")
	}
	if config.Module == "" {
		config.Module = "example.com/" + slug
	}
	if config.Name == "" {
		config.Name = filepath.Base(path)
	}
	if config.Author == "" {
		config.Author = "Your Name"
	}
	if config.BundleID == "" {
		config.BundleID = "com.example." + slug
	}
	if config.GoVersion == "" {
		return Result{}, fmt.Errorf("Go version is required")
	}
	if (config.SDKDir == "") == (config.SDKVersion == "") {
		return Result{}, fmt.Errorf("exactly one gopdsdk module directory or version is required")
	}
	if config.SDKVersion != "" && !releaseVersionPattern.MatchString(config.SDKVersion) {
		return Result{}, fmt.Errorf("gopdsdk module version must be a stable vMAJOR.MINOR.PATCH release")
	}
	for _, field := range []struct {
		name  string
		value string
	}{
		{name: "module", value: config.Module},
		{name: "name", value: config.Name},
		{name: "author", value: config.Author},
		{name: "bundle ID", value: config.BundleID},
	} {
		if strings.ContainsAny(field.value, "\r\n") || strings.TrimSpace(field.value) == "" {
			return Result{}, fmt.Errorf("%s must be a non-empty single-line value", field.name)
		}
	}
	if !strings.Contains(config.Module, "/") || strings.ContainsAny(config.Module, " \t") {
		return Result{}, fmt.Errorf("module must be a valid module path")
	}
	if !strings.Contains(config.BundleID, ".") || strings.ContainsAny(config.BundleID, " \t=") {
		return Result{}, fmt.Errorf("bundle ID must use reverse DNS notation")
	}

	if err := os.MkdirAll(path, 0o755); err != nil {
		return Result{}, fmt.Errorf("create project directory: %w", err)
	}
	created := true
	defer func() {
		if err != nil && created {
			_ = os.RemoveAll(path)
		}
	}()
	files := []struct {
		name     string
		contents string
	}{
		{name: "go.mod", contents: renderGoMod(config)},
		{name: "game.go", contents: renderGame()},
		{name: "pdxinfo", contents: renderPDXInfo(config)},
	}
	for _, file := range files {
		if writeErr := os.WriteFile(filepath.Join(path, file.name), []byte(file.contents), 0o644); writeErr != nil {
			return Result{}, fmt.Errorf("write %s: %w", file.name, writeErr)
		}
	}
	created = false
	return Result{Path: path, Module: config.Module}, nil
}

func projectSlug(name string) string {
	var builder strings.Builder
	separator := false
	for _, character := range strings.ToLower(name) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
			separator = false
		} else if builder.Len() > 0 && !separator {
			builder.WriteByte('-')
			separator = true
		}
	}
	return strings.Trim(builder.String(), "-")
}

func renderGoMod(config Config) string {
	if config.SDKVersion != "" {
		return fmt.Sprintf("module %s\n\ngo %s\n\nrequire %s %s\n",
			config.Module, config.GoVersion, sdkModule, config.SDKVersion)
	}
	return fmt.Sprintf("module %s\n\ngo %s\n\nrequire %s v0.0.0\n\nreplace %s => %s\n",
		config.Module, config.GoVersion, sdkModule, sdkModule, gomodule.FormatPath(config.SDKDir))
}

func renderGame() string {
	return `// Package game contains the Playdate application.
package game

import "github.com/ivan-gromov-dev/gopdsdk/playdate"

type game struct{}

// New creates the application entry point expected by gopdsdk.
func New() playdate.Game { return game{} }

func (game) Init(playdate.Context) error { return nil }

func (game) Update(context playdate.Context) (bool, error) {
context.DrawText("Hello from gopdsdk", 16, 16)
	return true, nil
}
`
}

func renderPDXInfo(config Config) string {
	return fmt.Sprintf("name=%s\nauthor=%s\nbundleID=%s\nversion=0.0.1\nbuildNumber=1\n",
		config.Name, config.Author, config.BundleID)
}

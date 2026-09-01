package analyzer

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"golang.org/x/tools/go/packages"
)

// LoadConfig selects one read-only package snapshot.
type LoadConfig struct {
	ModuleRoot string
	Patterns   []string
	BuildTags  []string
	Tests      bool
	Target     Target
	Overlay    map[string][]byte
}

// SourceRole classifies one source file without discarding overlapping roles.
type SourceRole string

const (
	RoleProduction SourceRole = "production"
	RoleTest       SourceRole = "test"
	RoleExample    SourceRole = "example"
	RoleGenerated  SourceRole = "generated"
	RoleVendored   SourceRole = "vendored"
	RoleDependency SourceRole = "dependency"
	RoleShared     SourceRole = "shared"
	RoleSimulator  SourceRole = "simulator"
	RoleDevice     SourceRole = "device"
)

// SourceFile is a normalized source identity relative to the analyzed module
// whenever the file belongs to that module.
type SourceFile struct {
	Path         string
	AbsolutePath string
	Roles        []SourceRole
}

// Package is one deterministically ordered loaded Go package.
type Package struct {
	ID      string
	Path    string
	Name    string
	Files   []SourceFile
	Errors  []LoadError
	Partial bool
}

// LoadError preserves a package loading, parse, or type error without turning
// a partial package into an internal analyzer failure.
type LoadError struct {
	PackageID string
	Position  string
	Message   string
	Kind      string
}

// Snapshot is the normalized result of one package load. Loaded contains the
// underlying analysis inputs and is intentionally internal to this feature.
type Snapshot struct {
	ModuleRoot string
	Target     Target
	Packages   []Package
	Loaded     []*packages.Package
	overlay    map[string][]byte
}

// LoadPackages loads syntax and type information without mutating the target
// module. Package errors are returned in the snapshot; configuration,
// invocation, and cancellation errors are returned directly.
func LoadPackages(ctx context.Context, config LoadConfig) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	root, err := validateLoadConfig(config)
	if err != nil {
		return Snapshot{}, err
	}
	overlay, err := normalizeOverlay(root, config.Overlay)
	if err != nil {
		return Snapshot{}, err
	}
	patterns := append([]string(nil), config.Patterns...)
	if len(patterns) == 0 {
		patterns = []string{"./..."}
	}
	packageConfig := &packages.Config{
		Context: ctx,
		Dir:     root,
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo |
			packages.NeedImports | packages.NeedDeps | packages.NeedModule,
		Tests:      config.Tests,
		Overlay:    overlay,
		BuildFlags: []string{"-mod=readonly"},
	}
	if len(config.BuildTags) != 0 {
		packageConfig.BuildFlags = append(packageConfig.BuildFlags, "-tags="+strings.Join(config.BuildTags, ","))
	}
	loaded, err := packages.Load(packageConfig, patterns...)
	if err != nil {
		if contextErr := ctx.Err(); contextErr != nil {
			return Snapshot{}, contextErr
		}
		return Snapshot{}, fmt.Errorf("load Go packages: %w", err)
	}
	if contextErr := ctx.Err(); contextErr != nil {
		return Snapshot{}, contextErr
	}
	sort.Slice(loaded, func(i, j int) bool { return loaded[i].ID < loaded[j].ID })
	snapshot := Snapshot{ModuleRoot: filepath.ToSlash(root), Target: config.Target, Loaded: loaded, overlay: overlay}
	for _, loadedPackage := range loaded {
		snapshot.Packages = append(snapshot.Packages, normalizePackage(root, config.Target, overlay, loadedPackage))
	}
	return snapshot, nil
}

func validateLoadConfig(config LoadConfig) (string, error) {
	if config.Target != TargetShared && config.Target != TargetSimulator && config.Target != TargetDevice {
		return "", fmt.Errorf("invalid analyzer target %q", config.Target)
	}
	if config.ModuleRoot == "" {
		return "", fmt.Errorf("analyzer module root is empty")
	}
	root, err := filepath.Abs(config.ModuleRoot)
	if err != nil {
		return "", fmt.Errorf("resolve analyzer module root: %w", err)
	}
	info, err := os.Stat(root)
	if err != nil {
		return "", fmt.Errorf("inspect analyzer module root: %w", err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("analyzer module root %q is not a directory", root)
	}
	if _, err := os.Stat(filepath.Join(root, "go.mod")); err != nil {
		return "", fmt.Errorf("analyzer module root has no readable go.mod: %w", err)
	}
	return filepath.Clean(root), nil
}

func normalizeOverlay(root string, source map[string][]byte) (map[string][]byte, error) {
	if len(source) == 0 {
		return nil, nil
	}
	overlay := make(map[string][]byte, len(source))
	for name, content := range source {
		path := name
		if !filepath.IsAbs(path) {
			path = filepath.Join(root, path)
		}
		absolute, err := filepath.Abs(path)
		if err != nil {
			return nil, fmt.Errorf("resolve overlay %q: %w", name, err)
		}
		overlay[filepath.Clean(absolute)] = bytes.Clone(content)
	}
	return overlay, nil
}

func normalizePackage(root string, target Target, overlay map[string][]byte, loaded *packages.Package) Package {
	result := Package{ID: loaded.ID, Path: loaded.PkgPath, Name: loaded.Name, Partial: len(loaded.Errors) != 0}
	for _, name := range loaded.CompiledGoFiles {
		result.Files = append(result.Files, normalizeSourceFile(root, target, overlay, name))
	}
	sort.Slice(result.Files, func(i, j int) bool { return result.Files[i].Path < result.Files[j].Path })
	for _, packageError := range loaded.Errors {
		result.Errors = append(result.Errors, LoadError{PackageID: loaded.ID, Position: normalizePosition(root, packageError.Pos), Message: packageError.Msg, Kind: loadErrorKind(packageError.Kind)})
	}
	sort.Slice(result.Errors, func(i, j int) bool {
		left, right := result.Errors[i], result.Errors[j]
		return left.Position < right.Position || left.Position == right.Position && left.Message < right.Message
	})
	return result
}

func loadErrorKind(kind packages.ErrorKind) string {
	switch kind {
	case packages.ListError:
		return "list"
	case packages.ParseError:
		return "parse"
	case packages.TypeError:
		return "type"
	default:
		return "unknown"
	}
}

func normalizeSourceFile(root string, target Target, overlay map[string][]byte, name string) SourceFile {
	absolute, _ := filepath.Abs(name)
	absolute = filepath.Clean(absolute)
	relative, err := filepath.Rel(root, absolute)
	dependency := err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator))
	path := filepath.ToSlash(relative)
	if dependency {
		path = filepath.ToSlash(absolute)
	}
	roles := []SourceRole{RoleProduction}
	if strings.HasSuffix(strings.ToLower(absolute), "_test.go") {
		roles[0] = RoleTest
	}
	segments := "/" + strings.ToLower(filepath.ToSlash(relative)) + "/"
	if strings.Contains(segments, "/examples/") || strings.HasPrefix(strings.TrimPrefix(segments, "/"), "examples/") {
		roles = append(roles, RoleExample)
	}
	if strings.Contains(segments, "/vendor/") {
		roles = append(roles, RoleVendored)
	}
	if dependency {
		roles = append(roles, RoleDependency)
	}
	content := overlay[absolute]
	if content == nil {
		content, _ = os.ReadFile(absolute)
	}
	if generatedSource(content) {
		roles = append(roles, RoleGenerated)
	}
	switch target {
	case TargetShared:
		roles = append(roles, RoleShared)
	case TargetSimulator:
		roles = append(roles, RoleSimulator)
	case TargetDevice:
		roles = append(roles, RoleDevice)
	}
	return SourceFile{Path: path, AbsolutePath: filepath.ToSlash(absolute), Roles: roles}
}

func generatedSource(content []byte) bool {
	if len(content) > 2048 {
		content = content[:2048]
	}
	return bytes.Contains(content, []byte("// Code generated ")) && bytes.Contains(content, []byte(" DO NOT EDIT."))
}

func normalizePosition(root, position string) string {
	if position == "" {
		return ""
	}
	// packages.Error positions end in :line[:column], so only normalize a
	// leading absolute module root and preserve the compiler-owned suffix.
	root = filepath.ToSlash(root)
	position = filepath.ToSlash(position)
	if strings.HasPrefix(strings.ToLower(position), strings.ToLower(root)+"/") {
		return strings.TrimPrefix(position[len(root):], "/")
	}
	return position
}

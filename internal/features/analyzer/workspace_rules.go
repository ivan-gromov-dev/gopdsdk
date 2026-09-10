package analyzer

import (
	"bufio"
	"context"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"golang.org/x/mod/modfile"
)

const gopdsdkModule = "github.com/ivan-gromov-dev/gopdsdk"

func workspaceFindings(ctx context.Context, snapshot Snapshot, active map[RuleID]bool) ([]Finding, error) {
	root := filepath.FromSlash(snapshot.ModuleRoot)
	var findings []Finding
	add := func(rule RuleID, file string, line, column int, message string) {
		if !active[rule] {
			return
		}
		position := fmt.Sprintf("%s:%d:%d", filepath.ToSlash(file), line, column)
		findings = append(findings, Finding{RuleID: rule, Analyzer: "workspace", PackageID: "workspace", Target: snapshot.Target, Position: position, End: position, Category: string(FamilyWorkspace), Message: message})
	}
	applications := workspaceApplications(snapshot)
	if len(applications) != 0 && active["workspace-module-version"] {
		data, err := os.ReadFile(filepath.Join(root, "go.mod"))
		if err != nil {
			return nil, fmt.Errorf("read workspace go.mod: %w", err)
		}
		module, err := modfile.Parse("go.mod", data, nil)
		if err != nil {
			add("workspace-module-version", "go.mod", 1, 1, "go.mod cannot be parsed: "+err.Error())
		} else if module.Module == nil || module.Module.Mod.Path != gopdsdkModule {
			var requirement *modfile.Require
			for _, item := range module.Require {
				if item.Mod.Path == gopdsdkModule {
					requirement = item
					break
				}
			}
			if requirement == nil {
				add("workspace-module-version", "go.mod", 1, 1, "application module must require "+gopdsdkModule)
			} else if requirement.Mod.Version == "" || requirement.Mod.Version == "v0.0.0" {
				add("workspace-module-version", "go.mod", requirement.Syntax.Start.Line, 1, "gopdsdk dependency must declare a release version")
			}
		}
	}
	for _, application := range applications {
		relative, _ := filepath.Rel(root, application)
		prefix := filepath.ToSlash(relative)
		manifestPath := path.Join(prefix, "pdxinfo")
		manifestValues, err := inspectPDXInfo(filepath.Join(application, "pdxinfo"), manifestPath, add)
		if err != nil {
			return nil, err
		}
		resources, err := inspectResources(ctx, filepath.Join(application, "resources"), path.Join(prefix, "resources"), add)
		if err != nil {
			return nil, err
		}
		if imagePath := manifestValues["imagePath"]; imagePath != "" {
			for _, name := range []string{"card.png", "icon.png", "launchImage.png"} {
				wanted := path.Join(imagePath, name)
				if _, ok := resources[wanted]; !ok {
					add("workspace-missing-resource", manifestPath, 1, 1, "imagePath requires packaged resource "+wanted)
				}
			}
		}
		inspectStaticResourceCalls(snapshot, application, resources, add)
	}
	sortFindings(findings)
	return findings, nil
}

func workspaceApplications(snapshot Snapshot) []string {
	seen := map[string]bool{}
	var result []string
	for _, pkg := range snapshot.Loaded {
		for _, name := range pkg.CompiledGoFiles {
			directory := filepath.Dir(name)
			if seen[directory] {
				continue
			}
			seen[directory] = true
			if info, err := os.Stat(filepath.Join(directory, "pdxinfo")); err == nil && !info.IsDir() {
				result = append(result, directory)
			}
		}
	}
	sort.Strings(result)
	return result
}

func inspectPDXInfo(name, displayName string, add func(RuleID, string, int, int, string)) (map[string]string, error) {
	file, err := os.Open(name)
	if err != nil {
		return nil, fmt.Errorf("read workspace pdxinfo: %w", err)
	}
	defer file.Close()
	values, lines := map[string]string{}, map[string]int{}
	scanner := bufio.NewScanner(file)
	for line := 1; scanner.Scan(); line++ {
		text := strings.TrimSpace(scanner.Text())
		if text == "" {
			continue
		}
		key, value, found := strings.Cut(text, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		if !found || key == "" || value == "" {
			add("workspace-manifest", displayName, line, 1, "manifest line must be key=value")
			continue
		}
		if prior := lines[key]; prior != 0 {
			add("workspace-manifest", displayName, line, 1, fmt.Sprintf("field %s duplicates line %d", key, prior))
			continue
		}
		values[key], lines[key] = value, line
		if strings.ContainsAny(value, "\x00\r\n") {
			add("workspace-manifest", displayName, line, 1, "manifest value contains an unsafe control character")
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read workspace pdxinfo: %w", err)
	}
	for _, field := range []string{"name", "author", "bundleID", "version", "buildNumber"} {
		if values[field] == "" {
			add("workspace-manifest", displayName, 1, 1, "required field "+field+" is missing")
		}
	}
	if value := values["bundleID"]; value != "" && (!strings.Contains(value, ".") || strings.ContainsAny(value, " \t/\\")) {
		add("workspace-manifest", displayName, lines["bundleID"], 1, "bundleID must use safe reverse-DNS notation")
	}
	if value := values["buildNumber"]; value != "" {
		number, parseErr := strconv.ParseUint(value, 10, 64)
		if parseErr != nil || number == 0 {
			add("workspace-manifest", displayName, lines["buildNumber"], 1, "buildNumber must be a positive integer")
		}
	}
	for _, field := range []string{"imagePath"} {
		if value := values[field]; value != "" && !validResourcePath(value) {
			add("workspace-manifest", displayName, lines[field], 1, field+" must be a safe package-relative path")
		}
	}
	return values, nil
}

func inspectResources(ctx context.Context, root, displayRoot string, add func(RuleID, string, int, int, string)) (map[string]string, error) {
	result, folded := map[string]string{}, map[string]string{}
	err := filepath.WalkDir(root, func(name string, entry fs.DirEntry, walkErr error) error {
		if walkErr != nil {
			if os.IsNotExist(walkErr) && name == root {
				return nil
			}
			return walkErr
		}
		if err := ctx.Err(); err != nil {
			return err
		}
		if name == root {
			return nil
		}
		relative, err := filepath.Rel(root, name)
		if err != nil {
			return err
		}
		relative = filepath.ToSlash(relative)
		if entry.Type()&os.ModeSymlink != 0 {
			add("workspace-resource-path", path.Join(displayRoot, relative), 1, 1, "symbolic links are excluded from packaged resources")
			if entry.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if entry.IsDir() {
			return nil
		}
		if !validResourcePath(relative) {
			add("workspace-resource-path", path.Join(displayRoot, relative), 1, 1, "resource path is not a safe portable package path")
			return nil
		}
		key := strings.ToLower(relative)
		if prior, exists := folded[key]; exists && prior != relative {
			add("workspace-resource-path", path.Join(displayRoot, relative), 1, 1, "resource collides case-insensitively with "+path.Join(displayRoot, prior))
		} else {
			folded[key] = relative
		}
		result[relative] = relative
		return nil
	})
	if err != nil {
		return nil, fmt.Errorf("inspect workspace resources: %w", err)
	}
	return result, nil
}

func validResourcePath(value string) bool {
	return value != "" && value == path.Clean(value) && value != "." && value != ".." && !strings.HasPrefix(value, "../") && !strings.HasPrefix(value, "/") && !strings.ContainsAny(value, "\\:\x00")
}

func inspectStaticResourceCalls(snapshot Snapshot, application string, resources map[string]string, add func(RuleID, string, int, int, string)) {
	loaders := map[string]bool{"LoadBitmap": true, "LoadBitmapTable": true, "LoadIntoBitmap": true, "LoadIntoBitmapTable": true, "LoadFont": true, "LoadVideo": true, "LoadSample": true, "LoadSamplePlayer": true, "LoadSoundEffect": true, "LoadFilePlayer": true, "LoadMIDI": true}
	for _, pkg := range snapshot.Loaded {
		for _, file := range pkg.Syntax {
			filename := pkg.Fset.Position(file.Pos()).Filename
			if filepath.Clean(filepath.Dir(filename)) != filepath.Clean(application) {
				continue
			}
			ast.Inspect(file, func(node ast.Node) bool {
				call, ok := node.(*ast.CallExpr)
				if !ok || len(call.Args) == 0 {
					return true
				}
				selector, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || !loaders[selector.Sel.Name] {
					return true
				}
				if object, ok := pkg.TypesInfo.Uses[selector.Sel].(*types.Func); !ok || object.Pkg() == nil || object.Pkg().Path() != gopdsdkModule+"/playdate" {
					return true
				}
				literal, ok := call.Args[0].(*ast.BasicLit)
				if !ok || literal.Kind != token.STRING {
					return true
				}
				value, err := strconv.Unquote(literal.Value)
				if err != nil || !validResourcePath(value) {
					position := pkg.Fset.Position(literal.Pos())
					add("workspace-resource-path", normalizePosition(filepath.FromSlash(snapshot.ModuleRoot), position.Filename), position.Line, position.Column, "literal SDK resource path is not a safe package-relative path")
					return true
				}
				actual, exact := matchResource(value, selector.Sel.Name, resources)
				if exact {
					return true
				}
				position := pkg.Fset.Position(literal.Pos())
				source := normalizePosition(filepath.FromSlash(snapshot.ModuleRoot), position.Filename)
				if actual != "" {
					add("workspace-missing-resource", source, position.Line, position.Column, fmt.Sprintf("resource path %q differs in case from packaged %q", value, actual))
				} else {
					add("workspace-missing-resource", source, position.Line, position.Column, fmt.Sprintf("statically named resource %q is not packaged beneath resources", value))
				}
				return true
			})
		}
	}
}

func matchResource(wanted, loader string, resources map[string]string) (string, bool) {
	for name := range resources {
		candidate := strings.TrimSuffix(name, path.Ext(name))
		if name == wanted || candidate == wanted || tableSource(candidate, wanted, loader) {
			return name, true
		}
	}
	for name := range resources {
		candidate := strings.TrimSuffix(name, path.Ext(name))
		if strings.EqualFold(name, wanted) || strings.EqualFold(candidate, wanted) || tableSource(strings.ToLower(candidate), strings.ToLower(wanted), loader) {
			return name, false
		}
	}
	return "", false
}

func tableSource(candidate, wanted, loader string) bool {
	if loader != "LoadBitmapTable" && loader != "LoadIntoBitmapTable" {
		return false
	}
	suffix := strings.TrimPrefix(candidate, wanted+"-table-")
	if suffix == candidate {
		return false
	}
	width, height, found := strings.Cut(suffix, "-")
	if !found || width == "" || height == "" {
		return false
	}
	_, widthErr := strconv.ParseUint(width, 10, 32)
	_, heightErr := strconv.ParseUint(height, 10, 32)
	return widthErr == nil && heightErr == nil
}

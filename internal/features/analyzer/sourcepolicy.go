package analyzer

import (
	"fmt"
	pathpkg "path"
	"path/filepath"
	"sort"
	"strings"
)

type GeneratedPolicy string

const (
	GeneratedExclude GeneratedPolicy = "exclude"
	GeneratedInclude GeneratedPolicy = "include"
)

func normalizeChangedFiles(source []string) ([]string, error) {
	seen := make(map[string]bool, len(source))
	result := make([]string, 0, len(source))
	for _, name := range source {
		if name == "" || name != filepath.ToSlash(name) || name != pathpkg.Clean(name) || filepath.IsAbs(name) || strings.HasPrefix(name, "/") || strings.Contains(name, ":") || name == ".." || strings.HasPrefix(name, "../") {
			return nil, fmt.Errorf("invalid module-relative changed file %q", name)
		}
		if !seen[name] {
			seen[name] = true
			result = append(result, name)
		}
	}
	sort.Strings(result)
	return result, nil
}

func changedFileSet(files []string) map[string]bool {
	if files == nil {
		return nil
	}
	result := make(map[string]bool, len(files))
	for _, name := range files {
		result[name] = true
	}
	return result
}

func filterSourceFindings(snapshot Snapshot, findings []Finding, generated GeneratedPolicy, changed map[string]bool) ([]Finding, error) {
	generatedFiles := generatedSourcePaths(snapshot)
	result := findings[:0]
	for _, finding := range findings {
		path, _, err := parsePosition(finding.Position)
		if err != nil {
			return nil, err
		}
		if changed != nil && !changed[path] {
			continue
		}
		if generatedFiles[path] {
			if generated == GeneratedExclude {
				continue
			}
			finding.Fixes = nil
		}
		result = append(result, finding)
	}
	return result, nil
}

func reportingSourcePaths(snapshot Snapshot, generated GeneratedPolicy, changed map[string]bool) map[string]bool {
	result := make(map[string]bool)
	generatedFiles := generatedSourcePaths(snapshot)
	for _, pkg := range snapshot.Packages {
		for _, file := range pkg.Files {
			if changed != nil && !changed[file.Path] {
				continue
			}
			if generated == GeneratedExclude && generatedFiles[file.Path] {
				continue
			}
			result[file.Path] = true
		}
	}
	return result
}

func generatedSourcePaths(snapshot Snapshot) map[string]bool {
	result := make(map[string]bool)
	for _, pkg := range snapshot.Packages {
		for _, file := range pkg.Files {
			result[file.Path] = sourceHasRole(file.Roles, RoleGenerated)
		}
	}
	return result
}

func sourceHasRole(roles []SourceRole, wanted SourceRole) bool {
	for _, role := range roles {
		if role == wanted {
			return true
		}
	}
	return false
}

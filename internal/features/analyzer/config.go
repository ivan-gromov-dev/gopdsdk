package analyzer

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

const (
	RepositoryConfigName   = ".gopdsdk-check.json"
	RepositoryConfigSchema = "gopdsdk-check-config/v1"
)

// RepositoryConfig supplies repository-wide check defaults. Pointer fields
// distinguish an omitted value from an explicit false or empty value.
type RepositoryConfig struct {
	Schema        string              `json:"schema"`
	Format        *string             `json:"format,omitempty"`
	Target        *string             `json:"target,omitempty"`
	BuildTags     *[]string           `json:"buildTags,omitempty"`
	Tests         *bool               `json:"tests,omitempty"`
	Patterns      *[]string           `json:"patterns,omitempty"`
	Profile       *AnalysisProfile    `json:"profile,omitempty"`
	Rules         *[]RuleID           `json:"rules,omitempty"`
	Categories    *[]RuleFamily       `json:"categories,omitempty"`
	ExcludedRules *[]RuleID           `json:"excludeRules,omitempty"`
	Severities    map[string]Severity `json:"severities,omitempty"`
	FailOn        *string             `json:"failOn,omitempty"`
	Baseline      *string             `json:"baseline,omitempty"`
	Generated     *GeneratedPolicy    `json:"generated,omitempty"`
	ChangedFiles  *[]string           `json:"changedFiles,omitempty"`
}

func loadRepositoryConfig(moduleRoot, requestedPath string, explicit bool) (RepositoryConfig, error) {
	path := requestedPath
	if path == "" {
		path = RepositoryConfigName
	}
	if !filepath.IsAbs(path) {
		path = filepath.Join(moduleRoot, path)
	}
	data, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		if errors.Is(err, os.ErrNotExist) && !explicit {
			return RepositoryConfig{}, nil
		}
		return RepositoryConfig{}, fmt.Errorf("read check configuration %q: %w", filepath.ToSlash(path), err)
	}
	decoder := json.NewDecoder(bytes.NewReader(data))
	decoder.DisallowUnknownFields()
	var config RepositoryConfig
	if err := decoder.Decode(&config); err != nil {
		return RepositoryConfig{}, fmt.Errorf("decode check configuration %q: %w", filepath.ToSlash(path), err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return RepositoryConfig{}, fmt.Errorf("decode check configuration %q: %w", filepath.ToSlash(path), err)
	}
	if err := config.Validate(); err != nil {
		return RepositoryConfig{}, fmt.Errorf("validate check configuration %q: %w", filepath.ToSlash(path), err)
	}
	return config, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return errors.New("configuration contains multiple JSON values")
}

// Validate rejects unknown schemas and values that would otherwise be ignored
// or interpreted differently by package loading.
func (config RepositoryConfig) Validate() error {
	if config.Schema != RepositoryConfigSchema {
		return fmt.Errorf("unsupported schema %q", config.Schema)
	}
	if config.Format != nil && *config.Format != "text" && *config.Format != "json" {
		return fmt.Errorf("invalid format %q", *config.Format)
	}
	if config.Target != nil {
		if _, err := checkTargets(*config.Target); err != nil {
			return err
		}
	}
	if config.BuildTags != nil {
		tags, err := normalizeBuildTags(*config.BuildTags)
		if err != nil {
			return err
		}
		*config.BuildTags = tags
	}
	if config.Patterns != nil {
		if len(*config.Patterns) == 0 {
			return errors.New("patterns must not be empty")
		}
		for _, pattern := range *config.Patterns {
			if strings.TrimSpace(pattern) == "" || pattern != strings.TrimSpace(pattern) {
				return fmt.Errorf("invalid package pattern %q", pattern)
			}
		}
	}
	if config.Profile != nil && *config.Profile != ProfileDefault && *config.Profile != ProfileExperimental && *config.Profile != ProfileDeep {
		return fmt.Errorf("invalid profile %q", *config.Profile)
	}
	if config.FailOn != nil {
		if _, err := failRank(*config.FailOn); err != nil {
			return err
		}
	}
	for selector, severity := range config.Severities {
		if selector == "" || !severity.valid() {
			return fmt.Errorf("invalid severity override %q=%q", selector, severity)
		}
	}
	if config.Baseline != nil && (strings.TrimSpace(*config.Baseline) == "" || *config.Baseline != strings.TrimSpace(*config.Baseline)) {
		return errors.New("baseline path is invalid")
	}
	if config.Generated != nil && *config.Generated != GeneratedExclude && *config.Generated != GeneratedInclude {
		return fmt.Errorf("invalid generated-source policy %q", *config.Generated)
	}
	if config.ChangedFiles != nil {
		if len(*config.ChangedFiles) == 0 {
			return errors.New("changedFiles must not be empty")
		}
		files, err := normalizeChangedFiles(*config.ChangedFiles)
		if err != nil {
			return err
		}
		*config.ChangedFiles = files
	}
	return nil
}

func normalizeBuildTags(source []string) ([]string, error) {
	seen := make(map[string]bool, len(source))
	result := make([]string, 0, len(source))
	for _, tag := range source {
		if tag == "" || strings.ContainsAny(tag, " ,\t\r\n") {
			return nil, fmt.Errorf("invalid check build tag %q", tag)
		}
		if !seen[tag] {
			seen[tag] = true
			result = append(result, tag)
		}
	}
	sort.Strings(result)
	return result, nil
}

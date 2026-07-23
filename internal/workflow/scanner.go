package workflow

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/joshuadavidthomas/gh-actionkit/internal/actions"
	"gopkg.in/yaml.v3"
)

type ScanResult struct {
	Files int
	Uses  []actions.ActionUse
}

func ScanRepository(repository string) (ScanResult, error) {
	files, err := files(repository)
	if err != nil {
		return ScanResult{}, err
	}
	result := ScanResult{Files: len(files)}
	for _, path := range files {
		uses, parseErr := scanFile(repository, path)
		if parseErr != nil {
			return ScanResult{}, parseErr
		}
		result.Uses = append(result.Uses, uses...)
	}
	return result, nil
}

func files(repository string) ([]string, error) {
	directory := filepath.Join(repository, ".github", "workflows")
	entries, err := os.ReadDir(directory)
	if os.IsNotExist(err) {
		return []string{}, nil
	}
	if err != nil {
		return nil, err
	}
	paths := make([]string, 0, len(entries))
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}
		name := entry.Name()
		if strings.HasSuffix(name, ".yml") || strings.HasSuffix(name, ".yaml") {
			paths = append(paths, filepath.Join(directory, name))
		}
	}
	sort.Strings(paths)
	return paths, nil
}

func scanFile(repository, path string) ([]actions.ActionUse, error) {
	content, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var document yaml.Node
	if err := yaml.Unmarshal(content, &document); err != nil {
		relativePath, relativeErr := filepath.Rel(repository, path)
		if relativeErr != nil {
			relativePath = path
		}
		return nil, fmt.Errorf("parse %s: %w", relativePath, err)
	}

	relativePath, err := filepath.Rel(repository, path)
	if err != nil {
		return nil, err
	}
	if len(document.Content) == 0 {
		return []actions.ActionUse{}, nil
	}
	root := document.Content[0]
	jobs := mappingValue(root, "jobs")
	if jobs == nil || jobs.Kind != yaml.MappingNode {
		return []actions.ActionUse{}, nil
	}

	var uses []actions.ActionUse
	addUse := func(node *yaml.Node) {
		if node == nil || node.Kind != yaml.ScalarNode {
			return
		}
		if use, ok := parseUse(node.Value, filepath.ToSlash(relativePath), node.Line); ok {
			uses = append(uses, use)
		}
	}
	for index := 1; index < len(jobs.Content); index += 2 {
		job := jobs.Content[index]
		if job.Kind != yaml.MappingNode {
			continue
		}
		addUse(mappingValue(job, "uses"))
		steps := mappingValue(job, "steps")
		if steps == nil || steps.Kind != yaml.SequenceNode {
			continue
		}
		for _, step := range steps.Content {
			if step.Kind == yaml.MappingNode {
				addUse(mappingValue(step, "uses"))
			}
		}
	}
	return uses, nil
}

func mappingValue(mapping *yaml.Node, key string) *yaml.Node {
	if mapping == nil || mapping.Kind != yaml.MappingNode {
		return nil
	}
	for index := 0; index+1 < len(mapping.Content); index += 2 {
		if mapping.Content[index].Value == key {
			return mapping.Content[index+1]
		}
	}
	return nil
}

func parseUse(spec, file string, line int) (actions.ActionUse, bool) {
	if strings.HasPrefix(spec, "./") || strings.HasPrefix(spec, "docker://") {
		return actions.ActionUse{}, false
	}
	separator := strings.LastIndex(spec, "@")
	if separator < 1 || separator == len(spec)-1 {
		return actions.ActionUse{}, false
	}
	action, ref := spec[:separator], spec[separator+1:]
	parts := strings.Split(action, "/")
	if len(parts) < 2 || parts[0] == "" || parts[1] == "" {
		return actions.ActionUse{}, false
	}
	return actions.ActionUse{
		Action:     action,
		Repository: actions.Repository{Owner: parts[0], Name: parts[1]},
		Ref:        ref,
		Location:   actions.Location{File: file, Line: line},
	}, true
}

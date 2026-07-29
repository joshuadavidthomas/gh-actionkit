package actions

import (
	"fmt"
	"strings"
)

// ActionIdentifier identifies a GitHub Action repository and an optional
// directory containing its action manifest.
type ActionIdentifier struct {
	repository Repository
	path       string
}

func ParseActionIdentifier(value string) (ActionIdentifier, error) {
	segments := strings.Split(value, "/")
	if len(segments) < 2 {
		return ActionIdentifier{}, fmt.Errorf("invalid action %q: expected owner/repo[/path]", value)
	}
	for _, segment := range segments {
		if segment == "" {
			return ActionIdentifier{}, fmt.Errorf("invalid action %q: path segments must be nonempty", value)
		}
	}

	return ActionIdentifier{
		repository: Repository{Owner: segments[0], Name: segments[1]},
		path:       strings.Join(segments[2:], "/"),
	}, nil
}

func ParseRepository(value string) (Repository, error) {
	identifier, err := ParseActionIdentifier(value)
	if err != nil || identifier.Path() != "" {
		return Repository{}, fmt.Errorf("invalid repository %q: expected owner/repo", value)
	}
	return identifier.Repository(), nil
}

func (i ActionIdentifier) Repository() Repository {
	return i.repository
}

func (i ActionIdentifier) WithRepository(repository Repository) ActionIdentifier {
	i.repository = repository
	return i
}

func (i ActionIdentifier) Path() string {
	return i.path
}

func (i ActionIdentifier) String() string {
	value := i.repository.Owner + "/" + i.repository.Name
	if i.path != "" {
		value += "/" + i.path
	}
	return value
}

func (i ActionIdentifier) ManifestCandidates() (yml string, yaml string) {
	if i.path == "" {
		return "action.yml", "action.yaml"
	}
	return i.path + "/action.yml", i.path + "/action.yaml"
}

package actions

import "testing"

func TestParseActionIdentifier(t *testing.T) {
	tests := []struct {
		value      string
		repository Repository
		path       string
		yml        string
		yaml       string
	}{
		{
			value:      "Actions/Checkout",
			repository: Repository{Owner: "Actions", Name: "Checkout"},
			yml:        "action.yml",
			yaml:       "action.yaml",
		},
		{
			value:      "github/codeql-action/init",
			repository: Repository{Owner: "github", Name: "codeql-action"},
			path:       "init",
			yml:        "init/action.yml",
			yaml:       "init/action.yaml",
		},
		{
			value:      "owner/repo/a/../b%2Fc",
			repository: Repository{Owner: "owner", Name: "repo"},
			path:       "a/../b%2Fc",
			yml:        "a/../b%2Fc/action.yml",
			yaml:       "a/../b%2Fc/action.yaml",
		},
	}

	for _, test := range tests {
		t.Run(test.value, func(t *testing.T) {
			identifier, err := ParseActionIdentifier(test.value)
			if err != nil {
				t.Fatal(err)
			}
			if identifier.Repository() != test.repository || identifier.Path() != test.path ||
				identifier.String() != test.value {
				t.Fatalf("unexpected identifier: %#v", identifier)
			}
			yml, yaml := identifier.ManifestCandidates()
			if yml != test.yml || yaml != test.yaml {
				t.Fatalf("manifest candidates = %q, %q", yml, yaml)
			}
		})
	}
}

func TestActionIdentifierWithRepositoryPreservesPath(t *testing.T) {
	identifier := mustParseIdentifier(t, "Owner/Repo/sub/action")
	canonical := identifier.WithRepository(Repository{Owner: "owner", Name: "repo"})

	if canonical.String() != "owner/repo/sub/action" || canonical.Path() != "sub/action" {
		t.Fatalf("unexpected canonical identifier: %#v", canonical)
	}
	if identifier.String() != "Owner/Repo/sub/action" {
		t.Fatalf("original identifier changed: %#v", identifier)
	}
}

func TestParseRepository(t *testing.T) {
	repository, err := ParseRepository("Actions/Checkout")
	if err != nil {
		t.Fatal(err)
	}
	if repository != (Repository{Owner: "Actions", Name: "Checkout"}) {
		t.Fatalf("unexpected repository: %#v", repository)
	}
}

func TestParseRepositoryRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{"", "owner", "/repo", "owner/", "owner/repo/path"} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseRepository(value); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func TestParseActionIdentifierRejectsMalformedValues(t *testing.T) {
	for _, value := range []string{
		"",
		"owner",
		"/repo",
		"owner/",
		"owner//path",
		"owner/repo/",
		"owner/repo//path",
	} {
		t.Run(value, func(t *testing.T) {
			if _, err := ParseActionIdentifier(value); err == nil {
				t.Fatal("expected an error")
			}
		})
	}
}

func mustParseIdentifier(t testing.TB, value string) ActionIdentifier {
	t.Helper()
	identifier, err := ParseActionIdentifier(value)
	if err != nil {
		t.Fatal(err)
	}
	return identifier
}

package cli

import (
	"fmt"
	"os"
	"path/filepath"
)

func resolveRepository(path string) (string, error) {
	absolutePath, err := filepath.Abs(path)
	if err != nil {
		return "", fmt.Errorf("resolve repository path %q: %w", path, err)
	}
	info, err := os.Stat(absolutePath)
	if err != nil {
		return "", fmt.Errorf("open repository path %q: %w", path, err)
	}
	if !info.IsDir() {
		return "", fmt.Errorf("repository path %q is not a directory", path)
	}
	return absolutePath, nil
}

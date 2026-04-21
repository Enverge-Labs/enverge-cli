package cloudflared

import (
	"fmt"
	"os"
	"path/filepath"
)

// BinPath returns the path to the cloudflared binary: ~/.config/enverge/bin/cloudflared
func BinPath() (string, error) {
	home, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(home, ".config", "enverge", "bin", "cloudflared"), nil
}

// EnsureInstalled checks if cloudflared exists at BinPath. If not, returns an error
// with instructions to install it manually.
func EnsureInstalled() (string, error) {
	path, err := BinPath()
	if err != nil {
		return "", fmt.Errorf("could not determine cloudflared path: %w", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		return "", fmt.Errorf("cloudflared not found at %s. Install it with: brew install cloudflared", path)
	} else if err != nil {
		return "", fmt.Errorf("could not check cloudflared at %s: %w", path, err)
	}

	return path, nil
}

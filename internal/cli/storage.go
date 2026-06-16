package cli

import (
	"os"
	"path/filepath"
)

func configDir() (string, error) {
	dir, err := os.UserConfigDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, "mcquery"), nil
}

func configFile(name string) (string, error) {
	dir, err := configDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(dir, name), nil
}

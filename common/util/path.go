package util

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/mitchellh/go-homedir"
)

func GetRootPath(path string) (string, error) {
	path = strings.Trim(path, " ")
	if path == "" {
		path = "."
	}
	fullPath, err := homedir.Expand(path)
	if err != nil {
		return fullPath, err
	}
	if !filepath.IsAbs(fullPath) {
		defaultRootPath, err := os.Getwd()
		if err != nil {
			return fullPath, err
		}
		fullPath = filepath.Join(defaultRootPath, fullPath)
	}
	if !strings.HasSuffix(fullPath, string(os.PathSeparator)) {
		fullPath += string(os.PathSeparator)
	}
	return fullPath, nil
}

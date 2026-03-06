package options

import (
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

func ParseConf() (ProjectOptions, error) {
	content, err := loadFileContent()
	if err != nil {
		return ProjectOptions{}, err
	}

	opt := ProjectOptions{}
	err = yaml.Unmarshal(content, &opt)
	return opt, err
}

func loadFileContent() ([]byte, error) {
	searchDirs := []string{
		"etc",
		filepath.Join(os.TempDir(), "etc"),
	}

	for _, dir := range searchDirs {
		matches, err := searchConfFiles(dir)
		if err != nil {
			return nil, err
		}
		if len(matches) == 0 {
			continue
		}

		content, err := os.ReadFile(matches[0])
		if err != nil {
			return nil, err
		}
		return content, nil
	}

	return nil, fmt.Errorf("no configuration file found in ./etc or %s", filepath.Join(os.TempDir(), "etc"))
}

func searchConfFiles(dir string) ([]string, error) {
	patterns := []string{"*.yml", "*.yaml", "*.yml*", "*.yaml*"}
	var matches []string
	for _, pattern := range patterns {
		found, err := filepath.Glob(filepath.Join(dir, pattern))
		if err != nil {
			return nil, err
		}
		matches = append(matches, found...)
	}
	return matches, nil
}

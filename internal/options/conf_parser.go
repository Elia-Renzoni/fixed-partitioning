package options

import (
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
	err = yaml.Unmarshal(content, opt)
	return opt, err
}

func loadFileContent() ([]byte, error) {
	var file *os.File
	err := os.Chdir("etc/")
	if err != nil {
		return nil, err
	}

	var match string
	match, err = searchConfFile()
	if err != nil {
		return nil, err
	}

	file, err = os.OpenFile(match, os.O_RDONLY, 0644)
	if err != nil {
		return nil, err
	}

	buffer := make([]byte, 5048)
	_, err = file.Read(buffer)
	if err != nil {
		return nil, err
	}

	return buffer, nil
}

func searchConfFile() (string, error) {
	matches, err := filepath.Glob(".yaml")
	if err != nil {
		return "", err
	}
	return matches[0], nil
}

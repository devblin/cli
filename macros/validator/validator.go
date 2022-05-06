package validator

import (
	"bytes"
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	validate "github.com/go-playground/validator/v10"
	"github.com/spf13/viper"
)

// Checks if the analyzer.toml file and the issue directory is present
func CheckForAnalyzerConfig(analyzerTOMLPath, issuesDirectoryPath string) (err error) {
	// Check if `analyzer.toml` is present in `.deepsource/analyzer` folder
	if _, err := os.Stat(analyzerTOMLPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("the analyzer.toml file doesn't exist")
		}
	}

	// Check if `issues/` directory is present in `.deepsource/analyzer` folder and is not empty.
	if _, err := os.Stat(issuesDirectoryPath); err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return errors.New("the issue descriptions directory doesn't exist")
		}
	}
	return
}

// Validates analyzer.toml file
func ValidateAnalyzerTOML(analyzerTOMLPath string) (analyzerConfig AnalyzerMetadata, err error) {
	config := AnalyzerMetadata{}
	// Read the contents of analyzer.toml file
	analyzerTOMLContent, err := ioutil.ReadFile(analyzerTOMLPath)
	if err != nil {
		return config, errors.New("failed to read analyzer.toml file")
	}

	viper.SetConfigType("toml")
	if err = viper.ReadConfig(bytes.NewBuffer(analyzerTOMLContent)); err != nil {
		return config, err
	}
	// Unmarshaling the configdata into AnalyzerMetadata struct
	viper.Unmarshal(&config)

	// Validate analyzer.toml fields based on type and sanity checks
	v := validate.New()
	if err := v.Struct(&config); err != nil {
		missingRequiredFields := formatValidationErrors(err, config)
		missingFields := strings.Join(missingRequiredFields, ", ")
		// Improve error message returned by `go-playground/validator`
		return config, fmt.Errorf("missing the following required fields from analyzer.toml: %v\n", missingFields)
	}
	return config, nil
}

// Validates issue description TOML files
func ValidateIssueDescriptions(issuesDirectoryPath string) (err error) {
	validationFailed := false
	issuesList, err := ioutil.ReadDir(issuesDirectoryPath)
	if err != nil {
		return err
	}

	for _, issuePath := range issuesList {
		config := AnalyzerIssue{}

		// Read the contents of issue toml file
		issueTOMLContent, err := ioutil.ReadFile(filepath.Join(issuesDirectoryPath, issuePath.Name()))
		if err != nil {
			return fmt.Errorf("failed to read file: %s", filepath.Join(issuesDirectoryPath, issuePath.Name()))
		}
		viper.SetConfigType("toml")
		if err = viper.ReadConfig(bytes.NewBuffer(issueTOMLContent)); err != nil {
			return err
		}
		// Unmarshaling the configdata into AnalyzerMetadata struct
		viper.Unmarshal(&config)

		// Validate the data
		v := validate.New()
		if err := v.Struct(&config); err != nil {
			validationFailed = true
			missingRequiredFields := formatValidationErrors(err, config)
			missingFields := strings.Join(missingRequiredFields, ", ")
			fmt.Printf("Missing the following required fields from issue %s: %v\n", issuePath.Name(), missingFields)
		}
	}

	if validationFailed {
		return fmt.Errorf("found the above validation errors in issue descriptions")
	}
	return nil
}

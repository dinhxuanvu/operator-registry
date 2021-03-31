package config

import (
	"github.com/operator-framework/operator-registry/pkg/declcfg"
)

// ValidateConfig takes a directory containing the declarative config file(s)
// 1. Validate if declarative config file(s) are valid based on specified schema
// 2. Validate the `replaces` chains of the upgrade graph
// Inputs:
// directory: the directory where declarative config file(s) exist
// Outputs:
// error: ValidattionError which contains a list of errors
func ValidateConfig(directory string) error {
	// Load config files
	cfg, err := declcfg.LoadDir(directory)
	if err != nil {
		return err
	}
	// Validate the config using model validation
	_, err = declcfg.ConvertToModel(*cfg)
	if err != nil {
		return err
	}
	return nil
}

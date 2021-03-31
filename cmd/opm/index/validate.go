package index

import (
	"os"

	"github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/operator-framework/operator-registry/pkg/lib/config"
)

func newConfigValidateCmd() *cobra.Command {
	validate := &cobra.Command{
		Use:   "validate <directory>",
		Short: "Validate the declarative index config",
		Long:  "Validate the declarative config JSON file(s) in a given directory",
		Args: func(cmd *cobra.Command, args []string) error {
			return cobra.ExactArgs(1)(cmd, args)
		},
		RunE: configValidate,
	}

	return validate
}

func configValidate(cmd *cobra.Command, args []string) error {
	logger := logrus.WithField("cmd", "validate")

	if _, err := os.Stat(args[0]); os.IsNotExist(err) {
		logger.Error(err.Error())
	}

	return config.ValidateConfig(args[0])
}

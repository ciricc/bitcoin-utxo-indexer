package config

import (
	"fmt"
	"strings"

	"dario.cat/mergo"
	"github.com/iancoleman/strcase"
	"github.com/knadh/koanf"
	"github.com/knadh/koanf/parsers/yaml"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
)

const (
	Delimiter = "__"
	EnvPrefix = "SERVICE__"
)

type ServiceConfig interface {
	Validate() error
}

// LoadServiceConfig initializes a ServiceConfig instance with settings from a specified YAML config file and environment variables.
// - `config` must be a pointer to a ServiceConfig instance where the loaded configuration will be stored.
// - `filePath` specifies the path to the YAML configuration file. If `filePath` is empty, loading from the file is skipped.
// - Configuration values are first loaded from the YAML file (if provided) and then overridden by environment variables that match the prefix and structure defined by EnvPrefix.
// Environment variable names are converted to lowerCamelCase to match the YAML file keys.
// - After loading the configurations, it validates the final configuration structure using the `Validate` method on the ServiceConfig instance.
// - Returns an error if reading from the file, environment variables, unmarshaling, or validation fails.
func LoadServiceConfig[SC ServiceConfig](config SC, filePath string) error {
	k := koanf.New(".")

	if len(filePath) > 0 {
		if err := k.Load(file.Provider(filePath), yaml.Parser()); err != nil {
			return fmt.Errorf("failed to read config with path %s: %w", filePath, err)
		}
	}

	if err := k.Load(env.Provider(EnvPrefix, Delimiter, func(s string) string {
		s = strings.TrimPrefix(s, EnvPrefix)
		nested := strings.Split(s, Delimiter)
		for index := 0; index < len(nested); index++ {
			nested[index] = strcase.ToLowerCamel(nested[index])
		}
		return strings.Join(nested, Delimiter)
	}), nil, koanf.WithMergeFunc(func(src, dest map[string]interface{}) error {
		return mergo.Merge(&dest, src, mergo.WithOverride)
	})); err != nil {
		return fmt.Errorf("failed to read environment variables: %w", err)
	}

	if err := k.Unmarshal("", config); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	if err := config.Validate(); err != nil {
		return fmt.Errorf("failed to validate config: %w", err)
	}

	return nil
}

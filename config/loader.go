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

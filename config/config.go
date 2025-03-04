package config

import (
	"fmt"
	"strings"

	"github.com/knadh/koanf/parsers/toml/v2"
	"github.com/knadh/koanf/providers/confmap"
	"github.com/knadh/koanf/providers/env"
	"github.com/knadh/koanf/providers/file"
	"github.com/knadh/koanf/v2"
)

// Global koanf instance, using "." as the key path delimiter.
var k = koanf.New(".")

type Config struct {
	Camera struct {
		Protocol string `koanf:"protocol"`
		IP       string `koanf:"ip"`
	} `koanf:"camera"`
}

func Init(cfgFile string, flags map[string]any) error {
	// 1. Load default values (lowest priority)
	if err := loadDefaults(); err != nil {
		return err
	}

	// 2. Load configuration file
	if cfgFile != "" {
		if err := loadFile(cfgFile); err != nil {
			return err
		}
	}

	// 3. Load environment variables
	if err := loadEnv(); err != nil {
		return err
	}

	// 4. Apply command line flag overrides (highest priority)
	_, err := ApplyFlags(flags)
	return err
}

func loadDefaults() error {
	defaults := map[string]any{
		"camera.ip":       "auto",
		"camera.protocol": "http",
	}

	return k.Load(confmap.Provider(defaults, "."), nil)
}

func loadFile(cfgFile string) error {
	return k.Load(file.Provider(cfgFile), toml.Parser())
}

func loadEnv() error {
	return k.Load(env.Provider("HEROSYNC_", ".", func(s string) string {
		return strings.Replace(strings.ToLower(
			strings.TrimPrefix(s, "HEROSYNC_")), "_", ".", -1)
	}), nil)
}

func ApplyFlags(flags map[string]any) (*Config, error) {
	if err := k.Load(confmap.Provider(flags, "-"), nil); err != nil {
		return nil, err
	}
	return GetConfig()
}

// GetConfig returns the parsed configuration
func GetConfig() (*Config, error) {
	var cfg Config
	if err := k.Unmarshal("", &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %v", err)
	}
	if err := validateConfig(&cfg); err != nil {
		return nil, err
	}
	return &cfg, nil
}

func validateConfig(cfg *Config) error {
	if cfg.Camera.Protocol != "http" && cfg.Camera.Protocol != "https" {
		return fmt.Errorf("invalid protocol: %s", cfg.Camera.Protocol)
	}
	return nil
}

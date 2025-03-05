package config

import (
	"fmt"
	"net/url"
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
	GoPro struct {
		Host   string `koanf:"host"`
		Scheme string `koanf:"scheme"`
	} `koanf:"gopro"`
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

	// 4. Load command line flags (highest priority)
	if err := LoadFlags(flags); err != nil {
		return err
	}

	return nil
}

func loadDefaults() error {
	defaults := map[string]any{
		"gopro.host":   "", // empty means use mDNS discovery
		"gopro.scheme": "http",
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

func LoadFlags(flags map[string]any) error {
	return k.Load(confmap.Provider(flags, "-"), nil)
}

// Get the current configuration state.
func Get() (*Config, error) {
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
	if cfg.GoPro.Scheme != "http" && cfg.GoPro.Scheme != "https" {
		return fmt.Errorf("invalid scheme: %s; choose http or https", cfg.GoPro.Scheme)
	}
	return nil
}

func (c *Config) GetGoProURL() (*url.URL, error) {
	return resolveGoPro(c.GoPro.Host, c.GoPro.Scheme)
}

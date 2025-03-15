package config

import (
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"github.com/adrg/xdg"
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
	Group struct {
		By string `koanf:"by"`
	} `koanf:"group"`
	Log struct {
		Level string `koanf:"level"`
	} `koanf:"log"`
	Media struct {
		Dir string `koanf:"dir"`
	} `koanf:"media"`
}

// DefaultConfigPath returns the default config file path following XDG specification.
func DefaultConfigPath() string {
	return filepath.Join(xdg.ConfigHome, "herosync", "config.toml")
}

// DefaultMediaDir returns the default media files directory following XDG specification.
func DefaultMediaDir() string {
	return filepath.Join(xdg.DataHome, "herosync", "media")
}

func Init(configFile string, flags map[string]any) error {
	// 1. Load default values (lowest priority)
	if err := loadDefaults(); err != nil {
		return err
	}

	// 2. Load configuration file
	if err := loadFile(configFile); err != nil {
		return err
	}

	// 3. Load environment variables
	if err := loadEnv(); err != nil {
		return err
	}

	// 4. Load global command line flags (highest priority)
	if err := LoadFlags(flags); err != nil {
		return err
	}

	return nil
}

func loadDefaults() error {
	defaults := map[string]any{
		"gopro.host":   "", // Empty means use mDNS discovery
		"gopro.scheme": "http",
		"group.by":     "chapters",
		"log.level":    "info",
		"media.dir":    DefaultMediaDir(),
	}
	return k.Load(confmap.Provider(defaults, "."), nil)
}

func loadFile(configFile string) error {
	if _, err := os.Stat(configFile); os.IsNotExist(err) {
		// Config file doesn't exist, that's okay - just skip it
		return nil
	}
	return k.Load(file.Provider(configFile), toml.Parser())
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
	switch cfg.GoPro.Scheme {
	case "http", "https":
		// valid
	default:
		return fmt.Errorf("invalid scheme: %q (choose http or https)", cfg.GoPro.Scheme)
	}

	switch cfg.Group.By {
	case "chapters", "date":
		// valid
	default:
		return fmt.Errorf("invalid grouping: %q (choose chapters or date)", cfg.Group.By)
	}

	// Try unmarshalling the log level to validate it.
	var level slog.Level
	if err := level.UnmarshalText([]byte(cfg.Log.Level)); err != nil {
		return fmt.Errorf("invalid log level: %s", cfg.Log.Level)
	}

	return nil
}

// IncomingMediaDir returns the full path to the incoming media directory.
func (c *Config) IncomingMediaDir() string {
	return filepath.Join(c.Media.Dir, "incoming")
}

// OutgoingMediaDir returns the full path to the outgoing media directory.
func (c *Config) OutgoingMediaDir() string {
	return filepath.Join(c.Media.Dir, "outgoing")
}

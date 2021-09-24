package config

import (
	"io/ioutil"

	"github.com/gravitational/configure"

	"github.com/gravitational/trace"
)

// Config represents a process configuration
type Config struct {
	// Debug is debug flag
	Debug bool `yaml:"debug" env:"DEBUG"`
	// WorkingDir is the working directory to store state
	WorkingDir string `yaml:"workingDir"`
	// Addr is the daemon network address
	Addr string `yaml:"addr"`
}

// CheckAndSetDefaults checks and sets the default values
func (c *Config) CheckAndSetDefaults() error {
	if c.WorkingDir == "" {
		return trace.BadParameter("missing workingDir")
	}

	if c.Addr == "" {
		return trace.BadParameter("missing addr")
	}

	return nil
}

// New returns config object read from the specified path,
// fields read from env vars override fields in the config files
// and missing fields are filled with defaults
func New(path string) (Config, error) {
	var cfg Config
	if path != "" {
		bytes, err := ioutil.ReadFile(path)
		if err != nil {
			return cfg, trace.Wrap(err)
		}
		err = configure.ParseYAML(bytes, &cfg)
		if err != nil {
			return cfg, trace.Wrap(err)
		}
	}
	err := configure.ParseEnv(&cfg)
	if err != nil {
		return cfg, trace.Wrap(err)
	}

	err = cfg.CheckAndSetDefaults()
	if err != nil {
		return cfg, trace.Wrap(err)
	}

	return cfg, nil
}

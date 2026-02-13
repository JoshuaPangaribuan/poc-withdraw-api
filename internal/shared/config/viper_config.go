package config

import (
	"fmt"
	"os"
	"sync"
	"time"

	"github.com/fsnotify/fsnotify"
	"github.com/spf13/viper"
)

var _ ConfigProvider = (*viperConfig)(nil)

type viperConfig struct {
	v         *viper.Viper
	source    string
	callbacks []func()
	mu        sync.RWMutex
	done      chan struct{}
}

// Init loads configuration from a YAML file (primary) or .env file (exclusive fallback).
// Returns a ConfigProvider interface. Returns error if neither file exists or parsing fails.
func Init(opts Options) (ConfigProvider, error) {
	v := viper.New()
	cfg := &viperConfig{
		v:    v,
		done: make(chan struct{}),
	}

	yamlExists := fileExists(opts.YAMLPath)
	envExists := fileExists(opts.EnvPath)

	switch {
	case yamlExists:
		v.SetConfigFile(opts.YAMLPath)
		v.SetConfigType("yaml")
		cfg.source = "yaml"
	case envExists:
		v.SetConfigFile(opts.EnvPath)
		v.SetConfigType("env")
		cfg.source = "env"
	default:
		return nil, fmt.Errorf("config: no config file found (tried %q and %q)", opts.YAMLPath, opts.EnvPath)
	}

	if err := v.ReadInConfig(); err != nil {
		return nil, fmt.Errorf("config: failed to read %s file: %w", cfg.source, err)
	}

	return cfg, nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	return err == nil && !info.IsDir()
}

func (c *viperConfig) GetString(key string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetString(key)
}

func (c *viperConfig) GetInt(key string) int {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetInt(key)
}

func (c *viperConfig) GetBool(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetBool(key)
}

func (c *viperConfig) GetDuration(key string) time.Duration {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetDuration(key)
}

func (c *viperConfig) GetFloat64(key string) float64 {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetFloat64(key)
}

func (c *viperConfig) GetStringSlice(key string) []string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetStringSlice(key)
}

func (c *viperConfig) GetStringMap(key string) map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.GetStringMap(key)
}

func (c *viperConfig) IsSet(key string) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.IsSet(key)
}

func (c *viperConfig) AllSettings() map[string]interface{} {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.v.AllSettings()
}

func (c *viperConfig) Source() string { return c.source }

func (c *viperConfig) OnChange(fn func()) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.callbacks = append(c.callbacks, fn)
}

func (c *viperConfig) WatchChanges() {
	if c.source != "yaml" {
		return
	}

	c.v.OnConfigChange(func(e fsnotify.Event) {
		c.mu.Lock()
		c.v.ReadInConfig()
		cbs := make([]func(), len(c.callbacks))
		copy(cbs, c.callbacks)
		c.mu.Unlock()

		for _, fn := range cbs {
			fn()
		}
	})
	c.v.WatchConfig()
}

func (c *viperConfig) StopWatching() {
	select {
	case <-c.done:
	default:
		close(c.done)
	}
}

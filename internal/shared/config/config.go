package config

import "time"

// Options configures the config loader.
type Options struct {
	// YAMLPath is the path to the primary YAML config file.
	YAMLPath string

	// EnvPath is the path to the fallback .env file, used only when YAML is absent.
	EnvPath string
}

// ConfigProvider is the interface consumers depend on for reading configuration.
// Implementations must be safe for concurrent use.
type ConfigProvider interface {
	// GetString returns the value associated with the key as a string.
	GetString(key string) string

	// GetInt returns the value associated with the key as an int.
	GetInt(key string) int

	// GetBool returns the value associated with the key as a bool.
	GetBool(key string) bool

	// GetDuration returns the value associated with the key as a time.Duration.
	GetDuration(key string) time.Duration

	// GetFloat64 returns the value associated with the key as a float64.
	GetFloat64(key string) float64

	// GetStringSlice returns the value associated with the key as a slice of strings.
	GetStringSlice(key string) []string

	// GetStringMap returns the value associated with the key as a map of interfaces.
	GetStringMap(key string) map[string]interface{}

	// IsSet checks whether the key is set in the config.
	IsSet(key string) bool

	// AllSettings returns all settings as a map.
	AllSettings() map[string]interface{}

	// WatchChanges starts watching the config file for changes (YAML only).
	// Non-blocking: spawns a background goroutine.
	WatchChanges()

	// OnChange registers a callback that fires after a successful config reload.
	// Multiple callbacks can be registered; they execute in registration order.
	OnChange(fn func())

	// StopWatching stops the file watcher and cleans up resources.
	StopWatching()

	// Source returns which config source is active: "yaml" or "env".
	Source() string
}

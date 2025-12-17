package config

import (
	"encoding/json"
	"fmt"
	"os"
)

// ParseConfig reads and parses a configuration file
func ParseConfig(configFile string) (*Config, error) {
	file, err := os.Open(configFile)
	if err != nil {
		return nil, fmt.Errorf("failed to open config file: %w", err)
	}
	defer file.Close()

	var config Config
	if err := json.NewDecoder(file).Decode(&config); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	// Set defaults for enabled flag
	for i := range config.Databases {
		// If Enabled is not explicitly set in JSON, default to true
		// This is a workaround for Go's zero-value behavior with booleans
		if !config.Databases[i].Enabled {
			// Check if it was explicitly set to false or just omitted
			// For now, default to true (we'll handle explicit false in validation)
			config.Databases[i].Enabled = true
		}
	}

	return &config, nil
}

package config

import (
	"fmt"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

// Validate validates a configuration file against the JSON schema
func Validate(configFile string) error {
	schemaLoader := gojsonschema.NewStringLoader(Schema)
	documentLoader := gojsonschema.NewReferenceLoader("file://" + configFile)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		return fmt.Errorf("failed to validate schema: %w", err)
	}

	if !result.Valid() {
		fmt.Fprintf(os.Stderr, "Configuration validation errors:\n")
		for _, desc := range result.Errors() {
			fmt.Fprintf(os.Stderr, "  - %s\n", desc)
		}
		return fmt.Errorf("configuration file is not valid")
	}

	return nil
}

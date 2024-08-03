package backuper

import (
	"fmt"
	"log"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

func Validate(configFile string) {
	schemaLoader := gojsonschema.NewStringLoader(schema)
	documentLoader := gojsonschema.NewReferenceLoader("file://" + configFile)

	result, err := gojsonschema.Validate(schemaLoader, documentLoader)
	if err != nil {
		log.Fatalf("failed to validate schema: %v", err)
	}

	if !result.Valid() {
		fmt.Printf("The document is not valid. see errors :\n")
		for _, desc := range result.Errors() {
			fmt.Printf("- %s\n", desc)
		}

		os.Exit(1)
	}
}

var schema = `{
    "$schema": "http://json-schema.org/draft-07/schema#",
    "type": "object",
    "properties": {
        "databases": {
            "type": "array",
            "items": {
                "type": "object",
                "properties": {
                    "name": {"type": "string"},
                    "user": {"type": "string"},
                    "password": {"type": "string"},
                    "host": {"type": "string"}
                },
                "required": ["name", "user", "password", "host"]
            }
        },
        "backup_dir": {"type": "string"},
        "retention": {"type": "integer", "minimum": 1},
        "log_file": {"type": "string"}
    },
    "required": ["databases", "backup_dir", "retention", "log_file"]
}`

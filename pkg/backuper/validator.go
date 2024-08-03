package backuper

import (
	"fmt"
	"log"
	"os"

	"github.com/xeipuuv/gojsonschema"
)

func Validate(schemaFile, configFile string) {
	schemaLoader := gojsonschema.NewReferenceLoader("file://" + schemaFile)
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


package main

import (
	"fmt"

	"github.com/blues/note-go/note"
	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader" // Enable HTTP/HTTPS loading
)

func validateRequest(requestJSON []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	schemaURL := "https://raw.githubusercontent.com/blues/notecard-schema/master/notecard.api.json"
	schema, err := compiler.Compile(schemaURL)
	if err != nil {
		return fmt.Errorf("failed to compile schema: %v (use -force to bypass validation)", err)
	}

	var reqMap map[string]interface{}
	if err = note.JSONUnmarshal(requestJSON, &reqMap); err != nil {
		return fmt.Errorf("failed to parse request for validation: %v (use -force to bypass validation)", err)
	}

	if err = schema.Validate(reqMap); err != nil {
		return fmt.Errorf("validation failed: %v (use -force to bypass validation)", err)
	}

	return nil
}

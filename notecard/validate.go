
package main

import (
	"fmt"
	"net/http"

	"github.com/blues/note-go/note"
	"github.com/santhosh-tekuri/jsonschema/v5"
)

func validateRequest(requestJSON []byte) error {
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020

	schemaURL := "https://raw.githubusercontent.com/blues/notecard-schema/master/notecard.api.json"
	resp, err := http.Get(schemaURL)
	if err != nil {
		return fmt.Errorf("failed to fetch schema: %v (use -force to bypass validation)", err)
	}
	defer resp.Body.Close()

	if err = compiler.AddResource(schemaURL, resp.Body); err != nil {
		return fmt.Errorf("failed to add schema resource: %v (use -force to bypass validation)", err)
	}

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

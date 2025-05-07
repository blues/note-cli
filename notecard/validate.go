package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v5"
	_ "github.com/santhosh-tekuri/jsonschema/v5/httploader" // Enable HTTP/HTTPS loading
)

// schema is a cached, compiled JSON schema
var (
	schema     *jsonschema.Schema
	schemaOnce sync.Once
	schemaErr  error
)

// cacheDir is the directory where schemas are stored
const cacheDir = "/tmp/notecard-schema/"

// extractRefs recursively extracts $ref URLs from a schema
func extractRefs(schema map[string]interface{}, baseURL string) []string {
	var refs []string
	if ref, ok := schema["$ref"].(string); ok && strings.HasPrefix(ref, "http") {
		refs = append(refs, ref)
	}
	for _, v := range schema {
		switch v := v.(type) {
		case map[string]interface{}:
			refs = append(refs, extractRefs(v, baseURL)...)
		case []interface{}:
			for _, item := range v {
				if m, ok := item.(map[string]interface{}); ok {
					refs = append(refs, extractRefs(m, baseURL)...)
				}
			}
		}
	}
	return refs
}

// fetchAndCacheSchema fetches a schema from the URL and caches it
func fetchAndCacheSchema(url string, verbose bool) (io.Reader, error) {
	// Fetch the schema
	if verbose {
		fmt.Fprintf(os.Stderr, "*** fetching schema: %s ***\n", url)
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch schema %s: %v", url, err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("failed to fetch schema %s: status %d", url, resp.StatusCode)
	}
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read schema %s: %v", url, err)
	}
	// Verify it's valid JSON before caching
	var v interface{}
	if err := json.Unmarshal(data, &v); err != nil {
		return nil, fmt.Errorf("invalid JSON schema %s: %v", url, err)
	}
	// Save to cache
	cachePath := getCachePath(url)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		// Log error but continue
		if verbose {
			fmt.Fprintf(os.Stderr, "failed to cache schema %s: %v\n", url, err)
		}
	}
	return bytes.NewReader(data), nil
}

func formatErrorMessage(reqType string, errUnformatted error) (err error) {
	// Check if the error is nil
	if errUnformatted == nil {
		return nil
	}

	// Convert the error to a string
	errMsg := errUnformatted.Error()

	// Define constants
	const prefix = "jsonschema: '"
	const mid1 = "' does not validate with "
	const mid2 = ": "

	// Check if message starts with prefix
	if !strings.HasPrefix(errMsg, prefix) {
		return fmt.Errorf("invalid error message format")
	}

	// Remove prefix and split on mid1
	rest := strings.TrimPrefix(errMsg, prefix)
	parts := strings.SplitN(rest, mid1, 2)
	if len(parts) != 2 {
		return fmt.Errorf("invalid error message format")
	}

	// Extract property and remaining part
	property := parts[0]
	if len(property) > 0 {
		// As of jsonschema v5.3.1, a forward-slash is prefixed to the
		// property name. Remove it to improve readability.
		// Workaround for issue:
		// https://github.com/santhosh-tekuri/jsonschema/issues/220
		property = parts[0][1:]
	}
	remaining := parts[1]

	// Split remaining part on mid2
	finalParts := strings.SplitN(remaining, mid2, 2)
	if len(finalParts) != 2 {
		return fmt.Errorf("invalid error message format")
	}

	// Extract schema rule and error message
	// schemaRule := finalParts[0] // Not used in output, but available if needed
	errorMessage := finalParts[1]

	// Format the new error message
	if len(property) > 0 {
		err = fmt.Errorf("'%s' is not valid for %s: %s", property, reqType, errorMessage)
	} else {
		err = fmt.Errorf("for '%s' %s", reqType, errorMessage)
	}

	// Return the formatted error
	return err
}

// getCachePath converts a URL to a safe file path in the cache directory
func getCachePath(url string) string {
	// Use the URL path as the filename, replacing invalid characters
	filename := strings.ReplaceAll(filepath.Base(url), string(os.PathSeparator), "_")
	return filepath.Join(cacheDir, filename)
}

// initSchema compiles the schema, using cached files if available
func initSchema(url string, verbose bool) error {
	schemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft2020

		// Ensure cache directory exists
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			schemaErr = fmt.Errorf("failed to create cache directory %s: %v", cacheDir, err)
			return
		}

		// Load main schema
		mainSchemaReader, err := loadOrFetchSchema(url, verbose)
		if err != nil {
			schemaErr = fmt.Errorf("failed to load main schema %s: %v", url, err)
			return
		}
		// Read main schema to extract $ref URLs
		mainSchemaData, err := io.ReadAll(mainSchemaReader)
		if err != nil {
			schemaErr = fmt.Errorf("failed to read main schema %s: %v", url, err)
			return
		}
		var mainSchema map[string]interface{}
		if err := json.Unmarshal(mainSchemaData, &mainSchema); err != nil {
			schemaErr = fmt.Errorf("failed to parse main schema %s: %v", url, err)
			return
		}
		// Add main schema resource
		if err := compiler.AddResource(url, bytes.NewReader(mainSchemaData)); err != nil {
			schemaErr = fmt.Errorf("failed to add main schema resource %s: %v", url, err)
			return
		}
		// Extract and cache referenced schemas
		refs := extractRefs(mainSchema, url)
		for _, refURL := range refs {
			refReader, err := loadOrFetchSchema(refURL, verbose)
			if err != nil {
				schemaErr = fmt.Errorf("failed to load referenced schema %s: %v", refURL, err)
				return
			}
			if err := compiler.AddResource(refURL, refReader); err != nil {
				schemaErr = fmt.Errorf("failed to add referenced schema resource %s: %v", refURL, err)
				return
			}
		}

		// Compile the schema
		schema, err = compiler.Compile(url)
		if err != nil {
			schemaErr = fmt.Errorf("failed to compile schema %s: %v", url, err)
			return
		}
	})
	return schemaErr
}

// loadOrFetchSchema loads a schema from cache or fetches it from the URL, caching the result
func loadOrFetchSchema(url string, verbose bool) (io.Reader, error) {
	cachePath := getCachePath(url)
	// Try to load from cache
	if file, err := os.Open(cachePath); err == nil {
		defer file.Close()
		data, err := io.ReadAll(file)
		if err != nil {
			return nil, fmt.Errorf("failed to read cached schema %s: %v", cachePath, err)
		}
		// Verify it's valid JSON
		var v interface{}
		if err := json.Unmarshal(data, &v); err != nil {
			// Invalid cache: proceed to fetch
			return fetchAndCacheSchema(url, verbose)
		}
		return bytes.NewReader(data), nil
	}
	// Cache miss: fetch from URL
	return fetchAndCacheSchema(url, verbose)
}

func resolveSchemaError(reqMap map[string]interface{}, verbose bool) (err error) {
	reqType := reqMap["req"]
	if reqType == nil {
		reqType = reqMap["cmd"]
	}
	reqTypeStr, ok := reqType.(string)
	if !ok {
		err = fmt.Errorf("request type not a string")
	} else if reqTypeStr == "" {
		err = fmt.Errorf("no request type specified")
	} else {
		// Normalize request type
		reqTypeStr = strings.ToLower(reqTypeStr)

		// Validate against the specific request schema
		schemaPath := filepath.Join(cacheDir, reqTypeStr+".req.notecard.api.json")
		if _, err = os.Stat(schemaPath); os.IsNotExist(err) {
			err = fmt.Errorf("unknown request type: %s", reqTypeStr)
		} else if err == nil {
			var reqSchema *jsonschema.Schema
			reqSchema, err = jsonschema.Compile(schemaPath)
			if err == nil {
				err = reqSchema.Validate(reqMap)
				if err != nil && !verbose {
					err = formatErrorMessage(reqTypeStr, err)
				}
			}
		}
	}

	return err
}

func validateRequest(reqMap map[string]interface{}, url string, verbose bool) (err error) {
	// Ensure schema is initialized
	if err = initSchema(url, verbose); err != nil {
		return fmt.Errorf("failed to initialize schema: %v", err)
	}

	// Validate the request against the schema
	if err = schema.Validate(reqMap); err != nil {
		return resolveSchemaError(reqMap, verbose)
	}

	// Validates successfully
	return nil
}

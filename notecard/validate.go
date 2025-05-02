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
func fetchAndCacheSchema(url string) (io.Reader, error) {
	// Fetch the schema
	fmt.Printf("*** fetching schema: %s ***\n", url)
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
		return nil, fmt.Errorf("invalid schema %s: %v", url, err)
	}
	// Save to cache
	cachePath := getCachePath(url)
	if err := os.WriteFile(cachePath, data, 0644); err != nil {
		// Log error but continue
		fmt.Fprintf(os.Stderr, "Failed to cache schema %s: %v\n", url, err)
	}
	return bytes.NewReader(data), nil
}

// getCachePath converts a URL to a safe file path in the cache directory
func getCachePath(url string) string {
	// Use the URL path as the filename, replacing invalid characters
	filename := strings.ReplaceAll(filepath.Base(url), string(os.PathSeparator), "_")
	return filepath.Join(cacheDir, filename)
}

// initSchema compiles the schema, using cached files if available
func initSchema(url string) error {
	schemaOnce.Do(func() {
		compiler := jsonschema.NewCompiler()
		compiler.Draft = jsonschema.Draft2020

		// Ensure cache directory exists
		if err := os.MkdirAll(cacheDir, 0755); err != nil {
			schemaErr = fmt.Errorf("failed to create cache directory: %v", err)
			return
		}

		// Load main schema
		mainSchemaReader, err := loadOrFetchSchema(url)
		if err != nil {
			schemaErr = fmt.Errorf("failed to load schema %s: %v", url, err)
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
			schemaErr = fmt.Errorf("failed to add schema resource %s: %v", url, err)
			return
		}
		// Extract and cache referenced schemas
		refs := extractRefs(mainSchema, url)
		for _, refURL := range refs {
			refReader, err := loadOrFetchSchema(refURL)
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
			schemaErr = fmt.Errorf("failed to compile schema: %v (use -force to bypass validation)", err)
			return
		}
	})
	return schemaErr
}

// loadOrFetchSchema loads a schema from cache or fetches it from the URL, caching the result
func loadOrFetchSchema(url string) (io.Reader, error) {
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
			return fetchAndCacheSchema(url)
		}
		return bytes.NewReader(data), nil
	}
	// Cache miss: fetch from URL
	return fetchAndCacheSchema(url)
}

func resolveSchemaError(reqMap map[string]interface{}) {
	// Identify base request to deduce schema URL
	reqType := reqMap["req"]
	if reqType == nil {
		reqType = reqMap["cmd"]
	}
	reqTypeStr, ok := reqType.(string)
	if !ok {
		return
	}
	fmt.Fprintf(os.Stderr, "Failed to validate %s request!\n", reqTypeStr)

	// Compose the request schema URL
	var reqSchemaUrl string = cacheDir
	reqSchemaUrl += reqTypeStr
	reqSchemaUrl += ".req.notecard.api.json"

	// Load the request schema
	compiler := jsonschema.NewCompiler()
	compiler.Draft = jsonschema.Draft2020
	var reqSchema *jsonschema.Schema
	reqSchema, err := compiler.Compile(reqSchemaUrl)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to load error schema!\n%v\n", err)
	} else if err := reqSchema.Validate(reqMap); err != nil {
		fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
	}
}

func validateRequest(reqMap map[string]interface{}, url string, verbose bool) {
	// Ensure schema is initialized
	if err := initSchema(url); err != nil {
		fmt.Fprintf(os.Stderr, "Failed to initialize schema: %v\n", err)
		return
	}

	// Validate the request against the schema
	if err := schema.Validate(reqMap); err != nil {
		// fmt.Fprintf(os.Stderr, "Validation error: %v\n", err)
		resolveSchemaError(reqMap)
		return
	}

	if verbose {
		fmt.Println("Validated against schema:", url)
	}
}

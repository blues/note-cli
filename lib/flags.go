// Copyright 2025 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

package lib

import (
	"flag"
	"fmt"
)

// Define flag groups
type FlagGroup struct {
	Name        string
	Description string
	Flags       []*flag.Flag
}

// Helper function to get flag by name from the default command line flags
func GetFlagByName(name string) *flag.Flag {
	return flag.CommandLine.Lookup(name)
}

// Helper function to print grouped commands
func PrintGroupedFlags(groups []FlagGroup, cli string) {
	fmt.Println(cli + " - Command line tool for interacting with " + cli + "\n")
	fmt.Println("USAGE: " + cli + " [options]\n")

	// First pass: find the longest flag name + type
	maxLen := 0
	for _, group := range groups {
		for _, f := range group.Flags {
			typeName, _ := flag.UnquoteUsage(f)
			length := len(f.Name)
			if len(typeName) > 0 {
				length += len(typeName) + 3 // +3 for flagText formatting
			}
			if length > maxLen {
				maxLen = length
			}
		}
	}

	// Add padding for the flag prefix "  -" and some extra space
	padding := maxLen + 5

	for _, group := range groups {
		fmt.Printf("%s:\n", group.Description)
		for _, f := range group.Flags {
			typeName, usage := flag.UnquoteUsage(f)
			flagText := f.Name
			if len(typeName) > 0 {
				flagText = fmt.Sprintf("%s (%s)", f.Name, typeName)
			}
			fmt.Printf("  -%*s%s\n", -padding, flagText, usage)
		}
		fmt.Println()
	}

	fmt.Println("For more detailed documentation and examples, visit:")
	fmt.Println("https://dev.blues.io/tools-and-sdks/" + cli + "-cli/\n")
}

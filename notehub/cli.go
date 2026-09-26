// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Command line framework.  A command line has the form
//
//	notehub [mode] [switches] [arguments]
//
// where the optional mode keyword selects which switches apply and how the arguments are
// read.  With no mode, or "default", the CLI behaves as it always has.  Switches and modes
// are defined in switches.go and modes.go.

package main

import (
	"flag"
	"fmt"
	"strings"

	"github.com/blues/note-cli/lib"
)

// The name of this CLI as typed on the command line, and where its docs live
const cliName = "notehub"
const cliDocsURL = "https://dev.blues.io/tools-and-sdks/" + cliName + "-cli"

// cliAnyMode in a switch's Modes makes it available in every mode
const cliAnyMode = "*"

// cliGroupGeneral is the group of switches that belong to the CLI rather than to a mode,
// and the only ones short help shows
const cliGroupGeneral = "general"

// cliMode is an optional first-argument keyword that selects how the rest is parsed
type cliMode struct {
	// Name is the keyword typed on the command line
	Name string
	// Summary is the one-line description used when modes are listed
	Summary string
	// Args describes this mode's non-switch arguments, for the USAGE line
	Args string
	// Commands, if present, are this mode's non-switch commands
	Commands []cliCommand
	// Detail, if present, is displayed at the end of this mode's help
	Detail string
	// Hidden indicates that this mode works but is not listed as one of the modes
	Hidden bool
	// Run is the handler for the mode, called after the command line has been parsed
	Run func(config *lib.ConfigSettings) error
}

// cliCommand is a non-switch command word accepted by a mode
type cliCommand struct {
	Name    string
	Args    string
	Summary string
	Run     func(config *lib.ConfigSettings, args []string) error
}

// cliSwitch is the definition of a single command line switch
type cliSwitch struct {
	// Name is the switch without its leading hyphens
	Name string
	// Target receives the value: a *bool, *string or *int, or nil if External
	Target any
	// Default is the value when the switch isn't given, or nil for the zero value
	Default any
	// Usage is the help text; as in the flag package, a `backquoted` word names the value
	Usage string
	// Group is the help group this switch is displayed in
	Group string
	// Modes are the modes in which this switch may be used, or cliAnyMode for all
	Modes []string
	// External means another package registers this switch; it is here for help and modes
	External bool
	// ValueType names the switch's value in help output when Target is nil
	ValueType string
}

// allowedIn returns true if this switch may be used in the specified mode
func (s *cliSwitch) allowedIn(modeName string) bool {
	for _, m := range s.Modes {
		if m == cliAnyMode || strings.EqualFold(m, modeName) {
			return true
		}
	}
	return false
}

// modeList returns a readable list of the modes in which this switch may be used
func (s *cliSwitch) modeList() string {
	names := []string{}
	for _, m := range s.Modes {
		if m == cliAnyMode {
			return "all modes"
		}
		names = append(names, m)
	}
	return strings.Join(names, ", ")
}

// takesValue returns true if this switch consumes the argument that follows it
func (s *cliSwitch) takesValue() bool {
	if _, isBool := s.Target.(*bool); isBool {
		return false
	}
	if s.Target == nil {
		return s.ValueType != ""
	}
	return true
}

// unquoteUsage returns the switch's value name and usage as the flag package would: a
// `backquoted` word in the usage names the value, and otherwise its type does
func (s *cliSwitch) unquoteUsage() (valueName string, usage string) {
	usage = s.Usage
	for i := 0; i < len(usage); i++ {
		if usage[i] == '`' {
			for j := i + 1; j < len(usage); j++ {
				if usage[j] == '`' {
					valueName = usage[i+1 : j]
					usage = usage[:i] + valueName + usage[j+1:]
					return
				}
			}
			break
		}
	}
	switch s.Target.(type) {
	case *bool:
		valueName = ""
	case *int:
		valueName = "int"
	case *string:
		valueName = "string"
	default:
		valueName = s.ValueType
	}
	return
}

// displayName is how this switch is displayed in help, such as "product (string)"
func (s *cliSwitch) displayName() string {
	valueName, _ := s.unquoteUsage()
	if valueName == "" {
		return s.Name
	}
	return fmt.Sprintf("%s (%s)", s.Name, valueName)
}

// cliExtractMode returns the mode named by the first argument and the arguments after it,
// or the default mode and all of them.  A bare word there can't be anything else, since a
// request is JSON or @filename.  '--skills' is accepted too, though help doesn't say so,
// unless it names a real switch, so that '--help' is still the switch.
func cliExtractMode(args []string) (mode *cliMode, remaining []string) {
	if len(args) == 0 {
		return cliModeNamed(modeDefault), args
	}
	name := args[0]
	if strings.HasPrefix(name, "-") {
		name = strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-")
		if !cliIsBareWord(name) || cliSwitchNamed(name) != nil {
			return cliModeNamed(modeDefault), args
		}
	}
	if mode = cliModeNamed(name); mode != nil {
		return mode, args[1:]
	}
	return cliModeNamed(modeDefault), args
}

// cliModeNamed returns the named mode, or nil if there is no such mode
func cliModeNamed(name string) *cliMode {
	for _, mode := range cliModes() {
		if strings.EqualFold(name, mode.Name) {
			return mode
		}
	}
	return nil
}

// cliVisibleModes returns the modes that are listed when modes are displayed
func cliVisibleModes() (modes []*cliMode) {
	for _, mode := range cliModes() {
		if !mode.Hidden {
			modes = append(modes, mode)
		}
	}
	return
}

// cliModeNames returns the names of the visible modes
func cliModeNames() (names []string) {
	for _, mode := range cliVisibleModes() {
		names = append(names, mode.Name)
	}
	return
}

// cliSwitchNamed returns the named switch in any mode, or nil
func cliSwitchNamed(name string) *cliSwitch {
	for _, s := range cliSwitches() {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// cliRegisterSwitches registers only this mode's switches with the flag package
func cliRegisterSwitches(mode *cliMode) {
	for _, s := range cliSwitches() {
		if s.External || !s.allowedIn(mode.Name) {
			continue
		}
		switch target := s.Target.(type) {
		case *bool:
			defaultValue, _ := s.Default.(bool)
			flag.BoolVar(target, s.Name, defaultValue, s.Usage)
		case *string:
			defaultValue, _ := s.Default.(string)
			flag.StringVar(target, s.Name, defaultValue, s.Usage)
		case *int:
			defaultValue, _ := s.Default.(int)
			flag.IntVar(target, s.Name, defaultValue, s.Usage)
		default:
			panic(fmt.Sprintf("--%s is defined with an unsupported target type %T", s.Name, s.Target))
		}
	}
}

// cliValidateSwitches refuses a known switch used outside its modes, which the flag
// package alone would report only as "not defined"
func cliValidateSwitches(mode *cliMode, args []string) error {
	for _, name := range cliScanSwitchNames(args) {
		s := cliSwitchNamed(name)
		if s == nil || s.allowedIn(mode.Name) {
			continue
		}
		return fmt.Errorf("--%s is not available in '%s' mode (it is available in: %s)", name, mode.Name, s.modeList())
	}
	return nil
}

// cliScanSwitchNames returns the switches named in args, up to where the flag package stops
func cliScanSwitchNames(args []string) (names []string) {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" || len(arg) < 2 || arg[0] != '-' {
			break
		}
		name := strings.TrimPrefix(arg[1:], "-")
		if name == "" || name[0] == '-' || name[0] == '=' {
			break
		}
		if equals := strings.Index(name, "="); equals >= 0 {
			names = append(names, name[:equals])
			continue
		}
		names = append(names, name)
		if s := cliSwitchNamed(name); s != nil && s.takesValue() {
			i++
		}
	}
	return
}

// cliDispatch runs the command named by the first of a mode's arguments.  With none it
// shows short help, so a mode that does something by default does it before calling this.
func cliDispatch(mode *cliMode, config *lib.ConfigSettings, args []string) error {
	if len(args) == 0 {
		cliPrintHelp(mode, false)
		return nil
	}
	if strings.EqualFold(args[0], modeHelp) {
		cliPrintHelp(mode, true)
		return nil
	}
	for i := range mode.Commands {
		if strings.EqualFold(args[0], mode.Commands[i].Name) {
			positional, err := cliParseCommandSwitches(mode, args[1:])
			if err != nil {
				return err
			}
			return mode.Commands[i].Run(config, positional)
		}
	}
	return fmt.Errorf("'%s' is not a %s %s command - use '%s %s --help' to see what is available",
		args[0], cliName, mode.Name, cliName, mode.Name)
}

// cliParseCommandSwitches parses switches wherever they appear among a command's arguments,
// and returns the rest.  The flag package stops at the first non-switch, which would make
// 'notehub skills get all ./dir --project x' ignore the project.
func cliParseCommandSwitches(mode *cliMode, args []string) (positional []string, err error) {

	// Separate the switches, and the values they take, from the arguments
	switches := []string{}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			positional = append(positional, args[i+1:]...)
			break
		}
		if len(arg) < 2 || arg[0] != '-' {
			positional = append(positional, arg)
			continue
		}
		name := strings.TrimPrefix(arg[1:], "-")
		if name == "" || name[0] == '-' || name[0] == '=' {
			return nil, fmt.Errorf("bad flag syntax: %s", arg)
		}
		switches = append(switches, arg)
		if !strings.Contains(name, "=") {
			if s := cliSwitchNamed(name); s != nil && s.takesValue() && i+1 < len(args) {
				i++
				switches = append(switches, args[i])
			}
		}
	}

	// Hold them to the same mode rules as the switches that precede the command
	if err = cliValidateSwitches(mode, switches); err != nil {
		return nil, err
	}

	// Parsing again adds to what was already parsed
	if err = flag.CommandLine.Parse(switches); err != nil {
		return nil, err
	}

	return positional, nil

}

// cliPrintHelp displays help for a mode.  The short form, shown when there's nothing to
// do, lists the modes, the mode's commands and the general options; the full form, for
// --help, adds every switch the mode accepts.
func cliPrintHelp(mode *cliMode, full bool) {

	// Header
	usage := cliName
	if mode.Name == modeDefault {
		fmt.Printf("%s - Command line tool for interacting with %s\n", cliName, cliName)
		usage += " [mode]"
	} else {
		fmt.Printf("%s %s - %s\n", cliName, mode.Name, mode.Summary)
		usage += " " + mode.Name
	}
	usage += " [options]"
	if mode.Args != "" {
		usage += " " + mode.Args
	}
	fmt.Printf("USAGE: %s\n", usage)
	fmt.Println()

	// The switches to show; the short form shows only the general ones
	shown := []*cliSwitch{}
	for _, s := range cliSwitches() {
		if !s.allowedIn(mode.Name) || (!full && s.Group != cliGroupGeneral) {
			continue
		}
		shown = append(shown, s)
	}

	// Modes are listed only in top-level help
	modes := []*cliMode{}
	if mode.Name == modeDefault {
		modes = cliVisibleModes()
	}

	// Align against the longest label
	maxLen := 0
	measure := func(label string) {
		if len(label) > maxLen {
			maxLen = len(label)
		}
	}
	for _, s := range shown {
		measure(s.displayName())
	}
	for _, c := range mode.Commands {
		measure(cliCommandLabel(c))
	}
	for _, m := range modes {
		measure(cliModeLabel(m))
	}
	padding := maxLen + 5

	// The modes
	if len(modes) != 0 {
		fmt.Printf("Modes:\n")
		for _, m := range modes {
			fmt.Printf("  %*s%s\n", -(padding + 2), cliModeLabel(m), m.Summary)
		}
		fmt.Println()
	}

	// This mode's commands
	if len(mode.Commands) != 0 {
		fmt.Printf("Commands:\n")
		for _, c := range mode.Commands {
			fmt.Printf("  %*s%s\n", -(padding + 2), cliCommandLabel(c), c.Summary)
		}
		fmt.Println()
	}

	// This mode's switches, in group order
	for _, group := range cliSwitchGroups() {
		printedGroup := false
		for _, s := range shown {
			if s.Group != group.Name {
				continue
			}
			if !printedGroup {
				fmt.Printf("%s:\n", group.Description)
				printedGroup = true
			}
			_, usage := s.unquoteUsage()
			fmt.Printf("  --%*s%s\n", -padding, s.displayName(), usage)
		}
		if printedGroup {
			fmt.Println()
		}
	}

	// Anything else this mode wishes to say
	if full && mode.Detail != "" {
		fmt.Printf("%s\n\n", strings.TrimRight(mode.Detail, "\n"))
	}

	fmt.Println("For more detailed documentation and examples, visit:")
	fmt.Println(cliDocsURL)

}

// cliCommandLabel is how a command is identified when commands are listed
func cliCommandLabel(command cliCommand) string {
	return strings.TrimSpace(command.Name + " " + command.Args)
}

// cliModeLabel is how a mode is identified when modes are listed
func cliModeLabel(mode *cliMode) string {
	if mode.Name == modeDefault {
		return mode.Name + " (or omitted)"
	}
	return mode.Name
}

// cliCheckNotAMode refuses a bare word where a mode's argument is expected.  A mode's
// arguments are requests or commands, so a bare word is nearly always a mode keyword that
// was misspelled or misplaced.
func cliCheckNotAMode(arg string) error {
	if !cliIsBareWord(arg) {
		return nil
	}
	if mode := cliModeNamed(arg); mode != nil {
		usage := cliName + " " + mode.Name + " [options]"
		if mode.Args != "" {
			usage += " " + mode.Args
		}
		return fmt.Errorf("'%s' is a mode, and a mode must be the first thing on the command line, as in: %s",
			arg, usage)
	}
	return fmt.Errorf("'%s' is neither a request nor a mode - a request is JSON or @filename, and the modes are: %s",
		arg, strings.Join(cliModeNames(), ", "))
}

// cliIsBareWord returns true if arg is a single word, without the punctuation of a
// request, a filename or a switch
func cliIsBareWord(arg string) bool {
	if arg == "" {
		return false
	}
	for _, c := range arg {
		isLetter := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z')
		isDigit := c >= '0' && c <= '9'
		if !isLetter && !isDigit && c != '-' && c != '_' {
			return false
		}
	}
	return true
}

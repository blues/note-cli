// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Command line framework.
//
// A command line has the form
//
//	notehub [mode] [switches] [arguments]
//
// where 'mode' is an optional leading keyword that determines which switches are
// available and how the remaining arguments are interpreted.  When no mode keyword
// is present, or when the keyword is literally "default", the CLI behaves exactly as
// it did before modes existed.
//
// Every switch declares the modes in which it may be used (see switches.go), and only
// the switches belonging to the selected mode are registered with the flag package.
// Help is generated from those same definitions, so the help for a mode describes
// precisely the switches that the mode accepts.
//
// This file is the machinery.  To add a switch or a mode, see switches.go and modes.go.

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

// cliAnyMode may be used in a switch's Modes list to indicate that the switch is
// available in every mode
const cliAnyMode = "*"

// cliGroupGeneral is the group of switches that are general to the CLI rather than
// belonging to any one mode.  These are the only switches displayed when help is
// displayed in its short form.
const cliGroupGeneral = "general"

// cliMode is a keyword that may optionally appear as the first argument on the
// command line, selecting a context in which the rest of the command line is parsed
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
	// Name is the switch without its leading hyphens.  Help spells it --name, which
	// is also how it should be spelled anywhere we mention it.
	Name string
	// Target is where the parsed value is stored, and must be a *bool, *string or
	// *int.  It is nil for switches that are registered by another package.
	Target any
	// Default is the value when the switch is not specified, or nil for the zero
	// value of the target's type
	Default any
	// Usage is the help text.  As with the flag package, a `backquoted` word within
	// it names the switch's value in help output.
	Usage string
	// Group is the name of the help group in which this switch is displayed
	Group string
	// Modes are the modes in which this switch may be used, or cliAnyMode for all
	Modes []string
	// External indicates that this switch is registered with the flag package by
	// another package.  It is defined here so that it is documented in help and so
	// that its mode list is enforced, but it is not registered by us.
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

// unquoteUsage extracts the name of this switch's value from its usage text, exactly
// as the flag package does: a `backquoted` word within the usage text names the
// value, and otherwise the name is derived from the type of the switch.  Switches
// that take no value, such as booleans, have no value name.
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

// cliExtractMode examines the arguments that follow the program name and, if the
// first of them is a mode keyword, returns that mode along with the arguments that
// remain after the keyword has been removed.  If no mode keyword is present the
// default mode is returned and the arguments are returned unchanged, which is what
// makes the mode keyword purely additive: every command line that was legal before
// modes existed still means exactly what it has always meant.
//
// A mode keyword is recognized only in the very first position.  A bare word there is
// unambiguous: the only other argument that has ever been legal in the first position
// is a device-like request, which is either JSON or an @filename, so no pre-existing
// command line can be mistaken for a mode keyword.  A mode typed as though it were a
// switch, as in '--train', is also accepted there, because a mode keyword looks like
// one to anyone used to a CLI whose command line is nothing but switches, and it is a
// natural thing to type.  These hyphenated forms work but are deliberately not
// mentioned in help, and a name that is a real switch is always left to the flag
// package, so '--help' remains the help switch rather than the hidden help mode.
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

// cliModeNames returns the names of the modes that are listed when modes are
// displayed
func cliModeNames() (names []string) {
	for _, mode := range cliVisibleModes() {
		names = append(names, mode.Name)
	}
	return
}

// cliSwitchNamed returns the named switch regardless of mode, or nil if there is no
// such switch
func cliSwitchNamed(name string) *cliSwitch {
	for _, s := range cliSwitches() {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// cliRegisterSwitches registers the switches available in the specified mode, and
// only those switches, with the flag package
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

// cliValidateSwitches returns an error if the command line uses a switch that this
// CLI knows about but that is not available in the specified mode.  Without this the
// flag package would report only that the switch is "not defined", which is unhelpful
// when the switch does exist but belongs to another mode.
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

// cliScanSwitchNames returns the names of the switches that appear on the specified
// command line, stopping where the flag package itself would stop parsing
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

// cliDispatch runs the command named by the first of a mode's non-switch arguments.
//
// What a mode does when it is given no command at all is up to the mode, and a mode
// that wants to do something there does it before calling this.  Note that this is
// where a mode's human-readable help is displayed but that it is never what a mode
// does by default, because the output of a bare 'notehub <mode>' belongs to the mode
// and may well be intended for something other than a person.
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

// cliParseCommandSwitches parses the switches that follow a command word, wherever they
// appear among that command's arguments, and returns the arguments that remain.
//
// This is needed because the flag package stops parsing at the first argument that isn't
// a switch, which would make 'notehub skills pull ./dir --project x' silently ignore the
// project.  Putting a switch after the thing it applies to is what both people and agents
// naturally do, so within a mode's commands we accept switches anywhere.
func cliParseCommandSwitches(mode *cliMode, args []string) (positional []string, err error) {

	// Separate the switches from everything else, using the switch definitions to know
	// which of them consume the argument that follows
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

	// Parsing again adds these to whatever was already parsed, because the flag package
	// leaves a switch alone unless it is set again
	if err = flag.CommandLine.Parse(switches); err != nil {
		return nil, err
	}

	return positional, nil

}

// cliPrintHelp displays help for the specified mode.  The short form is what we
// display when we are invoked with nothing to do, and it shows only where to go
// next: the modes, this mode's commands, and the general options.  The full form,
// which is what --help displays, adds every switch available in the mode.
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

	// The switches to be displayed, which in the short form are only the general ones
	shown := []*cliSwitch{}
	for _, s := range cliSwitches() {
		if !s.allowedIn(mode.Name) || (!full && s.Group != cliGroupGeneral) {
			continue
		}
		shown = append(shown, s)
	}

	// The modes, which are listed only in the top-level help
	modes := []*cliMode{}
	if mode.Name == modeDefault {
		modes = cliVisibleModes()
	}

	// Align everything that follows against the longest label
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

// cliCheckNotAMode returns an error if the specified argument, which is about to be
// used as something other than a mode keyword, is in fact a bare word of the kind
// that is used as one.  The arguments that a mode accepts are always specific things
// such as a request or a command, so a bare word that isn't one of them is nearly
// always a mode keyword that was misspelled or typed in the wrong place.
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

// cliIsBareWord returns true if the argument is a single unadorned word, containing
// none of the punctuation that a request, a filename, or a switch would contain
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

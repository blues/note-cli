// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// Package cli is the command line framework shared by the note CLIs.  A command line has
// the form
//
//	<tool> [mode] [switches] [arguments]
//
// where the optional mode keyword selects which switches apply and how the arguments are
// read.  With no mode, the tool runs its default mode.  A tool describes its modes and
// switches in one table each, and from those the framework registers only the switches of
// the mode being run, names a switch used in the wrong mode as such, dispatches a mode's
// commands, and generates help.
package cli

import (
	"errors"
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/blues/note-cli/lib"
)

// ModeDefault names the mode that runs when no mode keyword is given
const ModeDefault = "default"

// Help is the word that asks for help where a mode keyword or a mode's command goes, and
// the switch that asks for it anywhere
const Help = "help"

// AnyMode in a switch's Modes makes it available in every mode
const AnyMode = "*"

// GroupGeneral is the group of switches that belong to the tool rather than to a mode,
// and the only ones short help shows
const GroupGeneral = "general"

// Tool describes a command line tool: what it is called, and its modes and switches
type Tool struct {
	// Name is the tool's name as typed on the command line
	Name string
	// Title says what the tool is, on the first line of its help
	Title string
	// DocsURL is where the tool's documentation lives, or "" for the usual place
	DocsURL string
	// Modes are the tool's modes, in the order help lists them; one is named ModeDefault
	Modes []*Mode
	// Switches are the switches of every mode; the framework adds --help
	Switches []*Switch
	// Groups are the help groups, in display order
	Groups []Group
	// NotecardConfig has lib register --interface, --port and --portconfig, for a tool
	// that talks to a Notecard
	NotecardConfig bool
	// NotehubConfig has lib register --hub, for a tool that talks to Notehub
	NotehubConfig bool
	// AfterParse, if present, is called whenever switches have been parsed, for a tool
	// that reconciles them
	AfterParse func()
}

// Mode is an optional first-argument keyword that selects how the rest is parsed
type Mode struct {
	// Name is the keyword typed on the command line
	Name string
	// Summary is the one-line description used when modes are listed
	Summary string
	// Args describes this mode's non-switch arguments, for the USAGE line
	Args string
	// Commands, if present, are this mode's non-switch commands
	Commands []Command
	// Examples, if present, are displayed after this mode's switches
	Examples []Example
	// Preamble, if present, is displayed under the USAGE line, before anything else
	Preamble string
	// Detail, if present, is displayed at the end of this mode's help
	Detail string
	// Hidden indicates that this mode works but is not listed as one of the modes
	Hidden bool
	// Run is the handler for the mode, called after the command line has been parsed
	Run func(config *lib.ConfigSettings) error
}

// Example is one example in a mode's help: what it does, and the command line
type Example struct {
	What    string
	Command string
}

// Command is a non-switch command word accepted by a mode
type Command struct {
	Name    string
	Args    string
	Summary string
	Run     func(config *lib.ConfigSettings, args []string) error
}

// Group is a help group: its name in a switch's Group, and its heading in help
type Group struct {
	Name        string
	Description string
}

// Switch is the definition of a single command line switch
type Switch struct {
	// Name is the switch without its leading hyphens
	Name string
	// Target receives the value: a *bool, *string, *int, *OptionalValue or *StringList,
	// or nil if External
	Target any
	// Default is the value when the switch isn't given, or nil for the zero value
	Default any
	// Usage is the help text; as in the flag package, a `backquoted` word names the value
	Usage string
	// Group is the help group this switch is displayed in
	Group string
	// Modes are the modes in which this switch may be used, or AnyMode for all
	Modes []string
	// External means lib registers this switch; it is here for help and modes
	External bool
	// ValueType names the switch's value in help output when Target is nil
	ValueType string
	// Hidden means the switch works but isn't shown in help: a spelling kept for
	// compatibility
	Hidden bool
}

// OptionalValue is the target of a switch that may be given with or without a value.  Go's
// flag package has no such switches, but one that calls itself boolean isn't made to take
// an argument: it is given "true" when bare, and the value when written as --name=value.
// A value that follows after a space is left as a positional argument, for the command to
// take as it sees fit.
type OptionalValue struct {
	// Given reports that the switch was on the command line, with a value or without
	Given bool
	// Value is the value it was given, or "" when bare
	Value string
}

// String returns the value, as the flag package asks
func (v *OptionalValue) String() string { return v.Value }

// Set records the switch as given, with its value unless it was bare
func (v *OptionalValue) Set(value string) error {
	v.Given = true
	if value != "true" {
		v.Value = value
	}
	return nil
}

// IsBoolFlag tells the flag package not to make the switch take an argument
func (v *OptionalValue) IsBoolFlag() bool { return true }

// StringList is the target of a switch that may be given more than once, collecting every
// value
type StringList []string

// String returns the values, as the flag package asks
func (l *StringList) String() string { return strings.Join(*l, ",") }

// Set adds a value
func (l *StringList) Set(value string) error {
	*l = append(*l, value)
	return nil
}

// The tool that is running, with its switches and modes as the framework completes them
var (
	tool     *Tool
	switches []*Switch
	modes    []*Mode
)

// helpGiven is set by --help
var helpGiven bool

// Define makes tool the one that is running.  Main calls it before anything else, and so
// does a test.  A mistake in the tables is a programming error, and is reported as one.
func Define(t *Tool) {
	tool = t
	if tool.DocsURL == "" {
		tool.DocsURL = "https://dev.blues.io/tools-and-sdks/" + tool.Name + "-cli"
	}
	helpGiven = false
	switches = append([]*Switch{{Name: Help, Target: &helpGiven, Group: GroupGeneral, Modes: []string{AnyMode},
		Usage: "display every option available in this mode"}}, tool.Switches...)
	modes = append(append([]*Mode{}, tool.Modes...), &Mode{
		Name:    Help,
		Summary: "display help for this tool or for one of its modes",
		Args:    "[mode]",
		Hidden:  true,
		Run:     runHelp,
	})
	if err := CheckDefinitions(); err != nil {
		panic(fmt.Sprintf("%s: %s", tool.Name, err))
	}
}

// Name returns the tool's name
func Name() string {
	return tool.Name
}

// Modes returns every mode, in the order help displays them
func Modes() []*Mode {
	return modes
}

// VisibleModes returns the modes that are listed when modes are displayed
func VisibleModes() (visible []*Mode) {
	for _, mode := range modes {
		if !mode.Hidden {
			visible = append(visible, mode)
		}
	}
	return
}

// ModeNames returns the names of the visible modes
func ModeNames() (names []string) {
	for _, mode := range VisibleModes() {
		names = append(names, mode.Name)
	}
	return
}

// ModeNamed returns the named mode, or nil if there is no such mode
func ModeNamed(name string) *Mode {
	for _, mode := range modes {
		if strings.EqualFold(name, mode.Name) {
			return mode
		}
	}
	return nil
}

// Switches returns every switch, in any mode
func Switches() []*Switch {
	return switches
}

// SwitchNamed returns the named switch in any mode, or nil.  A name defined for more than
// one set of modes returns its first definition, which is enough to know how it is parsed.
func SwitchNamed(name string) *Switch {
	for _, s := range switches {
		if s.Name == name {
			return s
		}
	}
	return nil
}

// SwitchesNamed returns every definition of the named switch: a name may be defined once
// per set of modes, with its own usage and group in each
func SwitchesNamed(name string) (found []*Switch) {
	for _, s := range switches {
		if s.Name == name {
			found = append(found, s)
		}
	}
	return
}

// Groups returns the help groups, in display order
func Groups() []Group {
	return tool.Groups
}

// CommandNamed returns the named command among some, or nil
func CommandNamed(commands []Command, name string) *Command {
	for i := range commands {
		if strings.EqualFold(name, commands[i].Name) {
			return &commands[i]
		}
	}
	return nil
}

// CommandLabel is how a command is identified when commands are listed
func CommandLabel(command Command) string {
	return strings.TrimSpace(command.Name + " " + command.Args)
}

// CheckDefinitions reports what is wrong with the tool's tables, or nil
func CheckDefinitions() error {
	var problems []error
	problem := func(format string, args ...any) {
		problems = append(problems, fmt.Errorf(format, args...))
	}

	// The modes
	seen := map[string]bool{}
	for _, mode := range modes {
		if mode.Name == "" {
			problem("a mode is defined with no name")
			continue
		}
		if seen[mode.Name] {
			problem("the '%s' mode is defined more than once", mode.Name)
		}
		seen[mode.Name] = true
		if mode.Summary == "" {
			problem("the '%s' mode has no summary", mode.Name)
		}
		if mode.Run == nil {
			problem("the '%s' mode has no handler", mode.Name)
		}
		for _, c := range mode.Commands {
			if c.Name == "" || c.Run == nil {
				problem("the '%s' mode has a command with no name or no handler", mode.Name)
			}
		}
		for _, example := range mode.Examples {
			if example.What == "" || !strings.Contains(example.Command, tool.Name+" ") {
				problem("the '%s' mode has an example without a description or a command line: %+v", mode.Name, example)
			}
		}
	}
	if !seen[ModeDefault] {
		problem("there is no '%s' mode", ModeDefault)
	}

	// The switches
	defined := map[string][]*Switch{}
	for _, s := range switches {
		if s.Name == "" {
			problem("a switch is defined with no name")
			continue
		}
		// A name may be defined once per set of modes, and never twice for one mode
		for _, other := range defined[s.Name] {
			for _, mode := range modes {
				if s.AllowedIn(mode.Name) && other.AllowedIn(mode.Name) {
					problem("--%s is defined twice for the '%s' mode", s.Name, mode.Name)
				}
			}
		}
		defined[s.Name] = append(defined[s.Name], s)
		if len(s.Modes) == 0 {
			problem("--%s is not available in any mode", s.Name)
		}
		for _, m := range s.Modes {
			if m != AnyMode && ModeNamed(m) == nil {
				problem("--%s is available in '%s', which is not a mode", s.Name, m)
			}
		}
		if !groupExists(s.Group) {
			problem("--%s is in group '%s', which is not a group", s.Name, s.Group)
		}
		// General options are shown by short help, for every mode
		if s.Group == GroupGeneral {
			if s.Hidden {
				problem("--%s is hidden, and so can't be a general option", s.Name)
			}
			for _, mode := range modes {
				if !s.AllowedIn(mode.Name) {
					problem("--%s is a general option and so must be available in every mode, including '%s'", s.Name, mode.Name)
				}
			}
		}
		if valueName, _ := s.unquoteUsage(); valueName == "string" || valueName == "int" {
			problem("--%s doesn't say what its value is: put a `word` for it in its usage", s.Name)
		}
		switch s.Target.(type) {
		case *bool, *string, *int, *OptionalValue, *StringList:
			if s.External {
				problem("--%s is registered by lib and so must not have a target", s.Name)
			}
		case nil:
			if !s.External {
				problem("--%s has no target and so must be registered by lib", s.Name)
			}
		default:
			problem("--%s has an unsupported target type %T", s.Name, s.Target)
		}
	}
	return errors.Join(problems...)
}

// groupExists reports whether a group is one of the tool's
func groupExists(name string) bool {
	for _, group := range tool.Groups {
		if group.Name == name {
			return true
		}
	}
	return false
}

// AllowedIn returns true if this switch may be used in the specified mode
func (s *Switch) AllowedIn(modeName string) bool {
	for _, m := range s.Modes {
		if m == AnyMode || strings.EqualFold(m, modeName) {
			return true
		}
	}
	return false
}

// modeList returns a readable list of the modes in which this switch may be used
func (s *Switch) modeList() string {
	names := []string{}
	for _, m := range s.Modes {
		if m == AnyMode {
			return "all modes"
		}
		names = append(names, m)
	}
	return strings.Join(names, ", ")
}

// takesValue returns true if this switch consumes the argument that follows it
func (s *Switch) takesValue() bool {
	switch s.Target.(type) {
	case *bool, *OptionalValue:
		return false
	}
	if s.Target == nil {
		return s.ValueType != ""
	}
	return true
}

// unquoteUsage returns the switch's value name and usage as the flag package would: a
// `backquoted` word in the usage names the value, and otherwise its type does
func (s *Switch) unquoteUsage() (valueName string, usage string) {
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
	case *OptionalValue, *StringList:
		valueName = "value"
	default:
		valueName = s.ValueType
	}
	return
}

// displayName is how this switch is displayed in help, such as "product (productUID)"
func (s *Switch) displayName() string {
	valueName, _ := s.unquoteUsage()
	if valueName == "" {
		return s.Name
	}
	if _, isOptional := s.Target.(*OptionalValue); isOptional {
		return fmt.Sprintf("%s [%s]", s.Name, valueName)
	}
	return fmt.Sprintf("%s (%s)", s.Name, valueName)
}

// Start begins a run: it finds the mode keyword among args, which main passes as
// os.Args[1:], registers that mode's switches, and makes its help the usage that the flag
// package shows.  The keyword is removed from os.Args, so that the flag package and the
// configuration handling see the command line they always have.
func Start(args []string) (mode *Mode, rest []string) {
	mode, rest = ExtractMode(args)
	os.Args = append([]string{os.Args[0]}, rest...)
	RegisterSwitches(mode)
	flag.Usage = func() {
		PrintHelp(mode, true)
	}
	return mode, rest
}

// Parse parses a mode's switches, applies the configuration ones, and returns the
// configuration.  A command line that asks for help gets it, and the tool exits.  Only the
// default mode adopts a --hub given alone as the saved hub; the others act on the hub it
// names.
func Parse(mode *Mode, args []string) (*lib.ConfigSettings, error) {

	// Name a switch used in the wrong mode, rather than calling it undefined
	if err := ValidateSwitches(mode, args); err != nil {
		return nil, err
	}

	// Parse these switches and also lib's configuration switches
	if err := lib.FlagParseSaving(tool.NotecardConfig, tool.NotehubConfig, mode.Name == ModeDefault); err != nil {
		return nil, err
	}
	if tool.AfterParse != nil {
		tool.AfterParse()
	}

	// Get the resulting configuration
	config, err := lib.GetConfig()
	if err != nil {
		return nil, fmt.Errorf("config: %w", err)
	}

	// Display everything available in this mode, if that's all that was asked for
	if helpGiven {
		PrintHelp(mode, true)
		os.Exit(0)
	}
	return config, nil

}

// ExtractMode returns the mode named by the first argument and the arguments after it, or
// the default mode and all of them.  A bare word there can't be anything else, since a
// request is JSON or @filename.  '--skills' is accepted too, though help doesn't say so,
// unless it names a real switch, so that '--help' is still the switch.
func ExtractMode(args []string) (mode *Mode, remaining []string) {
	if len(args) == 0 {
		return ModeNamed(ModeDefault), args
	}
	name := args[0]
	if strings.HasPrefix(name, "-") {
		name = strings.TrimPrefix(strings.TrimPrefix(name, "-"), "-")
		if !IsBareWord(name) || SwitchNamed(name) != nil {
			return ModeNamed(ModeDefault), args
		}
	}
	if mode = ModeNamed(name); mode != nil {
		return mode, args[1:]
	}
	return ModeNamed(ModeDefault), args
}

// RegisterSwitches registers only this mode's switches with the flag package
func RegisterSwitches(mode *Mode) {
	for _, s := range switches {
		if s.External || !s.AllowedIn(mode.Name) {
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
		case *OptionalValue:
			flag.Var(target, s.Name, s.Usage)
		case *StringList:
			flag.Var(target, s.Name, s.Usage)
		default:
			panic(fmt.Sprintf("--%s is defined with an unsupported target type %T", s.Name, s.Target))
		}
	}
}

// ValidateSwitches refuses a known switch used outside its modes, which the flag package
// alone would report only as "not defined"
func ValidateSwitches(mode *Mode, args []string) error {
	for _, name := range ScanSwitchNames(args) {
		definitions := SwitchesNamed(name)
		if len(definitions) == 0 {
			continue
		}
		available := []string{}
		for _, s := range definitions {
			if s.AllowedIn(mode.Name) {
				available = nil
				break
			}
			available = append(available, s.modeList())
		}
		if available == nil {
			continue
		}
		return fmt.Errorf("--%s is not available in '%s' mode (it is available in: %s)", name, mode.Name, strings.Join(available, ", "))
	}
	return nil
}

// ScanSwitchNames returns the switches named in args, up to where the flag package stops
func ScanSwitchNames(args []string) (names []string) {
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
		if s := SwitchNamed(name); s != nil && s.takesValue() {
			i++
		}
	}
	return
}

// SwitchGiven reports whether the named switch was given on the command line, in whichever
// place the mode reads switches from
func SwitchGiven(name string) (given bool) {
	flag.Visit(func(f *flag.Flag) {
		if f.Name == name {
			given = true
		}
	})
	return
}

// SwitchAmong reports whether the named switch appears anywhere among args, before they
// have been parsed, and before a standalone "--"
func SwitchAmong(args []string, name string) bool {
	for _, arg := range args {
		if arg == "--" {
			return false
		}
		if len(arg) < 2 || arg[0] != '-' {
			continue
		}
		switchName := strings.TrimPrefix(arg[1:], "-")
		if switchName == name || strings.HasPrefix(switchName, name+"=") {
			return true
		}
	}
	return false
}

// FirstPositional returns the first of args that isn't a switch or a switch's value,
// before any have been parsed, or "" if there is none
func FirstPositional(args []string) string {
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if arg == "--" {
			if i+1 < len(args) {
				return args[i+1]
			}
			return ""
		}
		if len(arg) < 2 || arg[0] != '-' {
			return arg
		}
		name := strings.TrimPrefix(arg[1:], "-")
		if !strings.Contains(name, "=") {
			if s := SwitchNamed(name); s != nil && s.takesValue() {
				i++
			}
		}
	}
	return ""
}

// Dispatch runs the command named by the first of a mode's arguments.  With none it shows
// short help, so a mode that does something by default does it before calling this.
func Dispatch(mode *Mode, config *lib.ConfigSettings, args []string) error {
	if len(args) == 0 {
		PrintHelp(mode, false)
		return nil
	}
	if strings.EqualFold(args[0], Help) {
		PrintHelp(mode, true)
		return nil
	}
	for i := range mode.Commands {
		if strings.EqualFold(args[0], mode.Commands[i].Name) {
			positional, err := ParseCommandSwitches(mode, args[1:])
			if err != nil {
				return err
			}
			return mode.Commands[i].Run(config, positional)
		}
	}
	return fmt.Errorf("'%s' is not a %s %s command - use '%s %s --help' to see what is available",
		args[0], tool.Name, mode.Name, tool.Name, mode.Name)
}

// ParseCommandSwitches parses switches wherever they appear among a command's arguments,
// and returns the rest.  The flag package stops at the first non-switch, which would make
// 'notehub skills get all ./dir --project x' ignore the project.
func ParseCommandSwitches(mode *Mode, args []string) (positional []string, err error) {

	// Separate the switches, and the values they take, from the arguments
	found := []string{}
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
		found = append(found, arg)
		if !strings.Contains(name, "=") {
			if s := SwitchNamed(name); s != nil && s.takesValue() && i+1 < len(args) {
				i++
				found = append(found, args[i])
			}
		}
	}

	// Hold them to the same mode rules as the switches that precede the command
	if err = ValidateSwitches(mode, found); err != nil {
		return nil, err
	}

	// Parsing again adds to what was already parsed, and the configuration switches among
	// them, such as --hub, take effect now
	if err = flag.CommandLine.Parse(found); err != nil {
		return nil, err
	}
	if err = lib.ConfigFlagsProcess(); err != nil {
		return nil, err
	}
	if tool.AfterParse != nil {
		tool.AfterParse()
	}

	return positional, nil

}

// usage is a mode's USAGE line
func usage(mode *Mode) string {
	line := tool.Name
	if mode.Name != ModeDefault {
		line += " " + mode.Name
	}
	line += " [options]"
	if mode.Args != "" {
		line += " " + mode.Args
	}
	return line
}

// PrintHelp displays help for a mode.  The short form, shown when there's nothing to do,
// lists the modes, the mode's commands and the general options; the full form, for --help,
// adds every switch the mode accepts.
func PrintHelp(mode *Mode, full bool) {

	// Modes are listed only in top-level help, and the default mode isn't one of them
	// there: the usage line and the examples show its form
	listed := []*Mode{}
	if mode.Name == ModeDefault {
		for _, m := range VisibleModes() {
			if m.Name != ModeDefault {
				listed = append(listed, m)
			}
		}
	}

	// Header.  The default mode is the tool itself; every other mode is named.
	if mode.Name == ModeDefault {
		fmt.Printf("%s - %s\n", tool.Name, tool.Title)
		fmt.Printf("USAGE: %s\n", usage(mode))
		if len(listed) != 0 {
			fmt.Printf("       %s <mode> [options] [arguments]\n", tool.Name)
		}
	} else {
		fmt.Printf("%s %s - %s\n", tool.Name, mode.Name, mode.Summary)
		fmt.Printf("USAGE: %s\n", usage(mode))
	}
	fmt.Println()

	// What the mode wishes to say before anything else
	if mode.Preamble != "" {
		fmt.Printf("%s\n\n", strings.TrimRight(mode.Preamble, "\n"))
	}

	// The switches to show; the short form shows only the general ones
	shown := []*Switch{}
	for _, s := range switches {
		if s.Hidden || !s.AllowedIn(mode.Name) || (!full && s.Group != GroupGeneral) {
			continue
		}
		shown = append(shown, s)
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
		measure(CommandLabel(c))
	}
	for _, m := range listed {
		measure(m.Name)
	}
	padding := maxLen + 5

	// The modes
	if len(listed) != 0 {
		fmt.Printf("Modes:\n")
		for _, m := range listed {
			fmt.Printf("  %*s%s\n", -(padding + 2), m.Name, m.Summary)
		}
		fmt.Println()
	}

	// This mode's commands
	if len(mode.Commands) != 0 {
		fmt.Printf("Commands:\n")
		for _, c := range mode.Commands {
			fmt.Printf("  %*s%s\n", -(padding + 2), CommandLabel(c), c.Summary)
		}
		fmt.Println()
	}

	// This mode's switches, in group order
	for _, group := range tool.Groups {
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

	// How the mode is used, in gh's form: what each example does, then the command line
	if len(mode.Examples) != 0 {
		fmt.Printf("Examples:\n")
		for i, example := range mode.Examples {
			if i != 0 {
				fmt.Println()
			}
			fmt.Printf("  # %s\n  %s\n", example.What, example.Command)
		}
		fmt.Println()
	}

	// Anything else this mode wishes to say
	if full && mode.Detail != "" {
		fmt.Printf("%s\n\n", strings.TrimRight(mode.Detail, "\n"))
	}

	fmt.Println("For more detailed documentation and examples, visit:")
	fmt.Println(tool.DocsURL)

}

// PrintSettings shows the saved settings this tool uses, after its short help
func PrintSettings(config *lib.ConfigSettings) {
	config.PrintSettings(tool.NotecardConfig, tool.NotehubConfig)
}

// runHelp is the handler for '<tool> help [mode]'
func runHelp(config *lib.ConfigSettings) error {

	// With no argument, display the top-level help
	args := flag.Args()
	if len(args) == 0 {
		PrintHelp(ModeNamed(ModeDefault), true)
		return nil
	}

	// Display help for the specified mode
	mode := ModeNamed(args[0])
	if mode == nil {
		return fmt.Errorf("there is no '%s' mode - the modes are: %s", args[0], strings.Join(ModeNames(), ", "))
	}
	PrintHelp(mode, true)
	return nil

}

// CheckNotAMode refuses a bare word where a mode's argument is expected.  A mode's
// arguments are requests or commands, so a bare word is nearly always a mode keyword that
// was misspelled or misplaced.
func CheckNotAMode(arg string) error {
	if !IsBareWord(arg) {
		return nil
	}
	if mode := ModeNamed(arg); mode != nil {
		return fmt.Errorf("'%s' is a mode, and a mode must be the first thing on the command line, as in: %s",
			arg, usage(mode))
	}
	found, usages := []string{}, []string{}
	for _, mode := range VisibleModes() {
		for _, command := range mode.Commands {
			if strings.EqualFold(arg, command.Name) {
				found = append(found, mode.Name)
				usages = append(usages, tool.Name+" "+mode.Name+" "+CommandLabel(command))
			}
		}
	}
	if len(found) == 1 {
		return fmt.Errorf("'%s' is a command of the %s mode, as in: %s", arg, found[0], usages[0])
	}
	if len(found) > 1 {
		return fmt.Errorf("'%s' is a command of the %s modes, as in: %s", arg, strings.Join(found, " and "), strings.Join(usages, ", or "))
	}
	return fmt.Errorf("'%s' is neither a request nor a mode - a request is JSON or @filename, and the modes are: %s",
		arg, strings.Join(ModeNames(), ", "))
}

// IsBareWord returns true if arg is a single word, without the punctuation of a request,
// a filename or a switch
func IsBareWord(arg string) bool {
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

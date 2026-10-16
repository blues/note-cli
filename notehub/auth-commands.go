// Copyright 2026 Blues Inc.  All rights reserved.
// Use of this source code is governed by licenses granted by the
// copyright holder including that found in the LICENSE file.

// 'notehub login', 'notehub logout' and 'notehub auth': the commands that manage logins,
// of which there is one per hub.  'auth login' and 'auth logout' are the same as the
// top-level modes, and 'auth status', 'auth switch' and 'auth token' complete the set, in
// parallel with the gh CLI's auth commands.  The mechanics are in auth.go.

package main

import (
	"bufio"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/blues/note-cli/lib"
	"github.com/blues/note-cli/lib/cli"
	"github.com/blues/note-go/note"
	"github.com/mattn/go-isatty"
)

// The modes in which the login switches may be used
var inLoginAndAuth = []string{modeLogin, modeAuth}

// flagWithToken is --with-token, which may be given with the token or without it, in
// which case the token is read from stdin
var flagWithToken cli.OptionalValue

// flagLoginExpiresIn is how long the hub is asked to keep a sign-in open, in seconds
var flagLoginExpiresIn int

// flagAuthAll includes every saved login in 'auth status'.
var flagAuthAll bool

// loginExpiresInDefault is the hub's usual; the hub grants the lesser of what is asked
// and its own limit, and says which, and that is what the first message reports
const loginExpiresInDefault = 600

// authSwitches returns the switches of the login and auth modes
func authSwitches() []*cli.Switch {
	return []*cli.Switch{
		{Name: "with-token", Target: &flagWithToken, Group: "auth", Modes: inLoginAndAuth,
			Usage: "sign in with a personal access `token`, given after it or on stdin, instead of in the browser"},
		{Name: "name", Target: &flagSignInAgent, Group: "auth", Modes: inLoginAndAuth,
			Usage: "list this login in Notehub's API Access under this `name`, rather than the computer's"},
		{Name: "expires-in", Target: &flagLoginExpiresIn, Default: loginExpiresInDefault, Group: "auth", Modes: inLoginAndAuth,
			Usage: "how many `seconds` to ask the hub to keep the sign-in link good for, up to its own limit"},
		{Name: "json", Target: &flagJson, Group: "output", Modes: inLoginAndAuth,
			Usage: "write JSON instead of text: for a login, the URL, progress and result an agent needs; for status, the report"},
		{Name: "all", Target: &flagAuthAll, Group: "auth", Modes: []string{modeAuth},
			Usage: "with auth status, check every saved login instead of only the active connection"},

		// --pretty works wherever JSON is written; the main help describes it once
		{Name: "pretty", Target: &flagPretty, Group: "output", Modes: []string{modeAuth, modeApi, modeFirmware, modeFiles}, Hidden: true,
			Usage: "pretty print json output"},
	}
}

// authCommands returns the commands accepted by the auth mode
func authCommands() []cli.Command {
	return []cli.Command{
		{Name: "login", Summary: "sign into hub: in your browser, with --with-token, or from an agent with --json",
			Run: authLoginCommand},
		{Name: "logout", Summary: "sign out of the active hub, or of the one given by --hub",
			Run: authLogoutCommand},
		{Name: "status", Summary: "check the active connection's login; --all checks every saved login",
			Run: authStatusCommand},
		{Name: "switch", Args: "[hub]", Summary: "make a hub you have signed into the active one, or list them",
			Run: authSwitchCommand},
		{Name: "token", Summary: "print the token in use for the active hub, or for the one given by --hub",
			Run: authTokenCommand},
	}
}

// runLogin is the handler for 'notehub login', the same as 'notehub auth login'.  As in
// the auth mode, switches may come before or after the token.
func runLogin(config *lib.ConfigSettings) error {
	args, err := cli.ParseCommandSwitches(cli.ModeNamed(modeLogin), flag.Args())
	if err != nil {
		return err
	}
	return authLoginCommand(config, args)
}

// runLogout is the handler for 'notehub logout', the same as 'notehub auth logout'
func runLogout(config *lib.ConfigSettings) error {
	args, err := cli.ParseCommandSwitches(cli.ModeNamed(modeLogout), flag.Args())
	if err != nil {
		return err
	}
	return authLogoutCommand(config, args)
}

// runAuth is the handler for 'notehub auth'
func runAuth(config *lib.ConfigSettings) error {
	return cli.Dispatch(cli.ModeNamed(modeAuth), config, flag.Args())
}

// errAgentReported marks an error that has already been reported to the agent as JSON,
// so that main doesn't report it again
var errAgentReported = errors.New("reported to the agent")

// authLoginCommand logs in as the switches ask.  With --json, an agent is told everything
// as JSON, including whatever goes wrong here, and so is never told again by main.
func authLoginCommand(config *lib.ConfigSettings, args []string) error {
	if cli.SwitchGiven("json") {
		reporter := newAgentSignInReporter(os.Stdout)
		err := authLogin(config, args, reporter)
		if outputErr := reporter.finish(err); err == nil {
			err = outputErr
		}
		if err != nil {
			return errAgentReported
		}
		return nil
	}
	if err := authLogin(config, args, nil); err != nil {
		return fmt.Errorf("login: %w", err)
	}
	return nil
}

// authLogin logs in: in the browser, with a personal access token, or from an agent, which
// is reported to as it goes.  The login is to the hub in use, which --hub may have named
// for this command alone, and so may not be the active one.
func authLogin(config *lib.ConfigSettings, args []string, reporter *agentSignInReporter) error {
	withToken, token, _, err := authLoginChoice(args)
	if err != nil {
		return err
	}
	if withToken && token == "" {
		if token, err = authReadToken(os.Stdin, os.Stderr); err != nil {
			return err
		}
	}
	if withToken {
		err = authSignInToken(token)
	} else {
		err = authSignInWithAgent(flagSignInAgent, reporter)
	}
	if err != nil {
		return err
	}
	authHintActiveHub(config)
	return nil
}

// authReadToken reads a personal access token from stdin, for --with-token given without
// one: the whole of what is piped in, or one line typed at a terminal, which is asked for
func authReadToken(in io.Reader, prompt io.Writer) (string, error) {
	if f, isFile := in.(*os.File); isFile {
		if isatty.IsTerminal(f.Fd()) || isatty.IsCygwinTerminal(f.Fd()) {
			fmt.Fprint(prompt, "Personal access token: ")
			line, _ := bufio.NewReader(in).ReadString('\n')
			if token := strings.TrimSpace(line); token != "" {
				return token, nil
			}
			return "", errors.New("no token was entered")
		}
	}
	contents, err := io.ReadAll(in)
	if err != nil {
		return "", fmt.Errorf("reading the token from stdin: %w", err)
	}
	if token := strings.TrimSpace(string(contents)); token != "" {
		return token, nil
	}
	return "", errors.New("--with-token needs a personal access token, given after it or on stdin")
}

// authHintActiveHub says, after a login to a hub other than the active one, that requests
// still go to the active one, and how to change that
func authHintActiveHub(config *lib.ConfigSettings) {
	if active := lib.ConfigSavedHub(); config.Hub != active {
		fmt.Fprintf(os.Stderr, "the active hub is still %s - run '%s auth switch %s' to make %s active\n",
			active, cliName, config.Hub, config.Hub)
	}
}

// authLoginChoice reads the login switches and says which way to log in, and with what
// token when --with-token was given one, after it or as --with-token=token; "" means it
// is to be read from stdin.  Refused: a name for a token login, since a personal access
// token is named at Notehub, more than one token, a blank token or name, and any other
// argument.
func authLoginChoice(args []string) (withToken bool, token string, asJSON bool, err error) {
	withToken, asJSON, named := cli.SwitchGiven("with-token"), cli.SwitchGiven("json"), cli.SwitchGiven("name")
	token = flagWithToken.Value
	switch {
	case flagLoginExpiresIn <= 0:
		err = errors.New("--expires-in needs a number of seconds greater than zero")
	case withToken && named:
		err = errors.New("--name goes with a browser login: a personal access token already has its name at Notehub")
	case withToken && (len(args) > 1 || (len(args) == 1 && token != "")):
		given := args
		if token != "" {
			given = append([]string{token}, args...)
		}
		err = fmt.Errorf("--with-token takes one personal access token, but was given: %s", strings.Join(given, " "))
	case withToken && len(args) == 1 && strings.TrimSpace(args[0]) == "":
		err = errors.New("--with-token was given a blank token")
	case withToken && len(args) == 1:
		token = args[0]
	case len(args) != 0:
		err = fmt.Errorf("'%s login' takes no arguments, but was given: %s - to name the login, use --name \"Agent Name\"",
			cliName, strings.Join(args, " "))
	case named && strings.TrimSpace(flagSignInAgent) == "":
		err = errors.New("--name needs a nonempty name; quote names containing spaces")
	}
	token = strings.TrimSpace(token)
	return
}

// authLogoutCommand logs out of the active hub, or of the one given by --hub
func authLogoutCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("'%s logout' takes no arguments, but was given: %s - to log out of another hub, name it with --hub",
			cliName, strings.Join(args, " "))
	}
	if err := authLogout(config); err != nil {
		return fmt.Errorf("logout: %w", err)
	}
	return nil
}

// authLogout ends the login to the hub in use, which --signout does too
func authLogout(config *lib.ConfigSettings) error {
	if err := authEnvTokenInUse(config, "logging out would leave it in use"); err != nil {
		return err
	}
	user := ""
	if credentials := config.SavedCredentials(config.Hub); credentials != nil {
		user = " as " + credentials.User
	}
	if err := config.RemoveDefaultCredentials(); err != nil {
		return err
	}
	fmt.Fprintf(os.Stderr, "logged out of %s%s\n", config.Hub, user)
	return nil
}

// authStatusReport is the answer to 'notehub auth status --all': every hub with a login, and
// which one is active
type authStatusReport struct {
	Active string       `json:"active"`
	Hubs   []authStatus `json:"hubs"`
}

// authStatusCommand checks the connection in use, or with --all every saved login.
// The exit code says whether the hub in use accepts its login, except with --json,
// where the report says so and the exit code is 0.
func authStatusCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("'%s auth status' takes no arguments - use --all to check every saved login", cliName)
	}
	if flagAuthAll && cli.SwitchGiven("hub") {
		return errors.New("auth status: --all and --hub cannot be combined")
	}
	active := lib.ConfigSavedHub()
	report := authStatusAll(config, active, authStatusHubs(config, active, !flagAuthAll), time.Now())
	if flagJson {
		var result interface{} = report
		if !flagAuthAll {
			result = report.Hubs[0]
		}
		var reportJSON []byte
		var err error
		if flagPretty {
			reportJSON, err = note.JSONMarshalIndent(result, "", "    ")
		} else {
			reportJSON, err = note.JSONMarshal(result)
		}
		if err != nil {
			return err
		}
		fmt.Printf("%s\n", reportJSON)
		return nil
	}
	if config.EnvCredentials() != nil {
		fmt.Printf("%s is set, and its token is used for every request.\n\n", lib.ConfigTokenEnvVar)
	}
	for _, status := range report.Hubs {
		authStatusPrint(status)
	}
	for _, status := range report.Hubs {
		if status.Hub == config.Hub && !status.SignedIn {
			os.Exit(exitFail)
		}
	}
	return nil
}

// authStatusHubs returns either the connection in use alone, or the active hub first
// (even without a login) followed by every other hub with a login.
func authStatusHubs(config *lib.ConfigSettings, active string, onlyInUse bool) []string {
	if onlyInUse {
		return []string{config.Hub}
	}
	others := []string{}
	for hub := range config.HubCreds {
		if hub != active && config.SavedCredentials(hub) != nil {
			others = append(others, hub)
		}
	}
	sort.Strings(others)
	return append([]string{active}, others...)
}

// authStatusAll asks each hub whether it accepts its login
func authStatusAll(config *lib.ConfigSettings, active string, hubs []string, now time.Time) (report authStatusReport) {
	report.Active = active
	report.Hubs = []authStatus{}
	for _, hub := range hubs {
		status := authWhoAmI(config, hub, now)
		status.Active = hub == active
		report.Hubs = append(report.Hubs, status)
	}
	return
}

// authCredentialsFor returns the credentials that would be used for a hub: for the hub in
// use, those in the environment when NOTEHUB_TOKEN is set, and otherwise the saved login
func authCredentialsFor(config *lib.ConfigSettings, hub string) *lib.ConfigCreds {
	return config.CredentialsForHub(hub)
}

// authStatusPrint prints one hub's status: the hub, then its state beneath it
func authStatusPrint(status authStatus) {
	label := status.Hub
	if status.Active {
		label += " (active)"
	}
	fmt.Printf("%s\n", label)
	switch {
	case status.SignedIn:
		fmt.Printf("  logged in as %s via %s\n", status.User, authExpiresDescription(status))
	case status.Action != "":
		fmt.Printf("  %s - run '%s'\n", status.Reason, status.Action)
	default:
		fmt.Printf("  %s\n", status.Reason)
	}
}

// authSwitchCommand makes a hub you have logged in to the active one, or with no hub
// named, lists them
func authSwitchCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) > 1 {
		return fmt.Errorf("usage: %s auth switch [hub]", cliName)
	}
	active := lib.ConfigSavedHub()

	// The hubs to choose from, each with its login and when that expires, or expired
	if len(args) == 0 {
		hubs := authStatusHubs(config, active, false)
		width := 0
		for _, hub := range hubs {
			width = max(width, len(hub))
		}
		now := time.Now()
		for _, hub := range hubs {
			state := "not logged in"
			if credentials := config.SavedCredentials(hub); credentials != nil {
				state = credentials.User
				if expiration := credentials.ExpirationDescription(now); expiration != "" {
					state += ", " + expiration
				}
			}
			if hub == active {
				state += "  (active)"
			}
			fmt.Printf("  %-*s  %s\n", width, hub, state)
		}
		fmt.Printf("\nTo make another the active hub: %s auth switch <hub>\n", cliName)
		return nil
	}

	// A hub becomes active only once logged in to, unless the environment's token is
	// what every request uses
	hub := args[0]
	if hub == active {
		fmt.Printf("%s is already the active hub\n", hub)
		return nil
	}
	credentials := config.SavedCredentials(hub)
	if credentials == nil && config.EnvCredentials() == nil {
		others := []string{}
		for _, other := range authStatusHubs(config, active, false) {
			if config.SavedCredentials(other) != nil {
				others = append(others, other)
			}
		}
		hint := ""
		if len(others) != 0 {
			hint = fmt.Sprintf(" (the hubs you have logged in to are: %s)", strings.Join(others, ", "))
		}
		return fmt.Errorf("you haven't logged in to %s - run '%s login --hub %s' first%s", hub, cliName, hub, hint)
	}
	lib.ConfigSetHub(hub)
	config.AdoptHub()
	if err := config.Write(); err != nil {
		return err
	}
	as := ", with the token in " + lib.ConfigTokenEnvVar
	if credentials != nil {
		as = " as " + credentials.User
	}
	fmt.Printf("%s is now the active hub%s\n", hub, as)
	return nil
}

// authTokenCommand prints the token in use, for a program that needs it
func authTokenCommand(config *lib.ConfigSettings, args []string) error {
	if len(args) != 0 {
		return fmt.Errorf("'%s auth token' takes no arguments - for another hub's token, name it with --hub", cliName)
	}
	return authPrintToken(config)
}

// authPrintToken prints the token in use for the hub in use, which --token does too
func authPrintToken(config *lib.ConfigSettings) error {
	credentials := config.DefaultCredentials()
	if credentials == nil {
		return fmt.Errorf("not logged in to %s: set %s, or run '%s login'", config.Hub, lib.ConfigTokenEnvVar, cliName)
	}
	fmt.Printf("%s\n", credentials.Token)
	return nil
}

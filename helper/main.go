// The Remote Files Browser helper.
//
// Chrome extensions cannot open the raw network connections that FTP, FTPS and SFTP need. This small program does it
// for the extension. Chrome starts it (through "native messaging") when the extension asks, talks to it over standard
// input and output, and stops it when the extension page closes. It listens on no port and runs nothing in the background.
//
//	rocuo-files-helper                    double-click: install for the current user
//	rocuo-files-helper install            same, with options (--extension-id, --reset-extensions, --dir, --no-copy, --quiet)
//	rocuo-files-helper uninstall          remove the registration and the copy
//	rocuo-files-helper status             show what is installed and which extensions may use it
//	rocuo-files-helper chrome-extension://ID/   started by Chrome: speak the native messaging protocol
package main

import (
	"flag"
	"fmt"
	"os"
	"strings"
)

const messageTitle = "Remote Files Browser helper"

// haveOutput is true when what the commands print reaches someone (a terminal, a pipe or a file). It is false only
// when the Windows build was started without any terminal (the Run window, a shortcut): then results go in a message box.
var haveOutput = true

func main() {
	os.Exit(run(os.Args[1:]))
}

func isOrigin(arg string) bool {
	return strings.HasPrefix(arg, "chrome-extension://") || strings.HasPrefix(arg, "moz-extension://")
}

// say shows the result of a command: on the terminal when there is one, in a message box when there is not. A quiet run
// shows errors only.
func say(text string, isError, quiet bool) {
	if quiet && !isError {
		return
	}
	if !haveOutput {
		showMessage(messageTitle, text, isError)
		return
	}
	out := os.Stdout
	if isError {
		out = os.Stderr
	}
	fmt.Fprintln(out, strings.TrimRight(text, "\n"))
}

func run(args []string) int {
	if len(args) > 0 && isOrigin(args[0]) {
		return runHost()
	}
	if len(args) == 0 {
		return runSetup()
	}
	haveOutput = attachConsole()
	switch args[0] {
	case "install":
		return cmdInstall(args[1:])
	case "uninstall":
		return cmdUninstall(args[1:])
	case "status":
		say(statusReport(), false, false)
		return 0
	case "version", "--version", "-v":
		say(version, false, false)
		return 0
	default:
		isHelp := args[0] == "help" || args[0] == "-h" || args[0] == "--help"
		say(usage, !isHelp, false)
		if isHelp {
			return 0
		}
		return 2
	}
}

const usage = `Remote Files Browser helper

  rocuo-files-helper install [--extension-id ID] [--reset-extensions] [--dir FOLDER] [--no-copy] [--quiet]
  rocuo-files-helper uninstall [--dir FOLDER] [--quiet]
  rocuo-files-helper status
  rocuo-files-helper version

With no arguments the helper installs itself for the current user.
"install --extension-id ID" lets one more copy of the extension (for example one loaded from a folder) use the helper;
the addresses of an earlier setup are kept unless you add --reset-extensions.
"status" shows what is installed and which extensions may use it.
`

// runHost speaks the native messaging protocol until Chrome closes the pipe.
func runHost() (code int) {
	defer func() {
		if r := recover(); r != nil {
			fmt.Fprintln(os.Stderr, "helper stopped:", r)
			code = 2
		}
	}()
	s := newServer(os.Stdout)
	if err := s.run(os.Stdin); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

// runSetup is what happens on a double-click.
func runSetup() int {
	result, err := install(installOptions{})
	if err != nil {
		showMessage(messageTitle, "The setup did not work.\n\n"+err.Error(), true)
		return 1
	}
	text := "The helper is installed.\n\nGo back to Remote Files Browser in Chrome and press \"Check again\".\n\nInstalled in: " + result.Dir
	if extra := extraExtensions(result.Extensions); len(extra) > 0 {
		text += "\n\nIt also serves these copies of the extension (added earlier): " + strings.Join(extra, ", ")
	}
	showMessage(messageTitle, text, false)
	return 0
}

// extraExtensions are the allowed addresses that are not the published one.
func extraExtensions(ids []string) []string {
	var extra []string
	for _, id := range ids {
		published := false
		for _, p := range publishedExtensionIDs {
			published = published || id == p
		}
		if !published {
			extra = append(extra, id)
		}
	}
	return extra
}

type listFlag []string

func (l *listFlag) String() string { return strings.Join(*l, ",") }
func (l *listFlag) Set(v string) error {
	for _, part := range strings.Split(v, ",") {
		if part = strings.TrimSpace(part); part != "" {
			*l = append(*l, part)
		}
	}
	return nil
}

// installedText is what "install" says when it worked.
func installedText(result *installResult) string {
	var b strings.Builder
	fmt.Fprintf(&b, "The helper is installed in %s\n", result.Dir)
	fmt.Fprintf(&b, "Registered for: %s\n", strings.Join(result.Registered, ", "))
	b.WriteString("Extensions that may use it:\n")
	for _, id := range result.Extensions {
		fmt.Fprintf(&b, "  %s\n", describeExtension(id))
	}
	b.WriteString("\nGo back to Remote Files Browser in Chrome and press \"Check again\".")
	return b.String()
}

func cmdInstall(args []string) int {
	flags := flag.NewFlagSet("install", flag.ContinueOnError)
	var complaint strings.Builder // the flag package's own words, shown in the message (a terminal may not exist)
	flags.SetOutput(&complaint)
	var ids listFlag
	flags.Var(&ids, "extension-id", "address of an extra extension that may use the helper (a copy loaded from a folder)")
	reset := flags.Bool("reset-extensions", false, "forget the extra addresses of an earlier setup")
	dir := flags.String("dir", "", "folder to install into")
	noCopy := flags.Bool("no-copy", false, "register this program where it is, without copying it")
	quiet := flags.Bool("quiet", false, "print nothing but errors")
	if err := flags.Parse(args); err != nil {
		say("The options of install were not understood:\n"+strings.TrimSpace(complaint.String()), true, false)
		return 2
	}
	result, err := install(installOptions{ExtensionIDs: ids, ResetExtensions: *reset, Dir: *dir, NoCopy: *noCopy})
	if err != nil {
		say("Setup failed: "+err.Error(), true, false)
		return 1
	}
	say(installedText(result), false, *quiet)
	return 0
}

func cmdUninstall(args []string) int {
	flags := flag.NewFlagSet("uninstall", flag.ContinueOnError)
	var complaint strings.Builder
	flags.SetOutput(&complaint)
	dir := flags.String("dir", "", "folder the helper was installed into")
	quiet := flags.Bool("quiet", false, "print nothing but errors")
	if err := flags.Parse(args); err != nil {
		say("The options of uninstall were not understood:\n"+strings.TrimSpace(complaint.String()), true, false)
		return 2
	}
	if err := uninstall(installOptions{Dir: *dir}); err != nil {
		say("Removal failed: "+err.Error(), true, false)
		return 1
	}
	say("The helper is removed.", false, *quiet)
	return 0
}

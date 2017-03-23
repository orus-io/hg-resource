// Concourse Mercurial Resource
// Compiles to a single multi-call binary, similar to BusyBox
// This file dispatches to {check, in, out}.go

package main

import (
	"fmt"
	"io"
	"os"
	"path"
	"strings"
)

const (
	defaultBranch = "default"
	usageTemplate = "Usage: hgresource <%s> [arguments]"
)

type HandlerFunc func([]string, *JsonInput, io.Writer, io.Writer) int

type Command struct {
	Name    string
	Run     HandlerFunc
	NumArgs int
	Usage   func(string, io.Writer)
}

var commands = []*Command{
	cmdCheck,
	cmdIn,
	cmdOut,
}

func main() {
	status := run(os.Args, os.Stdin, os.Stdout, os.Stderr)
	os.Exit(status)
}

func run(args []string, inReader io.Reader, outWriter io.Writer, errWriter io.Writer) int {
	// try to dispatch by application name, then by first parameter
	handlerArgs := args[1:]
	appName, handler, err := getHandlerByAppName(args)
	if err != nil {
		appName, handler, err = getHandlerByFirstArgument(args)
		if err != nil {
			usage(errWriter)
			return 2
		} else {
			handlerArgs = args[2:]
		}
	}

	// print handler usage if required
	if len(handlerArgs) < handler.NumArgs {
		handler.Usage(appName, errWriter)
		return 2
	}

	input, err := parseInput(inReader)
	if err != nil {
		fmt.Fprintf(errWriter, "Error parsing input: %s\n", err)
		return 1
	}

	// run ssh-agent
	if len(input.Source.PrivateKey) != 0 {
		err := loadSshPrivateKey(input.Source.PrivateKey)
		defer cleanupSshAgent(os.Stderr)
		if err != nil {
			fmt.Fprintln(errWriter, err)
			return 1
		}
	}

	return handler.Run(handlerArgs, input, os.Stdout, os.Stderr)
}

func getHandlerByAppName(args []string) (appName string, handler *Command, err error) {
	appName = path.Base(args[0])
	handler, err = getHandler(appName)
	return
}

func getHandlerByFirstArgument(args []string) (appName string, handler *Command, err error) {
	if len(args) < 2 {
		err = fmt.Errorf("parameter required")
		return
	}
	handler, err = getHandler(args[1])
	appName = fmt.Sprintf("%s %s", args[0], args[1])
	return
}

func getHandler(name string) (*Command, error) {
	for _, cmd := range commands {
		if cmd.Name == name {
			return cmd, nil
		}
	}
	return nil, fmt.Errorf("command '%s' not found", name)
}

func makeUsage() string {
	var commandNames []string
	for _, cmd := range commands {
		commandNames = append(commandNames, cmd.Name)
	}

	joinedCommands := strings.Join(commandNames, "|")
	return fmt.Sprintf(usageTemplate, joinedCommands)
}

func usage(errWriter io.Writer) {
	fmt.Fprintln(errWriter, makeUsage())
}

func cleanupSshAgent(errWriter io.Writer) {
	err := killSshAgent()
	if err != nil {
		fmt.Fprintf(errWriter, "Error in cleanup: %s\n", err)
	}
}

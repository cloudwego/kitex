// Copyright 2024 CloudWeGo Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//   http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package util

import (
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
)

type Command struct {
	Use      string
	Short    string
	Long     string
	RunE     func(cmd *Command, args []string) error
	commands []*Command
	parent   *Command
	flags    *FlagSet
	// helpFunc is help func defined by user.
	helpFunc func(*Command, []string)
	// for debug
	args []string
}

// SetArgs sets arguments for the command. It is set to os.Args[1:] by default, if desired, can be overridden
// particularly useful when testing.
func (c *Command) SetArgs(a []string) {
	c.args = a
}

func (c *Command) AddCommand(cmds ...*Command) error {
	for i, x := range cmds {
		if cmds[i] == c {
			return fmt.Errorf("command can't be a child of itself")
		}
		cmds[i].parent = c
		c.commands = append(c.commands, x)
	}
	return nil
}

// Flags returns the FlagSet of the Command
func (c *Command) Flags() *FlagSet {
	if c.flags == nil {
		c.flags = NewFlagSet(c.Use, ContinueOnError)
	}
	return c.flags
}

// HasParent determines if the command is a child command.
func (c *Command) HasParent() bool {
	return c.parent != nil
}

// HasSubCommands determines if the command has children commands.
func (c *Command) HasSubCommands() bool {
	return len(c.commands) > 0
}

func stripFlags(args []string) []string {
	commands := make([]string, 0)
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if strings.HasPrefix(s, "-") {
			// handle "-f child child" args
			if len(args) <= 1 {
				break
			} else {
				args = args[1:]
				continue
			}
		} else if s != "" && !strings.HasPrefix(s, "-") {
			commands = append(commands, s)
		}
	}
	return commands
}

func (c *Command) findNext(next string) *Command {
	for _, cmd := range c.commands {
		if cmd.Use == next {
			return cmd
		}
	}
	return nil
}

func nextArgs(args []string, x string) []string {
	if len(args) == 0 {
		return args
	}
	for pos := 0; pos < len(args); pos++ {
		s := args[pos]
		switch {
		case strings.HasPrefix(s, "-"):
			pos++
			continue
		case !strings.HasPrefix(s, "-"):
			if s == x {
				// cannot use var ret []string cause it return nil
				ret := make([]string, 0)
				ret = append(ret, args[:pos]...)
				ret = append(ret, args[pos+1:]...)
				return ret
			}
		}
	}
	return args
}

func validateArgs(cmd *Command, args []string) error {
	// no subcommand, always take args
	if !cmd.HasSubCommands() {
		return nil
	}

	// root command with subcommands, do subcommand checking.
	if !cmd.HasParent() && len(args) > 0 {
		return fmt.Errorf("unknown command %q", args[0])
	}
	return nil
}

// Find the target command given the args and command tree
func (c *Command) Find(args []string) (*Command, []string, error) {
	var innerFind func(*Command, []string) (*Command, []string)

	innerFind = func(c *Command, innerArgs []string) (*Command, []string) {
		argsWithoutFlags := stripFlags(innerArgs)
		if len(argsWithoutFlags) == 0 {
			return c, innerArgs
		}
		nextSubCmd := argsWithoutFlags[0]

		cmd := c.findNext(nextSubCmd)
		if cmd != nil {
			return innerFind(cmd, nextArgs(innerArgs, nextSubCmd))
		}
		return c, innerArgs
	}
	commandFound, a := innerFind(c, args)
	return commandFound, a, validateArgs(commandFound, stripFlags(a))
}

// ParseFlags parses persistent flag tree and local flags.
func (c *Command) ParseFlags(args []string) error {
	err := c.Flags().Parse(args)
	return err
}

// SetHelpFunc sets help function. Can be defined by Application.
func (c *Command) SetHelpFunc(f func(*Command, []string)) {
	c.helpFunc = f
}

// HelpFunc returns either the function set by SetHelpFunc for this command
// or a parent, or it returns a function with default help behavior.
func (c *Command) HelpFunc() func(*Command, []string) {
	if c.helpFunc != nil {
		return c.helpFunc
	}
	if c.HasParent() {
		return c.parent.HelpFunc()
	}
	return nil
}

// PrintErrln is a convenience method to Println to the defined Err output, fallback to Stderr if not set.
func (c *Command) PrintErrln(i ...interface{}) {
	c.PrintErr(fmt.Sprintln(i...))
}

// PrintErr is a convenience method to Print to the defined Err output, fallback to Stderr if not set.
func (c *Command) PrintErr(i ...interface{}) {
	fmt.Fprint(c.ErrOrStderr(), i...)
}

// ErrOrStderr returns output to stderr
func (c *Command) ErrOrStderr() io.Writer {
	return c.getErr(os.Stderr)
}

func (c *Command) getErr(def io.Writer) io.Writer {
	if c.HasParent() {
		return c.parent.getErr(def)
	}
	return def
}

// ExecuteC executes the command.
func (c *Command) ExecuteC() (cmd *Command, err error) {
	args := c.args
	if c.args == nil {
		args = os.Args[1:]
	}
	cmd, flags, err := c.Find(args)
	if err != nil {
		return c, err
	}
	err = cmd.execute(flags)
	if err != nil {
		// Always show help if requested, even if SilenceErrors is in
		// effect
		if errors.Is(err, ErrHelp) {
			cmd.HelpFunc()(cmd, args)
			return cmd, err
		}
	}

	return cmd, err
}

func (c *Command) execute(a []string) error {
	if c == nil {
		return fmt.Errorf("called Execute() on a nil Command")
	}
	err := c.ParseFlags(a)
	if err != nil {
		return err
	}
	argWoFlags := c.Flags().Args()
	if c.RunE != nil {
		err := c.RunE(c, argWoFlags)
		if err != nil {
			return err
		}
	}
	return nil
}

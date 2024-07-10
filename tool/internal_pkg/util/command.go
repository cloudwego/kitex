package util

import (
	"flag"
	"github.com/cloudwego/kitex/tool/internal_pkg/log"
	"os"
)

type Command struct {
	Use      string
	Short    string
	Long     string
	RunE     func(cmd *Command, args []string) error
	commands []*Command
	parent   *Command
	flags    *flag.FlagSet
}

func (c *Command) AddCommand(cmds ...*Command) {
	for i, x := range cmds {
		if cmds[i] == c {
			panic("Command can't be a child of itself")
		}
		cmds[i].parent = c
		c.commands = append(c.commands, x)
	}
}

// Flags returns the FlagSet of the Command
func (c *Command) Flags() *flag.FlagSet {
	return c.flags
}

// PrintUsage prints the usage of the Command
func (c *Command) PrintUsage() {
	log.Warn("Usage: %s\n\n%s\n\n", c.Use, c.Long)
	c.flags.PrintDefaults()
	for _, cmd := range c.commands {
		log.Warnf("  %s: %s\n", cmd.Use, cmd.Short)
	}
}

// Parent returns a commands parent command.
func (c *Command) Parent() *Command {
	return c.parent
}

// HasParent determines if the command is a child command.
func (c *Command) HasParent() bool {
	return c.parent != nil
}

// Root finds root command.
func (c *Command) Root() *Command {
	if c.HasParent() {
		return c.Parent().Root()
	}
	return c
}

func (c *Command) Find(args []string) (*Command, []string, error) {
	if len(args) == 0 {
		return c, args, nil
	}

	for _, cmd := range c.commands {
		if cmd.Use == args[0] {
			return cmd.Find(args[1:])
		}
	}

	return c, args, nil
}

// ExecuteC executes the command.
func (c *Command) ExecuteC() (cmd *Command, err error) {
	args := os.Args[2:]
	// Regardless of what command execute is called on, run on Root only
	if c.HasParent() {
		return c.Root().ExecuteC()
	}

	cmd, flags, err := c.Find(args)
	if err != nil {
		log.Warn(err)
		return c, err
	}

	if cmd.RunE != nil {
		err = cmd.RunE(cmd, flags)
		if err != nil {
			log.Warn(err)
		}
	}

	return cmd, nil
}

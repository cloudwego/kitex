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
	"fmt"
	"reflect"
	"strings"
	"testing"
)

func emptyRun(*Command, []string) error { return nil }

func executeCommand(root *Command, args ...string) (err error) {
	_, err = executeCommandC(root, args...)
	return err
}

func executeCommandC(root *Command, args ...string) (c *Command, err error) {
	root.SetArgs(args)
	c, err = root.ExecuteC()
	return c, err
}

const onetwo = "one two"

func TestSingleCommand(t *testing.T) {
	rootCmd := &Command{
		Use:  "root",
		RunE: func(_ *Command, args []string) error { return nil },
	}
	aCmd := &Command{Use: "a", RunE: emptyRun}
	bCmd := &Command{Use: "b", RunE: emptyRun}
	rootCmd.AddCommand(aCmd, bCmd)

	_ = executeCommand(rootCmd, "one", "two")
}

func TestChildCommand(t *testing.T) {
	var child1CmdArgs []string
	rootCmd := &Command{Use: "root", RunE: emptyRun}
	child1Cmd := &Command{
		Use:  "child1",
		RunE: func(_ *Command, args []string) error { child1CmdArgs = args; return nil },
	}
	child2Cmd := &Command{Use: "child2", RunE: emptyRun}
	rootCmd.AddCommand(child1Cmd, child2Cmd)

	err := executeCommand(rootCmd, "child1", "one", "two")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	got := strings.Join(child1CmdArgs, " ")
	if got != onetwo {
		t.Errorf("child1CmdArgs expected: %q, got: %q", onetwo, got)
	}
}

func TestCallCommandWithoutSubcommands(t *testing.T) {
	rootCmd := &Command{Use: "root", RunE: emptyRun}
	err := executeCommand(rootCmd)
	if err != nil {
		t.Errorf("Calling command without subcommands should not have error: %v", err)
	}
}

func TestRootExecuteUnknownCommand(t *testing.T) {
	rootCmd := &Command{Use: "root", RunE: emptyRun}
	rootCmd.AddCommand(&Command{Use: "child", RunE: emptyRun})

	_ = executeCommand(rootCmd, "unknown")
}

func TestSubcommandExecuteC(t *testing.T) {
	rootCmd := &Command{Use: "root", RunE: emptyRun}
	childCmd := &Command{Use: "child", RunE: emptyRun}
	rootCmd.AddCommand(childCmd)

	_, err := executeCommandC(rootCmd, "child")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

func TestFind(t *testing.T) {
	var foo, bar string
	root := &Command{
		Use: "root",
	}
	root.Flags().StringVarP(&foo, "foo", "f", "", "")
	root.Flags().StringVarP(&bar, "bar", "b", "something", "")

	child := &Command{
		Use: "child",
	}
	root.AddCommand(child)

	testCases := []struct {
		args              []string
		expectedFoundArgs []string
	}{
		{
			[]string{"child"},
			[]string{},
		},
		{
			[]string{"child", "child"},
			[]string{"child"},
		},
		{
			[]string{"child", "foo", "child", "bar", "child", "baz", "child"},
			[]string{"foo", "child", "bar", "child", "baz", "child"},
		},
		{
			[]string{"-f", "child", "child"},
			[]string{"-f", "child"},
		},
		{
			[]string{"child", "-f", "child"},
			[]string{"-f", "child"},
		},
		{
			[]string{"-b", "child", "child"},
			[]string{"-b", "child"},
		},
		{
			[]string{"child", "-b", "child"},
			[]string{"-b", "child"},
		},
		{
			[]string{"child", "-b"},
			[]string{"-b"},
		},
		{
			[]string{"-b", "-f", "child", "child"},
			[]string{"-b", "-f", "child"},
		},
		{
			[]string{"-f", "child", "-b", "something", "child"},
			[]string{"-f", "child", "-b", "something"},
		},
		{
			[]string{"-f", "child", "child", "-b"},
			[]string{"-f", "child", "-b"},
		},
		{
			[]string{"-f=child", "-b=something", "child"},
			[]string{"-f=child", "-b=something"},
		},
		{
			[]string{"--foo", "child", "--bar", "something", "child"},
			[]string{"--foo", "child", "--bar", "something"},
		},
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("%v", tc.args), func(t *testing.T) {
			cmd, foundArgs, err := root.Find(tc.args)
			if err != nil {
				t.Fatal(err)
			}

			if cmd != child {
				t.Fatal("Expected cmd to be child, but it was not")
			}

			if !reflect.DeepEqual(tc.expectedFoundArgs, foundArgs) {
				t.Fatalf("Wrong args\nExpected: %v\nGot: %v", tc.expectedFoundArgs, foundArgs)
			}
		})
	}
}

func TestFlagLong(t *testing.T) {
	var cArgs []string
	c := &Command{
		Use:  "c",
		RunE: func(_ *Command, args []string) error { cArgs = args; return nil },
	}

	var stringFlagValue string
	c.Flags().StringVar(&stringFlagValue, "sf", "", "")

	err := executeCommand(c, "--sf=abc", "one", "--", "two")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if stringFlagValue != "abc" {
		t.Errorf("Expected stringFlagValue: %q, got %q", "abc", stringFlagValue)
	}

	got := strings.Join(cArgs, " ")
	if got != onetwo {
		t.Errorf("rootCmdArgs expected: %q, got: %q", onetwo, got)
	}
}

func TestFlagShort(t *testing.T) {
	var cArgs []string
	c := &Command{
		Use:  "c",
		RunE: func(_ *Command, args []string) error { cArgs = args; return nil },
	}

	var stringFlagValue string
	c.Flags().StringVarP(&stringFlagValue, "sf", "s", "", "")

	err := executeCommand(c, "-sabc", "one", "two")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}

	if stringFlagValue != "abc" {
		t.Errorf("Expected stringFlagValue: %q, got %q", "abc", stringFlagValue)
	}

	got := strings.Join(cArgs, " ")
	if got != onetwo {
		t.Errorf("rootCmdArgs expected: %q, got: %q", onetwo, got)
	}
}

func TestChildFlag(t *testing.T) {
	rootCmd := &Command{Use: "root", RunE: emptyRun}
	childCmd := &Command{Use: "child", RunE: emptyRun}
	rootCmd.AddCommand(childCmd)
	err := executeCommand(rootCmd, "child")
	if err != nil {
		t.Errorf("Unexpected error: %v", err)
	}
}

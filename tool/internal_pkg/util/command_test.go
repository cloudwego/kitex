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
	"os"
	"testing"
)

func TestAddCommand(t *testing.T) {
	rootCmd := &Command{Use: "root"}
	childCmd := &Command{Use: "child"}

	// Test adding a valid child command
	err := rootCmd.AddCommand(childCmd)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if len(rootCmd.commands) != 1 {
		t.Fatalf("expected 1 command, got %d", len(rootCmd.commands))
	}

	if rootCmd.commands[0] != childCmd {
		t.Fatalf("expected child command to be added")
	}

	// Test adding a command to itself
	err = rootCmd.AddCommand(rootCmd)
	if err == nil {
		t.Fatalf("expected an error, got nil")
	}

	expectedErr := "command can't be a child of itself"
	if err.Error() != expectedErr {
		t.Fatalf("expected error %q, got %q", expectedErr, err.Error())
	}
}

func TestExecuteC(t *testing.T) {
	rootCmd := &Command{
		Use: "root",
		RunE: func(cmd *Command, args []string) error {
			return nil
		},
	}

	subCmd := &Command{
		Use: "sub",
		RunE: func(cmd *Command, args []string) error {
			return nil
		},
	}

	rootCmd.AddCommand(subCmd)

	// Simulate command line arguments
	os.Args = []string{"root", "sub"}

	// Execute the command
	cmd, err := rootCmd.ExecuteC()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	if cmd.Use != "sub" {
		t.Fatalf("expected sub command to be executed, got %s", cmd.Use)
	}

	// Simulate command line arguments with an unknown command
	os.Args = []string{"root", "unknown"}

	_, err = rootCmd.ExecuteC()
	if err == nil {
		t.Fatalf("expected an error for unknown command, got nil")
	}
}

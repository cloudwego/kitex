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
	"io"
	"os"
	"testing"
)

func ResetForTesting() {
	CommandLine = &FlagSet{
		name:          os.Args[0],
		errorHandling: ContinueOnError,
		output:        io.Discard,
	}
}

// GetCommandLine returns the default FlagSet.
func GetCommandLine() *FlagSet {
	return CommandLine
}

func TestParse(t *testing.T) {
	ResetForTesting()
	testParse(GetCommandLine(), t)
}

func testParse(f *FlagSet, t *testing.T) {
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	boolFlag := f.Bool("bool", false, "bool value")
	bool2Flag := f.Bool("bool2", false, "bool2 value")
	bool3Flag := f.Bool("bool3", false, "bool3 value")
	stringFlag := f.String("string", "0", "string value")
	extra := "one-extra-argument"
	args := []string{
		"--bool",
		"--bool2=true",
		"--bool3=false",
		"--string=hello",
		extra,
	}
	if err := f.Parse(args); err != nil {
		t.Fatal(err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *boolFlag != true {
		t.Error("bool flag should be true, is ", *boolFlag)
	}
	if *bool2Flag != true {
		t.Error("bool2 flag should be true, is ", *bool2Flag)
	}
	if *bool3Flag != false {
		t.Error("bool3 flag should be false, is ", *bool2Flag)
	}
	if *stringFlag != "hello" {
		t.Error("string flag should be `hello`, is ", *stringFlag)
	}
	if len(f.Args()) != 1 {
		t.Error("expected one argument, got", len(f.Args()))
	} else if f.Args()[0] != extra {
		t.Errorf("expected argument %q got %q", extra, f.Args()[0])
	}
}

func TestShorthand(t *testing.T) {
	f := NewFlagSet("shorthand", ContinueOnError)
	if f.Parsed() {
		t.Error("f.Parse() = true before Parse")
	}
	boolaFlag := f.BoolP("boola", "a", false, "bool value")
	boolbFlag := f.BoolP("boolb", "b", false, "bool2 value")
	boolcFlag := f.BoolP("boolc", "c", false, "bool3 value")
	booldFlag := f.BoolP("boold", "d", false, "bool4 value")
	stringaFlag := f.StringP("stringa", "s", "0", "string value")
	stringzFlag := f.StringP("stringz", "z", "0", "string value")
	extra := "interspersed-argument"
	notaflag := "--i-look-like-a-flag"
	args := []string{
		"-ab",
		extra,
		"-cs",
		"hello",
		"-z=something",
		"-d=true",
		"--",
		notaflag,
	}
	f.SetOutput(io.Discard)
	if err := f.Parse(args); err != nil {
		t.Error("expected no error, got ", err)
	}
	if !f.Parsed() {
		t.Error("f.Parse() = false after Parse")
	}
	if *boolaFlag != true {
		t.Error("boola flag should be true, is ", *boolaFlag)
	}
	if *boolbFlag != true {
		t.Error("boolb flag should be true, is ", *boolbFlag)
	}
	if *boolcFlag != true {
		t.Error("boolc flag should be true, is ", *boolcFlag)
	}
	if *booldFlag != true {
		t.Error("boold flag should be true, is ", *booldFlag)
	}
	if *stringaFlag != "hello" {
		t.Error("stringa flag should be `hello`, is ", *stringaFlag)
	}
	if *stringzFlag != "something" {
		t.Error("stringz flag should be `something`, is ", *stringzFlag)
	}
	if len(f.Args()) != 2 {
		t.Error("expected one argument, got", len(f.Args()))
	} else if f.Args()[0] != extra {
		t.Errorf("expected argument %q got %q", extra, f.Args()[0])
	} else if f.Args()[1] != notaflag {
		t.Errorf("expected argument %q got %q", notaflag, f.Args()[1])
	}
}

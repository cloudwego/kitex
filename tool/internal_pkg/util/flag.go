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
	"bytes"
	"encoding/csv"
	"errors"
	goflag "flag"
	"fmt"
	"io"
	"os"
	"strconv"
	"strings"
)

// ErrHelp is the error returned if the flag -help is invoked but no such flag is defined.
var ErrHelp = errors.New("pflag: help requested")

// ErrorHandling defines how to handle flag parsing errors.
type ErrorHandling int

const (
	// ContinueOnError will return an err from Parse() if an error is found
	ContinueOnError ErrorHandling = iota
	// ExitOnError will call os.Exit(2) if an error is found when parsing
	ExitOnError
	// PanicOnError will panic() if an error is found when parsing flags
	PanicOnError
)

// ParseErrorsWhitelist defines the parsing errors that can be ignored
type ParseErrorsWhitelist struct {
	// UnknownFlags will ignore unknown flags errors and continue parsing rest of the flags
	UnknownFlags bool
}

// NormalizedName is a flag name that has been normalized according to rules
// for the FlagSet (e.g. making '-' and '_' equivalent).
type NormalizedName string

// A FlagSet represents a set of defined flags.
type FlagSet struct {
	// Usage is the function called when an error occurs while parsing flags.
	// The field is a function (not a method) that may be changed to point to
	// a custom error handler.
	Usage func()

	// SortFlags is used to indicate, if user wants to have sorted flags in
	// help/usage messages.
	SortFlags bool

	// ParseErrorsWhitelist is used to configure a whitelist of errors
	ParseErrorsWhitelist ParseErrorsWhitelist

	name              string
	parsed            bool
	actual            map[NormalizedName]*Flag
	orderedActual     []*Flag
	formal            map[NormalizedName]*Flag
	orderedFormal     []*Flag
	shorthands        map[byte]*Flag
	args              []string // arguments after flags
	argsLenAtDash     int      // len(args) when a '--' was located when parsing, or -1 if no --
	errorHandling     ErrorHandling
	output            io.Writer // nil means stderr; use out() accessor
	interspersed      bool      // allow interspersed option/non-option args
	normalizeNameFunc func(f *FlagSet, name string) NormalizedName

	addedGoFlagSets []*goflag.FlagSet
}

// A Flag represents the state of a flag.
type Flag struct {
	Name                string // name as it appears on command line
	Shorthand           string // one-letter abbreviated flag
	Usage               string // help message
	Value               Value  // value as set
	DefValue            string // default value (as text); for usage message
	Changed             bool   // If the user set the value (or if left to default)
	NoOptDefVal         string // default value (as text); if the flag is on the command line without any options
	Deprecated          string // If this flag is deprecated, this string is the new or now thing to use
	ShorthandDeprecated string // If the shorthand of this flag is deprecated, this string is the new or now thing to use
}

// Value is the interface to the dynamic value stored in a flag.
// (The default value is represented as a string.)
type Value interface {
	String() string
	Set(string) error
}

// SliceValue is a secondary interface to all flags which hold a list
// of values.  This allows full control over the value of list flags,
// and avoids complicated marshalling and unmarshalling to csv.
type SliceValue interface {
	// Append adds the specified value to the end of the flag value list.
	Append(string) error
	// Replace will fully overwrite any data currently in the flag value list.
	Replace([]string) error
	// GetSlice returns the flag value list as an array of strings.
	GetSlice() []string
}

// GetNormalizeFunc returns the previously set NormalizeFunc of a function which
// does no translation, if not set previously.
func (f *FlagSet) GetNormalizeFunc() func(f *FlagSet, name string) NormalizedName {
	if f.normalizeNameFunc != nil {
		return f.normalizeNameFunc
	}
	return func(f *FlagSet, name string) NormalizedName { return NormalizedName(name) }
}

func (f *FlagSet) normalizeFlagName(name string) NormalizedName {
	n := f.GetNormalizeFunc()
	return n(f, name)
}

// Lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) Lookup(name string) *Flag {
	return f.lookup(f.normalizeFlagName(name))
}

// lookup returns the Flag structure of the named flag, returning nil if none exists.
func (f *FlagSet) lookup(name NormalizedName) *Flag {
	return f.formal[name]
}

func (f *FlagSet) out() io.Writer {
	if f.output == nil {
		return os.Stderr
	}
	return f.output
}

// SetOutput sets the destination for usage and error messages.
// If output is nil, os.Stderr is used.
func (f *FlagSet) SetOutput(output io.Writer) {
	f.output = output
}

// Set sets the value of the named flag.
func (f *FlagSet) Set(name, value string) error {
	normalName := f.normalizeFlagName(name)
	flag, ok := f.formal[normalName]
	if !ok {
		return fmt.Errorf("no such flag -%v", name)
	}

	err := flag.Value.Set(value)
	if err != nil {
		var flagName string
		if flag.Shorthand != "" && flag.ShorthandDeprecated == "" {
			flagName = fmt.Sprintf("-%s, --%s", flag.Shorthand, flag.Name)
		} else {
			flagName = fmt.Sprintf("--%s", flag.Name)
		}
		return fmt.Errorf("invalid argument %q for %q flag: %v", value, flagName, err)
	}

	if !flag.Changed {
		if f.actual == nil {
			f.actual = make(map[NormalizedName]*Flag)
		}
		f.actual[normalName] = flag
		f.orderedActual = append(f.orderedActual, flag)

		flag.Changed = true
	}

	if flag.Deprecated != "" {
		fmt.Fprintf(f.out(), "Flag --%s has been deprecated, %s\n", flag.Name, flag.Deprecated)
	}
	return nil
}

// Args returns the non-flag arguments.
func (f *FlagSet) Args() []string { return f.args }

// VarPF is like VarP, but returns the flag created
func (f *FlagSet) VarPF(value Value, name, shorthand, usage string) (*Flag, error) {
	// Remember the default value as a string; it won't change.
	flag := &Flag{
		Name:      name,
		Shorthand: shorthand,
		Usage:     usage,
		Value:     value,
		DefValue:  value.String(),
	}
	err := f.AddFlag(flag)
	if err != nil {
		return nil, err
	}
	return flag, nil
}

// VarP is like Var, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) VarP(value Value, name, shorthand, usage string) {
	f.VarPF(value, name, shorthand, usage)
}

// AddFlag will add the flag to the FlagSet
func (f *FlagSet) AddFlag(flag *Flag) error {
	normalizedFlagName := f.normalizeFlagName(flag.Name)

	_, alreadyThere := f.formal[normalizedFlagName]
	if alreadyThere {
		msg := fmt.Sprintf("%s flag redefined: %s", f.name, flag.Name)
		fmt.Fprintln(f.out(), msg)
		return fmt.Errorf("%s flag redefined: %s", f.name, flag.Name)
	}
	if f.formal == nil {
		f.formal = make(map[NormalizedName]*Flag)
	}

	flag.Name = string(normalizedFlagName)
	f.formal[normalizedFlagName] = flag
	f.orderedFormal = append(f.orderedFormal, flag)

	if flag.Shorthand == "" {
		return nil
	}
	if len(flag.Shorthand) > 1 {
		fmt.Fprintf(f.out(), "%q shorthand is more than one ASCII character", flag.Shorthand)
		return fmt.Errorf("%q shorthand is more than one ASCII character", flag.Shorthand)
	}
	if f.shorthands == nil {
		f.shorthands = make(map[byte]*Flag)
	}
	c := flag.Shorthand[0]
	used, alreadyThere := f.shorthands[c]
	if alreadyThere {
		fmt.Fprintf(f.out(), "unable to redefine %q shorthand in %q flagset: it's already used for %q flag", c, f.name, used.Name)
		return fmt.Errorf("unable to redefine %q shorthand in %q flagset: it's already used for %q flag", c, f.name, used.Name)
	}
	f.shorthands[c] = flag
	return nil
}

// failf prints to standard error a formatted error and usage message and
// returns the error.
func (f *FlagSet) failf(format string, a ...interface{}) error {
	err := fmt.Errorf(format, a...)
	if f.errorHandling != ContinueOnError {
		fmt.Fprintln(f.out(), err)
	}
	return err
}

// --unknown (args will be empty)
// --unknown --next-flag ... (args will be --next-flag ...)
// --unknown arg ... (args will be arg ...)
func stripUnknownFlagValue(args []string) []string {
	if len(args) == 0 {
		//--unknown
		return args
	}

	first := args[0]
	if len(first) > 0 && first[0] == '-' {
		//--unknown --next-flag ...
		return args
	}

	//--unknown arg ... (args will be arg ...)
	if len(args) > 1 {
		return args[1:]
	}
	return nil
}

func (f *FlagSet) parseLongArg(s string, args []string, fn parseFunc) (a []string, err error) {
	a = args
	name := s[2:]
	if len(name) == 0 || name[0] == '-' || name[0] == '=' {
		err = f.failf("bad flag syntax: %s", s)
		return
	}

	split := strings.SplitN(name, "=", 2)
	name = split[0]
	flag, exists := f.formal[f.normalizeFlagName(name)]

	if !exists {
		switch {
		case name == "help":
			return a, ErrHelp
		case f.ParseErrorsWhitelist.UnknownFlags:
			// --unknown=unknownval arg ...
			// we do not want to lose arg in this case
			if len(split) >= 2 {
				return a, nil
			}

			return stripUnknownFlagValue(a), nil
		default:
			err = f.failf("unknown flag: --%s", name)
			return
		}
	}

	var value string
	if len(split) == 2 {
		// '--flag=arg'
		value = split[1]
	} else if flag.NoOptDefVal != "" {
		// '--flag' (arg was optional)
		value = flag.NoOptDefVal
	} else if len(a) > 0 {
		// '--flag arg'
		value = a[0]
		a = a[1:]
	} else {
		// '--flag' (arg was required)
		err = f.failf("flag needs an argument: %s", s)
		return
	}

	err = fn(flag, value)
	if err != nil {
		f.failf(err.Error())
	}
	return
}

func (f *FlagSet) parseSingleShortArg(shorthands string, args []string, fn parseFunc) (outShorts string, outArgs []string, err error) {
	outArgs = args

	if strings.HasPrefix(shorthands, "test.") {
		return
	}

	outShorts = shorthands[1:]
	c := shorthands[0]

	flag, exists := f.shorthands[c]
	if !exists {
		switch {
		case c == 'h':
			err = ErrHelp
			return
		case f.ParseErrorsWhitelist.UnknownFlags:
			// '-f=arg arg ...'
			// we do not want to lose arg in this case
			if len(shorthands) > 2 && shorthands[1] == '=' {
				outShorts = ""
				return
			}

			outArgs = stripUnknownFlagValue(outArgs)
			return
		default:
			err = f.failf("unknown shorthand flag: %q in -%s", c, shorthands)
			return
		}
	}

	var value string
	if len(shorthands) > 2 && shorthands[1] == '=' {
		// '-f=arg'
		value = shorthands[2:]
		outShorts = ""
	} else if flag.NoOptDefVal != "" {
		// '-f' (arg was optional)
		value = flag.NoOptDefVal
	} else if len(shorthands) > 1 {
		// '-farg'
		value = shorthands[1:]
		outShorts = ""
	} else if len(args) > 0 {
		// '-f arg'
		value = args[0]
		outArgs = args[1:]
	} else {
		// '-f' (arg was required)
		err = f.failf("flag needs an argument: %q in -%s", c, shorthands)
		return
	}

	if flag.ShorthandDeprecated != "" {
		fmt.Fprintf(f.out(), "Flag shorthand -%s has been deprecated, %s\n", flag.Shorthand, flag.ShorthandDeprecated)
	}

	err = fn(flag, value)
	if err != nil {
		f.failf(err.Error())
	}
	return
}

func (f *FlagSet) parseShortArg(s string, args []string, fn parseFunc) (a []string, err error) {
	a = args
	shorthands := s[1:]

	// "shorthands" can be a series of shorthand letters of flags (e.g. "-vvv").
	for len(shorthands) > 0 {
		shorthands, a, err = f.parseSingleShortArg(shorthands, args, fn)
		if err != nil {
			return
		}
	}

	return
}

func (f *FlagSet) parseArgs(args []string, fn parseFunc) (err error) {
	for len(args) > 0 {
		s := args[0]
		args = args[1:]
		if len(s) == 0 || s[0] != '-' || len(s) == 1 {
			if !f.interspersed {
				f.args = append(f.args, s)
				f.args = append(f.args, args...)
				return nil
			}
			f.args = append(f.args, s)
			continue
		}

		if s[1] == '-' {
			if len(s) == 2 { // "--" terminates the flags
				f.argsLenAtDash = len(f.args)
				f.args = append(f.args, args...)
				break
			}
			args, err = f.parseLongArg(s, args, fn)
		} else {
			args, err = f.parseShortArg(s, args, fn)
		}
		if err != nil {
			return
		}
	}
	return
}

// Parse parses flag definitions from the argument list, which should not
// include the command name.  Must be called after all flags in the FlagSet
// are defined and before flags are accessed by the program.
// The return value will be ErrHelp if -help was set but not defined.
func (f *FlagSet) Parse(arguments []string) error {
	if f.addedGoFlagSets != nil {
		for _, goFlagSet := range f.addedGoFlagSets {
			goFlagSet.Parse(nil)
		}
	}
	f.parsed = true

	f.args = make([]string, 0, len(arguments))

	set := func(flag *Flag, value string) error {
		return f.Set(flag.Name, value)
	}

	err := f.parseArgs(arguments, set)
	if err != nil {
		switch f.errorHandling {
		case ContinueOnError:
			return err
		case ExitOnError:
			fmt.Println(err)
			os.Exit(2)
		case PanicOnError:
			panic(err)
		}
	}
	return nil
}

type parseFunc func(flag *Flag, value string) error

// Parsed reports whether f.Parse has been called.
func (f *FlagSet) Parsed() bool {
	return f.parsed
}

var CommandLine = NewFlagSet(os.Args[0], ExitOnError)

// NewFlagSet returns a new, empty flag set with the specified name,
// error handling property and SortFlags set to true.
func NewFlagSet(name string, errorHandling ErrorHandling) *FlagSet {
	f := &FlagSet{
		name:          name,
		errorHandling: errorHandling,
		argsLenAtDash: -1,
		interspersed:  true,
		SortFlags:     true,
	}
	return f
}

// -- string Value
type stringValue string

func newStringValue(val string, p *string) *stringValue {
	*p = val
	return (*stringValue)(p)
}

func (s *stringValue) Set(val string) error {
	*s = stringValue(val)
	return nil
}

func (s *stringValue) String() string { return string(*s) }

// StringVar defines a string flag with specified name, default value, and usage string.
func (f *FlagSet) StringVar(p *string, name, value, usage string) {
	f.VarP(newStringValue(value, p), name, "", usage)
}

// StringVarP is like StringVar, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) StringVarP(p *string, name, shorthand, value, usage string) {
	f.VarP(newStringValue(value, p), name, shorthand, usage)
}

// String defines a string flag with specified name, default value, and usage string.
// The return value is the address of a string variable that stores the value of the flag.
func (f *FlagSet) String(name, value, usage string) *string {
	p := new(string)
	f.StringVarP(p, name, "", value, usage)
	return p
}

// StringP is like String, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) StringP(name, shorthand, value, usage string) *string {
	p := new(string)
	f.StringVarP(p, name, shorthand, value, usage)
	return p
}

// -- bool Value
type boolValue bool

func newBoolValue(val bool, p *bool) *boolValue {
	*p = val
	return (*boolValue)(p)
}

func (b *boolValue) Set(s string) error {
	v, err := strconv.ParseBool(s)
	*b = boolValue(v)
	return err
}

func (b *boolValue) String() string { return strconv.FormatBool(bool(*b)) }

func (b *boolValue) IsBoolFlag() bool { return true }

// BoolVar defines a bool flag with specified name, default value, and usage string.
// The argument p points to a bool variable in which to store the value of the flag.
func (f *FlagSet) BoolVar(p *bool, name string, value bool, usage string) error {
	err := f.BoolVarP(p, name, "", value, usage)
	if err != nil {
		return err
	}
	return nil
}

// BoolVarP is like BoolVar, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) BoolVarP(p *bool, name, shorthand string, value bool, usage string) error {
	flag, err := f.VarPF(newBoolValue(value, p), name, shorthand, usage)
	if err != nil {
		return err
	}
	flag.NoOptDefVal = "true"
	return nil
}

// Bool defines a bool flag with specified name, default value, and usage string.
// The return value is the address of a bool variable that stores the value of the flag.
func (f *FlagSet) Bool(name string, value bool, usage string) *bool {
	return f.BoolP(name, "", value, usage)
}

// BoolP is like Bool, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) BoolP(name, shorthand string, value bool, usage string) *bool {
	p := new(bool)
	f.BoolVarP(p, name, shorthand, value, usage)
	return p
}

// -- int Value
type intValue int

func newIntValue(val int, p *int) *intValue {
	*p = val
	return (*intValue)(p)
}

func (i *intValue) Set(s string) error {
	v, err := strconv.ParseInt(s, 0, 64)
	*i = intValue(v)
	return err
}

func (i *intValue) String() string { return strconv.Itoa(int(*i)) }

// IntVar defines an int flag with specified name, default value, and usage string.
// The argument p points to an int variable in which to store the value of the flag.
func (f *FlagSet) IntVar(p *int, name string, value int, usage string) {
	f.VarP(newIntValue(value, p), name, "", usage)
}

// IntVarP is like IntVar, but accepts a shorthand letter that can be used after a single dash.
func (f *FlagSet) IntVarP(p *int, name, shorthand string, value int, usage string) {
	f.VarP(newIntValue(value, p), name, shorthand, usage)
}

func (f *FlagSet) Int(name string, value int, usage string) *int {
	p := new(int)
	f.IntVarP(p, name, "", value, usage)
	return p
}

// -- stringArray Value
type stringArrayValue struct {
	value   *[]string
	changed bool
}

func newStringArrayValue(val []string, p *[]string) *stringArrayValue {
	ssv := new(stringArrayValue)
	ssv.value = p
	*ssv.value = val
	return ssv
}

func (s *stringArrayValue) Set(val string) error {
	if !s.changed {
		*s.value = []string{val}
		s.changed = true
	} else {
		*s.value = append(*s.value, val)
	}
	return nil
}

func writeAsCSV(vals []string) (string, error) {
	b := &bytes.Buffer{}
	w := csv.NewWriter(b)
	err := w.Write(vals)
	if err != nil {
		return "", err
	}
	w.Flush()
	return strings.TrimSuffix(b.String(), "\n"), nil
}

func (s *stringArrayValue) String() string {
	str, _ := writeAsCSV(*s.value)
	return "[" + str + "]"
}

// StringArrayVar defines a string flag with specified name, default value, and usage string.
// The argument p points to a []string variable in which to store the values of the multiple flags.
// The value of each argument will not try to be separated by comma. Use a StringSlice for that.
func (f *FlagSet) StringArrayVar(p *[]string, name string, value []string, usage string) {
	f.VarP(newStringArrayValue(value, p), name, "", usage)
}
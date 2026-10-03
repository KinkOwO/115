package main

import (
	"bytes"
	"flag"
	"os"
	"reflect"
	"strings"
	"testing"
)

func TestDispatchPreservesStandaloneArgumentsAndFlags(t *testing.T) {
	originalCommands, originalArgs, originalFlags := commands, os.Args, flag.CommandLine
	defer func() { commands = originalCommands }()
	called := false
	commands = []command{{name: "fixture", flags: true, run: func() {
		called = true
		path := flag.String("input", "", "input")
		flag.Parse()
		if *path != "a path/file.bin" || !reflect.DeepEqual(flag.Args(), []string{"tail"}) {
			t.Fatalf("flag arguments lost: %q %v", *path, flag.Args())
		}
		if !reflect.DeepEqual(os.Args, []string{"dfo-tool fixture", "-input", "a path/file.bin", "tail"}) {
			t.Fatalf("positional arguments shifted: %v", os.Args)
		}
	}}}
	var out, err bytes.Buffer
	if code := dispatch([]string{"fixture", "-input", "a path/file.bin", "tail"}, &out, &err); code != 0 || !called {
		t.Fatalf("dispatch failed: %d %s", code, err.String())
	}
	if !reflect.DeepEqual(os.Args, originalArgs) || flag.CommandLine != originalFlags {
		t.Fatal("dispatch leaked process argument or flag state")
	}
}

func TestHelpAndInvalidCommandsNeverExecuteTools(t *testing.T) {
	originalCommands := commands
	defer func() { commands = originalCommands }()
	commands = []command{{name: "fixture", group: "protocol", usage: "<input>", minimumArgs: 1, run: func() {
		t.Fatal("help or invalid arguments executed a tool")
	}}}
	for _, tc := range []struct {
		args []string
		code int
	}{
		{nil, 0}, {[]string{"-h"}, 0}, {[]string{"help"}, 0},
		{[]string{"unknown"}, 2}, {[]string{"fixture"}, 2},
		{[]string{"fixture", "-h"}, 0}, {[]string{"fixture", "--help"}, 0},
	} {
		var out, err bytes.Buffer
		if code := dispatch(tc.args, &out, &err); code != tc.code {
			t.Fatalf("%v: code %d, want %d", tc.args, code, tc.code)
		}
	}
}

func TestRegistryHasUniqueNamesAndVisibleHelp(t *testing.T) {
	var help bytes.Buffer
	printHelp(&help)
	seen := map[string]bool{}
	for _, c := range commands {
		if seen[c.name] || c.name == "" || c.run == nil {
			t.Fatalf("invalid command registration: %q", c.name)
		}
		seen[c.name] = true
		if !strings.Contains(help.String(), "  "+c.name+" ") {
			t.Fatalf("command missing from help: %s", c.name)
		}
	}
}

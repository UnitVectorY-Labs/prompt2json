package main

import (
	"os"
	"regexp"
	"strings"
	"testing"
)

func TestHelpColors(t *testing.T) {
	for _, test := range []struct {
		name     string
		terminal bool
		noColor  bool
		term     string
		want     bool
	}{
		{"terminal", true, false, "xterm", true},
		{"redirected", false, false, "xterm", false},
		{"no-color", true, true, "xterm", false},
		{"dumb-terminal", true, false, "dumb", false},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("TERM", test.term)
			t.Setenv("NO_COLOR", "")
			if !test.noColor {
				os.Unsetenv("NO_COLOR")
			}
			if got := helpColorEnabled(test.terminal); got != test.want {
				t.Fatalf("color enabled=%v, want %v", got, test.want)
			}
		})
	}
	t.Run("nonempty-no-color", func(t *testing.T) {
		t.Setenv("NO_COLOR", "1")
		t.Setenv("TERM", "xterm")
		if helpColorEnabled(true) {
			t.Fatal("NO_COLOR must disable help colors")
		}
	})
	plain := renderHelp(false)
	colored := renderHelp(true)
	if strings.Contains(plain, "\x1b[") || !strings.Contains(colored, "\x1b[36m--help\x1b[0m") {
		t.Fatal("incorrect help coloring")
	}
	if regexp.MustCompile(`\x1b\[[0-9;]*m`).ReplaceAllString(colored, "") != plain {
		t.Fatal("color formatting changed help content")
	}
}

func TestCLIHelpHint(t *testing.T) {
	for _, args := range [][]string{
		nil,
		{"--provider", "openai"},
		{"--provider", "openai", "--system-instruction", "Classify", "--schema", `{"type":"object","properties":{},"required":[],"additionalProperties":false}`},
		{"--mode", "decisions", "--provider", "systemone"},
		{"--unknown-option"},
		{"--model"},
	} {
		t.Run(strings.Join(args, " "), func(t *testing.T) {
			out, stderr, code := runDecisionCLI(t, "", args...)
			if code != 2 || out != "" || !strings.Contains(stderr, "Run again with --help: prompt2json --help") || strings.Contains(stderr, "\x1b[") {
				t.Fatalf("exit=%d stdout=%q stderr=%q", code, out, stderr)
			}
		})
	}
	out, stderr, code := runDecisionCLI(t, "", "--help")
	if code != 0 || out != "" || !strings.Contains(stderr, "Usage:") || strings.Contains(stderr, "\x1b[") {
		t.Fatalf("redirected help: exit=%d stdout=%q stderr=%q", code, out, stderr)
	}
}

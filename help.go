package main

import (
	"fmt"
	"os"
	"regexp"
	"strings"

	"golang.org/x/term"
)

func printHelpHint() {
	fmt.Fprintln(os.Stderr, "Run again with --help: prompt2json --help")
}

func helpColorEnabled(terminal bool) bool {
	_, noColor := os.LookupEnv("NO_COLOR")
	return terminal && !noColor && os.Getenv("TERM") != "dumb"
}

var helpOptionPattern = regexp.MustCompile(`--[a-z][a-z-]*`)

func renderHelp(color bool) string {
	if !color {
		return helpText
	}
	lines := strings.Split(helpText, "\n")
	for i, line := range lines {
		if i == 0 || (len(line) > 0 && line[0] != ' ' && strings.HasSuffix(line, ":")) {
			lines[i] = "\x1b[1;36m" + line + "\x1b[0m"
		} else {
			lines[i] = helpOptionPattern.ReplaceAllString(line, "\x1b[36m$0\x1b[0m")
		}
	}
	return strings.Join(lines, "\n")
}

func printHelp() {
	fmt.Fprint(os.Stderr, renderHelp(helpColorEnabled(term.IsTerminal(int(os.Stderr.Fd())))))
}

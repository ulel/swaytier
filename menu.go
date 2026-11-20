package main

import (
	"os"
	"os/exec"
	"strings"
)

const sep = "  ·  "

// menuCmd builds the launcher command. Set SWAYTIER_MENU to any
// dmenu-compatible command to replace wofi; it is run through sh and gets
// the prompt in $SWAYTIER_PROMPT.
func menuCmd(prompt string, extra ...string) *exec.Cmd {
	if custom := os.Getenv("SWAYTIER_MENU"); custom != "" {
		cmd := exec.Command("sh", "-c", custom)
		cmd.Env = append(os.Environ(), "SWAYTIER_PROMPT="+prompt)
		return cmd
	}
	args := []string{"--dmenu", "--insensitive", "--matching", "fuzzy", "--cache-file", "/dev/null", "--prompt", prompt}
	return exec.Command("wofi", append(args, extra...)...)
}

func ask(cmd *exec.Cmd, lines []string) (string, bool) {
	cmd.Stdin = strings.NewReader(strings.Join(lines, "\n"))
	cmd.Stderr = os.Stderr
	out, err := cmd.Output()
	if err != nil {
		return "", false
	}
	s := strings.TrimSpace(string(out))
	return s, s != ""
}

// pick shows a fuzzy menu and returns the chosen (or typed) line.
func pick(prompt string, lines []string) (string, bool) {
	return ask(menuCmd(prompt), lines)
}

// input asks for a line of free text.
func input(prompt string) (string, bool) {
	if os.Getenv("SWAYTIER_MENU") != "" {
		return ask(menuCmd(prompt), nil)
	}
	// wofi hides the prompt while the entry has focus, so repeat it as a
	// list line; --exec-search makes Enter return the typed text, never
	// that line.
	return ask(menuCmd(prompt, "--exec-search", "--lines", "3"), []string{prompt})
}

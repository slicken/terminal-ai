package main

import (
	"fmt"
	"os"
	"strings"
)

// promptStyle describes how to draw the interactive line and the thinking spinner.
type promptStyle struct {
	// Legacy: full TERMINAL_AI_PROMPT template with %w / %u / %h (old behavior).
	Legacy bool
	// AskMode: non-interactive; simple [Thinking] status line on stderr.
	AskMode bool

	FullPrompt string // legacy expanded prompt (full string)

	// Label mode (default): mimics user@host:path$ with label replacing user@host.
	LabelPlain string // e.g. TerminalAI — used for padding during wave
	LabelOpen  string // ANSI before label (e.g. green bold)
	LabelClose string // ANSI after label, before ":path$"
	AfterLabel string // ":path$ " with colors (colon, cwd, $)
}

func shortWorkingDir() string {
	wd, _ := os.Getwd()
	if wd == "" {
		wd = "."
	}
	home, _ := os.UserHomeDir()
	shortWd := wd
	if home != "" && (wd == home || strings.HasPrefix(wd, home+string(os.PathSeparator))) {
		shortWd = "~" + strings.TrimPrefix(wd, home)
	}
	return shortWd
}

// isLegacyTerminalAIPrompt returns true if the user wants the old full-template style (%w, %u, %h).
func isLegacyTerminalAIPrompt(raw string) bool {
	if strings.TrimSpace(raw) == "" {
		return false
	}
	return strings.Contains(raw, "%w") || strings.Contains(raw, "%u") || strings.Contains(raw, "%h")
}

// buildInteractivePrompt builds the displayed prompt line and metadata for the spinner.
// Default (no legacy placeholders): TerminalAI:~/path$  with colors like a typical user@host:path$ prompt.
func buildInteractivePrompt() (display string, style promptStyle) {
	raw := os.Getenv("TERMINAL_AI_PROMPT")
	if isLegacyTerminalAIPrompt(raw) {
		exp := expandPromptTemplate(raw)
		return exp, promptStyle{Legacy: true, FullPrompt: exp}
	}

	label := strings.TrimSpace(raw)
	if label == "" {
		label = "TerminalAI"
	}

	wd := shortWorkingDir()
	open, close, after := labelPromptANSI(wd)
	display = open + label + close + after
	return display, promptStyle{
		LabelPlain: label,
		LabelOpen:  open,
		LabelClose: close,
		AfterLabel: after,
	}
}

// labelPromptANSI returns open/close ANSI around the label and the tail ":path$ " with colors.
func labelPromptANSI(shortWd string) (open, close, after string) {
	// Green bold label, white :, blue path, white $ — similar to common distro prompts.
	open = "\033[1;32m"
	close = "\033[0m"
	after = fmt.Sprintf("\033[1;37m:\033[0m\033[1;34m%s\033[0m\033[1;37m$\033[0m ", shortWd)
	return
}

package main

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"os/exec"
	"runtime"
	"strings"

	"golang.org/x/term"
)

func main() {
	if len(os.Args) >= 2 {
		switch os.Args[1] {
		case "-h", "--help", "help":
			usage()
			os.Exit(0)
		case "ask":
			os.Exit(runAsk(os.Args[2:]))
		case "interactive", "i":
			os.Exit(runInteractive())
		default:
			fmt.Fprintf(os.Stderr, "terminal-ai: unknown command %q\n\n", os.Args[1])
			usage()
			os.Exit(2)
		}
	}
	os.Exit(runInteractive())
}

func usage() {
	fmt.Fprint(os.Stderr, `usage:
  terminal-ai                      interactive: prompt, one line of input, run, exit
  terminal-ai interactive          same as above
  terminal-ai ask [query...]       non-interactive; or read query from stdin if no args
  terminal-ai help

Bind your terminal hotkey to: terminal-ai
(or the full path to the binary).

Environment:
  OLLAMA_HOST          local server base URL (default http://127.0.0.1:11434)
  OLLAMA_MODEL         model name (default gemma4:e2b)
  OLLAMA_API_KEY       if set, uses https://ollama.com/api/generate
  TERMINAL_AI_PROMPT   interactive prompt; placeholders %w %u %h, %% for literal % (see README)
`)
}

// expandPromptTemplate replaces %w (cwd with ~), %u, %h (short host); %% is a literal %.
func expandPromptTemplate(tmpl string) string {
	const pct = "\x00"
	s := strings.ReplaceAll(tmpl, "%%", pct)
	wd, _ := os.Getwd()
	if wd == "" {
		wd = "."
	}
	home, _ := os.UserHomeDir()
	shortWd := wd
	if home != "" && (wd == home || strings.HasPrefix(wd, home+string(os.PathSeparator))) {
		shortWd = "~" + strings.TrimPrefix(wd, home)
	}
	s = strings.ReplaceAll(s, "%w", shortWd)
	s = strings.ReplaceAll(s, "%u", os.Getenv("USER"))
	s = strings.ReplaceAll(s, "%h", shortHostname())
	s = strings.ReplaceAll(s, pct, "%")
	return s
}

func shortHostname() string {
	h, err := os.Hostname()
	if err != nil || h == "" {
		return "host"
	}
	if i := strings.IndexByte(h, '.'); i >= 0 {
		return h[:i]
	}
	return h
}

// sttySaneOn restores cooked mode + echo; needed after Bash readline (e.g. bind -x).
func sttySaneOn(f *os.File) {
	if f == nil || runtime.GOOS == "windows" {
		return
	}
	c := exec.Command("stty", "sane")
	c.Stdin = f
	c.Stdout = f
	c.Stderr = f
	_ = c.Run()
}

func runInteractive() int {
	raw := os.Getenv("TERMINAL_AI_PROMPT")
	var prompt string
	if strings.TrimSpace(raw) == "" {
		prompt = "[terminal-ai]$ "
	} else {
		prompt = expandPromptTemplate(raw)
	}

	tty, err := openDevTTY()
	var in io.Reader
	var promptOut io.Writer
	if err == nil && tty != nil {
		defer tty.Close()
		sttySaneOn(tty)
		in, promptOut = tty, tty
	} else {
		if !term.IsTerminal(int(os.Stdin.Fd())) {
			fmt.Fprintf(os.Stderr, "terminal-ai: stdin is not a terminal; use: terminal-ai ask ...\n")
			return 2
		}
		sttySaneOn(os.Stdin)
		in, promptOut = os.Stdin, os.Stdout
	}

	fmt.Fprint(promptOut, prompt)
	line, err := bufio.NewReader(in).ReadString('\n')
	if err != nil {
		if err == io.EOF {
			return 0
		}
		fmt.Fprintf(os.Stderr, "terminal-ai: read input: %v\n", err)
		return 1
	}
	line = strings.TrimSpace(line)
	if line == "" {
		return 0
	}
	return runQuery(line)
}

func runAsk(argv []string) int {
	query := strings.TrimSpace(strings.Join(argv, " "))
	if query == "" {
		b, err := io.ReadAll(os.Stdin)
		if err != nil {
			fmt.Fprintf(os.Stderr, "terminal-ai: read stdin: %v\n", err)
			return 1
		}
		query = strings.TrimSpace(string(b))
	}
	return runQuery(query)
}

func runQuery(query string) int {
	query = strings.TrimSpace(query)
	if query == "" {
		fmt.Fprintln(os.Stderr, "terminal-ai: empty query")
		return 1
	}

	cmdStr, err := shellFromNaturalLanguage(query)
	if err != nil {
		fmt.Fprintf(os.Stderr, "terminal-ai: %v\n", err)
		return 1
	}
	cmdStr = sanitizeCommand(cmdStr)
	if cmdStr == "" {
		fmt.Fprintln(os.Stderr, "terminal-ai: model returned an empty command")
		return 1
	}

	out, err := exec.Command("sh", "-c", cmdStr).CombinedOutput()
	out = []byte(strings.TrimRight(string(out), "\n"))
	if err != nil {
		if len(out) > 0 {
			fmt.Fprintf(os.Stderr, "%s\n", out)
		}
		fmt.Fprintf(os.Stderr, "terminal-ai: %v\n", err)
		fmt.Fprintf(os.Stderr, "command was: %s\n", cmdStr)
		return 1
	}
	if len(out) > 0 {
		fmt.Printf("%s\n", out)
	}
	return 0
}

func sanitizeCommand(s string) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "```")
	s = strings.TrimSuffix(s, "```")
	s = strings.TrimSpace(s)
	lines := strings.Split(s, "\n")
	s = strings.TrimSpace(lines[0])
	for _, pref := range []string{"bash", "sh", "zsh", "fish"} {
		if strings.HasPrefix(strings.ToLower(s), pref+" ") {
			s = strings.TrimSpace(s[len(pref):])
			break
		}
	}
	return strings.TrimSpace(s)
}

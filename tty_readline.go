package main

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"strings"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// loneESCHotkeyPoll is how long we wait for the rest of TERMINAL_AI_HOTKEY after a leading ESC
// before treating the key as a lone Escape (cancel), so the input loop does not block forever.
const loneESCHotkeyPoll = 100 * time.Millisecond

// terminalAIHotkey returns the raw byte sequence for TERMINAL_AI_HOTKEY (toggle / cancel).
// Default is "\ep" (ESC then p, i.e. typical Alt+P). Empty env uses that default.
func terminalAIHotkey() []byte {
	s := strings.TrimSpace(os.Getenv("TERMINAL_AI_HOTKEY"))
	if s == "" {
		s = `\ep`
	}
	b, err := parseHotkeyString(s)
	if err != nil || len(b) == 0 {
		return []byte{0x1b, 'p'}
	}
	return b
}

// parseHotkeyString interprets backslash escapes like \e (ESC), \n, \x1b, etc.
func parseHotkeyString(s string) ([]byte, error) {
	var out []byte
	for i := 0; i < len(s); i++ {
		if s[i] != '\\' || i+1 >= len(s) {
			out = append(out, s[i])
			continue
		}
		i++
		switch s[i] {
		case 'e', 'E':
			out = append(out, 0x1b)
		case 'a':
			out = append(out, 0x07)
		case 'b':
			out = append(out, 0x08)
		case 'f':
			out = append(out, 0x0c)
		case 'n':
			out = append(out, 0x0a)
		case 'r':
			out = append(out, 0x0d)
		case 't':
			out = append(out, 0x09)
		case 'v':
			out = append(out, 0x0b)
		case '\\':
			out = append(out, '\\')
		case 'x':
			if i+2 >= len(s) {
				return nil, fmt.Errorf("incomplete \\x in hotkey")
			}
			hi := hexDigit(s[i+1])
			lo := hexDigit(s[i+2])
			if hi < 0 || lo < 0 {
				return nil, fmt.Errorf("invalid hex in hotkey")
			}
			out = append(out, byte(hi<<4|lo))
			i += 2
		default:
			out = append(out, s[i])
		}
	}
	return out, nil
}

// terminalAIRestoreLine is the shell prompt line to repaint on cancel (hotkey) so we stay on one row.
// Order: TERMINAL_AI_SHELL_LINE if set; else expanded ${PS1@P} via bash (unless TERMINAL_AI_EXPAND_PS1=0).
func terminalAIRestoreLine() string {
	if s, ok := os.LookupEnv("TERMINAL_AI_SHELL_LINE"); ok {
		return s
	}
	if os.Getenv("TERMINAL_AI_EXPAND_PS1") == "0" {
		return ""
	}
	return expandPS1ViaBash()
}

func expandPS1ViaBash() string {
	ctx, cancel := context.WithTimeout(context.Background(), 120*time.Millisecond)
	defer cancel()
	cmd := exec.CommandContext(ctx, "bash", "-c", `printf '%s' "${PS1@P}"`)
	cmd.Env = os.Environ()
	out, err := cmd.Output()
	if err != nil {
		return ""
	}
	return string(out)
}

// writeShellRestore clears the current row and redraws the remembered shell prompt (no newline).
// Call only after the tty is back in cooked mode (term.Restore + stty sane) so readline/bind see a sane line discipline.
func writeShellRestore(tty *os.File, restoreLine string) {
	_, _ = io.WriteString(tty, "\r\033[2K")
	if restoreLine != "" {
		_, _ = io.WriteString(tty, restoreLine)
	}
	_, _ = io.WriteString(tty, "\033[0m") // reset SGR; avoids stuck attributes confusing the next keypress
	_ = tty.Sync()
}

// repairTTYLineDiscipline runs stty sane on the session tty and, when it differs from stdin, on stdin too.
// Bash/readline can keep separate fds; after raw mode both must be sane or the next Meta key echoes as ^[p.
func repairTTYLineDiscipline(tty *os.File) {
	if tty == nil {
		return
	}
	sttySaneOn(tty)
	stdinFd := int(os.Stdin.Fd())
	ttyFd := int(tty.Fd())
	if stdinFd != ttyFd && term.IsTerminal(stdinFd) {
		sttySaneOn(os.Stdin)
	}
}

// announceTTYReadyForReadline resets common private modes so readline/bind handle the next keypress.
func announceTTYReadyForReadline(w io.Writer) {
	f, ok := w.(*os.File)
	if !ok || f == nil || !term.IsTerminal(int(f.Fd())) {
		return
	}
	_, _ = io.WriteString(f, "\033[?25h\033[?2004l") // show cursor; disable bracketed paste if stuck on
	_ = f.Sync()
}

// syncReadlineWithNewline sends a newline to the controlling tty so Bash readline accepts a line after
// bind -x (same effect as pressing Enter once).
// If needExtra is false, skip writing: e.g. successful runQuery already printed a trailing newline;
// hotkey cancel already used writeShellRestore on the same row; empty input already sent \r\n.
func syncReadlineWithNewline(needExtra bool) {
	if !needExtra {
		return
	}
	f, err := os.OpenFile("/dev/tty", os.O_WRONLY, 0)
	if err != nil {
		if term.IsTerminal(int(os.Stdout.Fd())) {
			_, _ = fmt.Fprint(os.Stdout, "\n")
		}
		return
	}
	defer f.Close()
	_, _ = io.WriteString(f, "\n")
	_ = f.Sync()
}

func hexDigit(b byte) int {
	switch {
	case b >= '0' && b <= '9':
		return int(b - '0')
	case b >= 'a' && b <= 'f':
		return int(b-'a') + 10
	case b >= 'A' && b <= 'F':
		return int(b-'A') + 10
	default:
		return -1
	}
}

// readInteractiveLine reads one line from the tty in raw mode so the kernel does not echo
// escape sequences as garbage. TERMINAL_AI_HOTKEY (default \ep) returns cancel=true to return to the shell.
func readInteractiveLine(tty *os.File, prompt string) (line string, cancel bool, err error) {
	restoreLine := terminalAIRestoreLine()
	hotkey := terminalAIHotkey()
	fd := int(tty.Fd())
	old, err := term.MakeRaw(fd)
	if err != nil {
		return "", false, err
	}
	var ttyRestored bool
	defer func() {
		if ttyRestored {
			return
		}
		if old != nil {
			_ = term.Restore(fd, old)
		}
		repairTTYLineDiscipline(tty)
		announceTTYReadyForReadline(tty)
	}()

	if _, err := io.WriteString(tty, prompt); err != nil {
		return "", false, err
	}

	var lineRunes []rune
	var pending []byte
	readBuf := make([]byte, 256)

	cancelToShell := func() (string, bool, error) {
		if old != nil {
			_ = term.Restore(fd, old)
		}
		repairTTYLineDiscipline(tty)
		writeShellRestore(tty, restoreLine)
		announceTTYReadyForReadline(tty)
		ttyRestored = true
		return "", true, nil
	}

	for {
		if len(pending) == 0 {
			n, rerr := tty.Read(readBuf)
			if n > 0 {
				pending = append(pending, readBuf[:n]...)
			}
			if rerr != nil && rerr != io.EOF {
				return "", false, rerr
			}
			if rerr == io.EOF && len(pending) == 0 {
				return string(lineRunes), false, io.EOF
			}
		}

		consumed, done, hotkeyCancel, cerr := processTTYInput(tty, &lineRunes, pending, hotkey)
		pending = pending[consumed:]
		if cerr != nil {
			return "", false, cerr
		}
		if hotkeyCancel {
			return cancelToShell()
		}
		if done {
			_, _ = io.WriteString(tty, "\r\n")
			return string(lineRunes), false, nil
		}

		// Buffer is only a strict prefix of the hotkey (e.g. ESC waiting for p). Without a follow-up
		// byte the loop would block forever; wait briefly, then treat as lone Escape = cancel.
		if len(pending) > 0 && len(hotkey) > 1 && pending[0] == 0x1b &&
			bytes.HasPrefix(hotkey, pending) && len(pending) < len(hotkey) {
			n2, rerr2 := ttyReadAfterPoll(tty, readBuf, loneESCHotkeyPoll)
			if n2 > 0 {
				pending = append(pending, readBuf[:n2]...)
				continue
			}
			if errors.Is(rerr2, io.EOF) {
				return string(lineRunes), false, io.EOF
			}
			if errors.Is(rerr2, os.ErrDeadlineExceeded) || rerr2 == nil {
				return cancelToShell()
			}
			return "", false, rerr2
		}

		if len(pending) == 0 {
			continue
		}
		if consumed == 0 {
			n, rerr := tty.Read(readBuf)
			if n > 0 {
				pending = append(pending, readBuf[:n]...)
				continue
			}
			if rerr != nil && rerr != io.EOF {
				return "", false, rerr
			}
			if rerr == io.EOF && len(pending) == 0 {
				return string(lineRunes), false, io.EOF
			}
			continue
		}
	}
}

func processTTYInput(tty *os.File, line *[]rune, pending []byte, hotkey []byte) (consumed int, done bool, hotkeyCancel bool, err error) {
	i := 0
	for i < len(pending) {
		rest := pending[i:]
		if len(hotkey) > 0 {
			if len(rest) >= len(hotkey) && bytes.Equal(rest[:len(hotkey)], hotkey) {
				return i + len(hotkey), true, true, nil
			}
			if len(rest) < len(hotkey) && bytes.HasPrefix(hotkey, rest) {
				break
			}
		}

		b := pending[i]

		if b == 0x1b {
			n := consumeEscSequence(pending[i:])
			if n == 0 {
				break
			}
			i += n
			continue
		}

		switch b {
		case '\r', '\n':
			i++
			if b == '\r' && i < len(pending) && pending[i] == '\n' {
				i++
			}
			return i, true, false, nil
		case 3: // ^C
			_, _ = io.WriteString(tty, "\r\n")
			return i + 1, false, false, io.EOF
		case 4: // ^D
			if len(*line) == 0 {
				_, _ = io.WriteString(tty, "\r\n")
				return i + 1, false, false, io.EOF
			}
			i++
			continue
		case 127, 8: // backspace / ^H
			if len(*line) > 0 {
				*line = (*line)[:len(*line)-1]
				_, _ = io.WriteString(tty, "\b \b")
			}
			i++
			continue
		}

		if !utf8.FullRune(pending[i:]) {
			break
		}
		r, sz := utf8.DecodeRune(pending[i:])
		if sz == 1 && r == utf8.RuneError {
			i++
			continue
		}
		if r < 32 || (r >= 0x7f && r < 0xa0) {
			i += sz
			continue
		}
		*line = append(*line, r)
		var buf [utf8.UTFMax]byte
		nw := utf8.EncodeRune(buf[:], r)
		_, _ = tty.Write(buf[:nw])
		i += sz
	}
	return i, false, false, nil
}

// consumeEscSequence returns how many bytes belong to one escape sequence starting with ESC
// (arrows, function keys, etc.), not matched as TERMINAL_AI_HOTKEY.
func consumeEscSequence(b []byte) (n int) {
	if len(b) == 0 || b[0] != 0x1b {
		return 0
	}
	for i := 1; i < len(b); i++ {
		c := b[i]
		if (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || c == '~' {
			return i + 1
		}
	}
	return 0
}

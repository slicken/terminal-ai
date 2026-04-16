package main

import (
	"fmt"
	"os"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"golang.org/x/term"
)

// GetSpinner defines the animation. Every frame is padded to the same width: the length of the
// longest string in this slice (e.g. len("thinkIng....")), so redraws never leave trailing junk.
func GetSpinner() []string {
	return []string{
		"⠋ Thinking",
		"⠙ Thinking",
		"⠹ Thinking",
		"⠸ Thinking",
		"⠼ Thinking",
		"⠴ Thinking",
		"⠦ Thinking",
		"⠧ Thinking",
		"⠇ Thinking",
		"⠏ Thinking",
	}
}

// maxSpinnerWidth is len(longest frame); all frames are padded to this width for stable overwrites.
func maxSpinnerWidth(frames []string) int {
	m := 0
	for _, f := range frames {
		if n := len(f); n > m {
			m = n
		}
	}
	return m
}

func stderrIsTTY() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}

// legacySpinnerPrefixVisibleRunes is how many visible runes at the start of the legacy expanded
// prompt count as the prefix before the [thinking] segment (ANSI escapes are not counted).
const legacySpinnerPrefixVisibleRunes = 12

func skipEscapeSeq(b []byte, start int) int {
	if start >= len(b) || b[start] != 0x1b {
		return start
	}
	j := start + 1
	if j >= len(b) {
		return len(b)
	}
	switch b[j] {
	case '[':
		j++
		for j < len(b) && (b[j] < 0x40 || b[j] > 0x7e) {
			j++
		}
		if j < len(b) {
			j++
		}
		return j
	case ']':
		j++
		for j < len(b) {
			if b[j] == 0x07 {
				return j + 1
			}
			if b[j] == 0x1b && j+1 < len(b) && b[j+1] == '\\' {
				return j + 2
			}
			j++
		}
		return len(b)
	default:
		return j + 1
	}
}

func promptSuffixAfterVisualPrefix(expandedPrompt string, n int) string {
	if n <= 0 {
		return expandedPrompt
	}
	b := []byte(expandedPrompt)
	i := 0
	vis := 0
	for i < len(b) && vis < n {
		if b[i] == 0x1b {
			next := skipEscapeSeq(b, i)
			if next <= i {
				i++
				continue
			}
			i = next
			continue
		}
		r, sz := utf8.DecodeRune(b[i:])
		if sz == 0 {
			i++
			continue
		}
		if r == utf8.RuneError && sz == 1 {
			i++
			continue
		}
		vis++
		i += sz
	}
	return expandedPrompt[i:]
}

// startThinkingSpinner draws the thinking animation. Label mode animates in place of the app name
// (TerminalAI:path$); legacy mode uses [frames] + suffix; ask mode uses a short [Thinking] line.
func startThinkingSpinner(style promptStyle, userDisplay string) (stop func()) {
	if !stderrIsTTY() {
		return func() {}
	}
	if style.AskMode {
		return startAskLineSpinner()
	}
	if style.Legacy {
		return startLegacyTemplateSpinner(style.FullPrompt, userDisplay)
	}
	return startLabelSpinner(style, userDisplay)
}

func startLabelSpinner(style promptStyle, userDisplay string) (stop func()) {
	inline := userDisplay != ""
	frames := GetSpinner()
	width := maxSpinnerWidth(frames)
	pad := func(s string) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(done)
		ticker := time.NewTicker(95 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		printFrame := func() {
			frame := pad(frames[i%len(frames)])
			i++
			if !inline {
				return
			}
			if stderrIsTTY() && (style.LabelOpen != "" || style.LabelClose != "") {
				fmt.Fprintf(os.Stderr, "\r%s%s%s%s%s\033[K",
					style.LabelOpen, frame, style.LabelClose, style.AfterLabel, userDisplay)
			} else {
				fmt.Fprintf(os.Stderr, "\r%s%s%s\033[K", frame, style.AfterLabel, userDisplay)
			}
		}

		if inline {
			fmt.Fprint(os.Stderr, "\033[1A\r")
		}
		printFrame()
		for {
			select {
			case <-stopCh:
				// Clear line only — no extra \n here or it inserts a blank row before stdout.
				fmt.Fprint(os.Stderr, "\r\033[K")
				_ = os.Stderr.Sync()
				return
			case <-ticker.C:
				printFrame()
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stopCh)
			<-done
		})
	}
}

func startLegacyTemplateSpinner(expandedPrompt, userDisplay string) (stop func()) {
	inline := userDisplay != "" && strings.TrimSpace(expandedPrompt) != ""
	n := legacySpinnerPrefixVisibleRunes
	promptSuffix := promptSuffixAfterVisualPrefix(expandedPrompt, n)

	frames := GetSpinner()
	width := maxSpinnerWidth(frames)
	pad := func(s string) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(done)
		ticker := time.NewTicker(95 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		printFrame := func() {
			inner := pad(frames[i%len(frames)])
			i++
			if inline {
				if stderrIsTTY() {
					fmt.Fprintf(os.Stderr, "\r\033[2;37m[\033[0m\033[1;38;5;214m%s\033[0m\033[2;37m]\033[0m%s%s\033[K",
						inner, promptSuffix, userDisplay)
				} else {
					fmt.Fprintf(os.Stderr, "\r[%s]%s%s\033[K", inner, promptSuffix, userDisplay)
				}
				return
			}
			if stderrIsTTY() {
				fmt.Fprintf(os.Stderr, "\r\033[2;37m[\033[0m\033[1;38;5;214m%s\033[0m\033[2;37m]\033[0m\033[K", inner)
			} else {
				fmt.Fprintf(os.Stderr, "\r[%s]\033[K", inner)
			}
		}

		if inline {
			fmt.Fprint(os.Stderr, "\033[1A\r")
		}
		printFrame()
		for {
			select {
			case <-stopCh:
				fmt.Fprint(os.Stderr, "\r\033[K")
				_ = os.Stderr.Sync()
				return
			case <-ticker.C:
				printFrame()
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stopCh)
			<-done
		})
	}
}

func startAskLineSpinner() (stop func()) {
	frames := GetSpinner()
	width := maxSpinnerWidth(frames)
	pad := func(s string) string {
		if len(s) >= width {
			return s
		}
		return s + strings.Repeat(" ", width-len(s))
	}

	stopCh := make(chan struct{})
	done := make(chan struct{})
	var once sync.Once

	go func() {
		defer close(done)
		ticker := time.NewTicker(95 * time.Millisecond)
		defer ticker.Stop()

		i := 0
		printFrame := func() {
			inner := pad(frames[i%len(frames)])
			i++
			if stderrIsTTY() {
				fmt.Fprintf(os.Stderr, "\r\033[2;37m[\033[0m\033[1;38;5;214m%s\033[0m\033[2;37m]\033[0m\033[K", inner)
			} else {
				fmt.Fprintf(os.Stderr, "\r[%s]\033[K", inner)
			}
		}

		printFrame()
		for {
			select {
			case <-stopCh:
				fmt.Fprint(os.Stderr, "\r\033[K")
				_ = os.Stderr.Sync()
				return
			case <-ticker.C:
				printFrame()
			}
		}
	}()

	return func() {
		once.Do(func() {
			close(stopCh)
			<-done
		})
	}
}

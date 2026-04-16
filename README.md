# terminal-ai

**terminal-ai** is a small **lightweight** helper for your terminal: it prints a **prompt**, you type what you want in plain language, it asks an **Ollama** model for **one** shell command, runs it with `sh -c`, prints combined stdout/stderr, then exits so your normal shell prompt returns.

**Interactive mode** (the default) reads **one** line, runs that workflow, and quits. Map a **hotkey** (e.g. **Alt+P** via your terminal or Bash `bind -x`) to the **binary**, or run `terminal-ai` when you need it.

## Requirements

- **Go** (to build)
- **Ollama**: local HTTP API and model, or [Ollama Cloud](https://ollama.com) with an API key

## Build and install

```bash
cd /path/to/terminal-ai
go build -o terminal-ai .
```

Put the binary on your `PATH`, or point your hotkey at the **file** (not the project directory), e.g. `/path/to/terminal-ai/terminal-ai`.

## CLI

```bash
terminal-ai                          # interactive: one line, then exit
terminal-ai interactive              # same
terminal-ai ask list files here    # non-interactive; query from arguments
terminal-ai ask                      # non-interactive; query from stdin if no args
terminal-ai help
terminal-ai -h                       # same as help
terminal-ai --help
```

## Prompt modes (`TERMINAL_AI_PROMPT`)

### Default (label mode)

If **`TERMINAL_AI_PROMPT`** does **not** contain **`%w`**, **`%u`**, or **`%h`**, the env var is **only the short label** — the text that stands where **`user@host`** would be in a normal prompt (e.g. **`TerminalAI`** or **`MyAI`**).

The app **always** builds the rest of the line itself: **`:path$ `** — current directory with **`~`** under **`$HOME`**, the **`$`**, spacing, and ANSI styling. You **do not** set **`:~/…$`** (or the colon, path, or dollar) via this variable in label mode.

| Part | Source |
|------|--------|
| **Label** (`user@host` stand-in) | **`TERMINAL_AI_PROMPT`**; if unset or empty, **`TerminalAI`** |
| **`:path$ `** (colon, cwd, dollar, colors) | **Built by the app** — not from the env value |

Example:

```bash
export TERMINAL_AI_PROMPT=MyAI
# Renders like: MyAI:~/project$
```

While the model runs, a **spinner** replaces **only the label** on that row; **`:path$ `** and what you typed stay visible.

### Legacy (full template)

If the value **contains** **`%w`**, **`%u`**, or **`%h`**, **legacy template mode** is used: **your string is the entire prompt line** (after expansion). You place **`%w`**, **`%u`**, **`%h`** wherever you want in that line — including where the directory appears — and you can add your own ANSI, spacing, and prompt character. This is **not** the same as label mode: the app does **not** append a separate **`:path$ `** block; the expanded template **is** what gets shown.

| Placeholder | Replaced with |
|-------------|----------------|
| **`%w`** | Current directory, `~` under `$HOME` |
| **`%u`** | **`$USER`** |
| **`%h`** | Short hostname (segment before first `.`) |
| **`%%`** | A literal **`%`** |

In this mode the thinking line shows a **`[ … ]`**-style segment after the **first 12 visible characters** of the expanded prompt (ANSI escapes are not counted toward that 12).

Example (legacy — **full line** is your template):

```bash
export TERMINAL_AI_PROMPT='%u@%h:%w$ '
terminal-ai
```

## Environment variables

| Variable | Purpose | Default |
|----------|---------|---------|
| **`OLLAMA_HOST`** | Base URL of the **local** Ollama HTTP API (no trailing slash). Not used when **`OLLAMA_API_KEY`** is set. | `http://127.0.0.1:11434` |
| **`OLLAMA_MODEL`** | Model name for `/api/generate`. | `gemma4:e2b` |
| **`OLLAMA_API_KEY`** | If set (non-empty), requests go to **Ollama Cloud** at `https://ollama.com/api/generate` with `Authorization: Bearer …`. | *(unset → local server)* |
| **`TERMINAL_AI_PROMPT`** | **Label mode:** label only (like `user@host`); app adds **`:path$ `**. **Legacy** (if value contains **`%w`** / **`%u`** / **`%h`**): **full-line** template with placeholders. | Label **`TerminalAI`** when unset |
| **`TERMINAL_AI_HOTKEY`** | Raw byte sequence that **cancels** interactive input (return to the shell). Go-style escapes: **`\e`** = ESC, **`\xNN`**, etc. | **`\\ep`** (ESC then **`p`**, typical **Alt+P** on xterm-like terminals) |
| **`TERMINAL_AI_SHELL_LINE`** | If set, this exact string is redrawn on **the same row** when you cancel (optional). | *(unset)* |
| **`TERMINAL_AI_EXPAND_PS1`** | If unset or **`1`**, cancel redraw uses **`bash -c 'printf %s "${PS1@P}"'`** when **`TERMINAL_AI_SHELL_LINE`** is unset. Set **`0`** to skip. | on |

## Hotkey, Escape, and Bash `bind -x`

**Terminal / WM shortcut:** bind your key to the **terminal-ai binary** (full path is safest).

**Cancel from inside the app:**

- The configured hotkey (default **Alt+P**, i.e. ESC then **`p`**) returns to the shell and, when possible, redraws your shell prompt on the **same line** as the app prompt.
- **Escape** alone also cancels: the program waits briefly for a possible second byte (so **Alt+P** still works when ESC and **`p`** arrive separately), then exits like a full hotkey cancel.

**Bash `bind -x`:** after the handler runs, readline sometimes needs the same effect as **Enter** on a new line. This program writes a newline to **`/dev/tty`** when exiting interactive mode **only when** that extra line is still needed (it is skipped when you cancel with the hotkey, submit an empty line, or finish a successful run that already printed a trailing newline). You can also reset the edit line in the binding:

```bash
bind -x '"\ep":"/path/to/terminal-ai/terminal-ai; READLINE_LINE=; READLINE_POINT=0"'
```

Wrapper example (set **`TERMINAL_AI_BIN`** to your binary path):

```bash
terminal_ai_run() {
  "$TERMINAL_AI_BIN"
  READLINE_LINE=
  READLINE_POINT=0
}
bind -x '"\ep":"terminal_ai_run"'
```

Use the **same** key sequence in **`TERMINAL_AI_HOTKEY`** as in **`bind -x`** so cancel in raw input matches your shell binding.

On cancel, the app restores **cooked** TTY mode, runs **`stty sane`** on **`/dev/tty`** and on **stdin** when they differ, then redraws: **`TERMINAL_AI_SHELL_LINE`** if set, otherwise expanded **`${PS1@P}`** via bash when **`TERMINAL_AI_EXPAND_PS1`** is not **`0`**.

Interactive mode opens **`/dev/tty`** when possible (Unix) and runs **`stty sane`** on startup so input works after **`bind -x`**.

## Security

The model may suggest **any** shell command; the tool runs it under **`sh -c`**. Use trusted models and hosts, and avoid pasting secrets into prompts.

## Example

![terminal-ai interactive prompt and output](terminal-ai.png)

Put **`terminal-ai.png`** in the **repository root** next to this README so the image resolves on GitHub and in editors.

## License

[MIT](LICENSE) — Copyright (c) 2026 slicken

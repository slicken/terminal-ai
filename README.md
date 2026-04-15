# terminal-ai

**terminal-ai** is a small, **lightweight** helper for your terminal: you get a short **prompt**, describe what you want in plain language, and it turns that into a **Linux shell command**, runs it with `sh -c`, and shows the output. Use it when you forget flags, pipeline order, or the right tool—without leaving the shell or opening a full-screen app.

It talks to a local or cloud **Ollama** model, suggests **one** command at a time, then exits so your normal shell prompt returns.

**Interactive mode** (the default) prints your configured prompt, reads **one** line, runs the workflow above, and quits. Map a **hotkey** (e.g. **Alt+p** via your terminal or Bash `bind -x`) to `terminal-ai`, or run `./terminal-ai` whenever you need it.

## Requirements

- **Go** (to build)
- **Ollama**: local server and model, *or* [Ollama Cloud](https://ollama.com) with an API key

## Build and install

```bash
cd /path/to/terminal-ai
go build -o terminal-ai .
```

Put the binary on your `PATH`, or configure your terminal shortcut with the full path.

## CLI

```bash
terminal-ai                          # interactive: one line, then exit
terminal-ai interactive              # same
terminal-ai ask list files here      # non-interactive (no prompt)
terminal-ai ask < file.txt           # query from stdin (whole file / stream)
terminal-ai help
```

## Environment variables

| Variable | Purpose | Default |
|----------|---------|---------|
| **`OLLAMA_HOST`** | Base URL of your **local** Ollama HTTP API (no trailing slash). Ignored when `OLLAMA_API_KEY` is set. | `http://127.0.0.1:11434` |
| **`OLLAMA_MODEL`** | Model name passed to `/api/generate`. | `gemma4:e2b` |
| **`OLLAMA_API_KEY`** | If set (non-empty), requests go to **`https://ollama.com/api/generate`** (Ollama Cloud) with `Authorization: Bearer …`. | *(unset → local server)* |
| **`TERMINAL_AI_PROMPT`** | String printed before reading a line in **interactive** mode. You can use **ANSI colors** (e.g. bash `$'...'`). Placeholders: **`%w`** current directory with `~` for home, **`%u`** user, **`%h`** short hostname; **`%%`** a literal `%`. | `[terminal-ai]$ ` |

**Colored prompt** example (user, host, path—similar idea to a Debian-style `PS1`):

```bash
export OLLAMA_MODEL=llama3.2
export TERMINAL_AI_PROMPT=$'\033[1;33m%u\033[0m@\033[1;32m%h\033[0m \033[1;34m%w\033[0m [ai]\$ '
terminal-ai
```

Put that in `~/.bashrc` (or wherever you set env) so it applies to **Alt+p** and normal `./terminal-ai` runs.

## Hotkey

**Terminal shortcut:** map **Alt+p** (or any key) to `terminal-ai` or `/path/to/terminal-ai`.

**Bash `bind -x`:** you can run the same binary from `~/.bashrc`:

```bash
export TERMINAL_AI_BIN=/path/to/terminal-ai
terminal_ai_run() {
  "$TERMINAL_AI_BIN"
  READLINE_LINE=
  READLINE_POINT=0
}
bind -x '"\ep":"terminal_ai_run"'
```

After **Alt+p**, readline often leaves the TTY in **raw** mode, so keystrokes would not echo. Interactive mode opens **`/dev/tty`** and runs **`stty sane`** on it first (Unix) so typing works the same as when you run `./terminal-ai` by hand.

## Security

The model may suggest **any** shell command; the tool runs it under **`sh -c`**. Only use models and hosts you trust, and avoid pasting sensitive data into prompts.

## Example

![terminal-ai interactive prompt and output](terminal-ai.png)

Place **`terminal-ai.png`** in the **repository root** next to this README (same folder as `go.mod`) so the image loads on GitHub and in editors that render relative paths.

## License

[MIT](LICENSE) — Copyright (c) 2026 slicken

package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"
)

// OllamaGenerateResponse matches /api/generate non-streaming JSON (see cloud_ollama.go).
type OllamaGenerateResponse struct {
	Response string `json:"response"`
}

func ollamaBaseURL() string {
	if h := strings.TrimSpace(os.Getenv("OLLAMA_HOST")); h != "" {
		return strings.TrimRight(h, "/")
	}
	return "http://127.0.0.1:11434"
}

func ollamaModel() string {
	if m := strings.TrimSpace(os.Getenv("OLLAMA_MODEL")); m != "" {
		return m
	}
	return "gemma4:e2b"
}

// Generate calls Ollama Cloud when OLLAMA_API_KEY is set (same pattern as cloud_ollama.go),
// otherwise the local server at OLLAMA_HOST (default http://127.0.0.1:11434).
func Generate(prompt string) (string, error) {
	apiKey := strings.TrimSpace(os.Getenv("OLLAMA_API_KEY"))
	payload := map[string]interface{}{
		"model":  ollamaModel(),
		"prompt": prompt,
		"stream": false,
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", err
	}

	var req *http.Request
	if apiKey != "" {
		req, err = http.NewRequest(http.MethodPost, "https://ollama.com/api/generate", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+apiKey)
	} else {
		req, err = http.NewRequest(http.MethodPost, ollamaBaseURL()+"/api/generate", bytes.NewReader(body))
		if err != nil {
			return "", err
		}
		req.Header.Set("Content-Type", "application/json")
	}

	client := &http.Client{Timeout: 120 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("ollama HTTP %d: %s", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var out OllamaGenerateResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("decode response: %w (body: %s)", err, truncate(string(raw), 200))
	}
	return strings.TrimSpace(out.Response), nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func shellFromNaturalLanguage(userLine string) (string, error) {
	prompt := `You translate a short user request into a single shell command for Linux.
Output exactly one line: the command only. No markdown, no backticks, no explanation.
Prefer common safe tools (ls, find, grep, git, cat, head, pwd, etc.).

User request: ` + strings.TrimSpace(userLine)
	return Generate(prompt)
}

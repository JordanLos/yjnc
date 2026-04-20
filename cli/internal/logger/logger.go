package logger

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

type Logger struct {
	f *os.File
}

type entry struct {
	Type      string `json:"type"`
	Timestamp string `json:"timestamp"`
	Unit      string `json:"unit"`
	Message   string `json:"message,omitempty"`
	ExitCode  *int   `json:"exit_code,omitempty"`
	Check     string `json:"check,omitempty"`
	Stdout    string `json:"stdout,omitempty"`
	Stderr    string `json:"stderr,omitempty"`
}

func New(root, unit string, run int) (*Logger, error) {
	dir := filepath.Join(root, ".juc", "logs", unit)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}
	f, err := os.Create(filepath.Join(dir, fmt.Sprintf("run-%d.jsonl", run)))
	if err != nil {
		return nil, err
	}
	return &Logger{f: f}, nil
}

func (l *Logger) Close() { l.f.Close() }

func (l *Logger) write(e entry) {
	e.Timestamp = time.Now().UTC().Format(time.RFC3339)
	data, _ := json.Marshal(e)
	fmt.Fprintln(l.f, string(data))
}

func (l *Logger) AgentStart(unit string) {
	l.write(entry{Type: "agent_start", Unit: unit})
}

func (l *Logger) AgentEnd(unit string) {
	l.write(entry{Type: "agent_end", Unit: unit})
}

func (l *Logger) CheckResult(unit, check string, exitCode int, stdout, stderr string) {
	l.write(entry{
		Type:     "check_result",
		Unit:     unit,
		Check:    check,
		ExitCode: &exitCode,
		Stdout:   stdout,
		Stderr:   stderr,
	})
}

func (l *Logger) Event(unit, msg string) {
	l.write(entry{Type: "event", Unit: unit, Message: msg})
}

package job

import (
	"fmt"
	"io"
	"os"
	"time"
)

// Logger provides prefixed logging for job execution.
type Logger struct {
	out io.Writer
}

// NewLogger creates a new logger that writes to stdout.
func NewLogger() *Logger {
	return &Logger{out: os.Stdout}
}

// Log writes a message with a source prefix.
func (l *Logger) Log(source, message string) {
	timestamp := time.Now().Format("2006-01-02T15:04:05Z")
	fmt.Fprintf(l.out, "[%s] [%-8s] %s\n", timestamp, source, message)
}

// Manfred logs a MANFRED message.
func (l *Logger) Manfred(message string) {
	l.Log("MANFRED", message)
}

// Docker logs a Docker message.
func (l *Logger) Docker(message string) {
	l.Log("DOCKER", message)
}

// Claude logs a Claude message.
func (l *Logger) Claude(message string) {
	l.Log("CLAUDE", message)
}

// Separator prints a visual separator line.
func (l *Logger) Separator() {
	fmt.Fprintln(l.out, "────────────────────────────────────────────────────────────")
}

// Blank prints a blank line.
func (l *Logger) Blank() {
	fmt.Fprintln(l.out)
}

// Writer returns an io.Writer that logs with the given source prefix.
func (l *Logger) Writer(source string) io.Writer {
	return &prefixWriter{logger: l, source: source}
}

// prefixWriter wraps a logger to implement io.Writer.
type prefixWriter struct {
	logger *Logger
	source string
	buffer []byte
}

func (w *prefixWriter) Write(p []byte) (n int, err error) {
	w.buffer = append(w.buffer, p...)

	// Process complete lines
	for {
		newline := -1
		for i, b := range w.buffer {
			if b == '\n' {
				newline = i
				break
			}
		}

		if newline < 0 {
			break
		}

		line := string(w.buffer[:newline])
		w.buffer = w.buffer[newline+1:]

		if line != "" {
			w.logger.Log(w.source, line)
		}
	}

	return len(p), nil
}

package logger

import (
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Logger struct {
	mu       sync.Mutex
	runtimeF *os.File
	errorF   *os.File
}

var L *Logger

func New(dir string) (*Logger, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, err
	}

	runtimeF, err := os.OpenFile(filepath.Join(dir, "log_runtime.txt"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		return nil, err
	}

	errorF, err := os.OpenFile(filepath.Join(dir, "log_error.txt"),
		os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0644)
	if err != nil {
		runtimeF.Close()
		return nil, err
	}

	return &Logger{runtimeF: runtimeF, errorF: errorF}, nil
}

func (l *Logger) Runtime(format string, args ...any) {
	l.write(l.runtimeF, "RUNTIME", format, args...)
}

func (l *Logger) Error(format string, args ...any) {
	l.write(l.errorF, "ERROR", format, args...)
}

func (l *Logger) write(f *os.File, tag, format string, args ...any) {
	msg := fmt.Sprintf(format, args...)
	line := fmt.Sprintf("[%s] [%s] %s\n",
		time.Now().Format("2006-01-02 15:04:05"), tag, msg)

	l.mu.Lock()
	defer l.mu.Unlock()
	f.WriteString(line)
}

func (l *Logger) Close() {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.runtimeF.Close()
	l.errorF.Close()
}

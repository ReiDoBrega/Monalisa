package logger

import (
	"fmt"
	"io"
	"os"
	"strings"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
	FATAL
)

const (
	colorReset  = "\033[0m"
	colorRed    = "\033[31m"
	colorGreen  = "\033[32m"
	colorYellow = "\033[33m"
	colorBlue   = "\033[34m"
	colorPurple = "\033[35m"
	colorCyan   = "\033[36m"
	colorGray   = "\033[37m"
	colorWhite  = "\033[97m"
)

type Logger struct {
	mu          sync.Mutex
	level       LogLevel
	output      io.Writer
	errorOutput io.Writer
	colored     bool
	timestamp   bool
	prefix      string
}

var (
	// DefaultLogger é o logger padrão da aplicação
	DefaultLogger *Logger
	once          sync.Once
)

// Init inicializa o logger padrão
func Init(level LogLevel, colored bool) {
	once.Do(func() {
		DefaultLogger = &Logger{
			level:       level,
			output:      os.Stdout,
			errorOutput: os.Stderr,
			colored:     colored,
			timestamp:   false,
		}
	})
}

// NewLogger cria um novo logger
func NewLogger(level LogLevel, colored bool) *Logger {
	return &Logger{
		level:       level,
		output:      os.Stdout,
		errorOutput: os.Stderr,
		colored:     colored,
		timestamp:   false,
	}
}

func (l *Logger) SetLevel(level LogLevel) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.level = level
}

func (l *Logger) SetPrefix(prefix string) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prefix = prefix
}

func (l *Logger) SetTimestamp(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.timestamp = enabled
}

func (l *Logger) SetColored(enabled bool) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.colored = enabled
}

func (level LogLevel) String() string {
	switch level {
	case DEBUG:
		return "DEBUG"
	case INFO:
		return "INFO"
	case WARN:
		return "WARN"
	case ERROR:
		return "ERROR"
	case FATAL:
		return "FATAL"
	default:
		return "UNKNOWN"
	}
}

func (l *Logger) getColor(level LogLevel) string {
	if !l.colored {
		return ""
	}
	switch level {
	case DEBUG:
		return colorGray
	case INFO:
		return colorCyan
	case WARN:
		return colorYellow
	case ERROR:
		return colorRed
	case FATAL:
		return colorPurple
	default:
		return colorWhite
	}
}

func (l *Logger) formatMessage(level LogLevel, format string, args ...interface{}) string {
	l.mu.Lock()
	defer l.mu.Unlock()

	var parts []string

	if l.timestamp {
		parts = append(parts, time.Now().Format("2006-01-02 15:04:05"))
	}

	color := l.getColor(level)
	levelStr := fmt.Sprintf("[%s]", level.String())
	if color != "" {
		levelStr = color + levelStr + colorReset
	}
	parts = append(parts, levelStr)

	if l.prefix != "" {
		parts = append(parts, fmt.Sprintf("[%s]", l.prefix))
	}

	message := fmt.Sprintf(format, args...)

	header := strings.Join(parts, " ")
	if header != "" {
		return header + " " + message
	}
	return message
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if level < l.level {
		return
	}

	message := l.formatMessage(level, format, args...)

	writer := l.output
	if level >= ERROR {
		writer = l.errorOutput
	}

	fmt.Fprintln(writer, message)

	if level == FATAL {
		os.Exit(1)
	}
}

func (l *Logger) Debug(format string, args ...interface{}) {
	l.log(DEBUG, format, args...)
}

func (l *Logger) Info(format string, args ...interface{}) {
	l.log(INFO, format, args...)
}

func (l *Logger) Warn(format string, args ...interface{}) {
	l.log(WARN, format, args...)
}

func (l *Logger) Error(format string, args ...interface{}) {
	l.log(ERROR, format, args...)
}

func (l *Logger) Fatal(format string, args ...interface{}) {
	l.log(FATAL, format, args...)
}

func (l *Logger) Success(format string, args ...interface{}) {
	message := format
	l.log(INFO, message, args...)
}

func (l *Logger) Warning(format string, args ...interface{}) {
	message := format
	l.log(WARN, message, args...)
}

func (l *Logger) Failure(format string, args ...interface{}) {
	message := format
	l.log(ERROR, message, args...)
}

func Debug(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Debug(format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Info(format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Warn(format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Error(format, args...)
	}
}

func Fatal(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Fatal(format, args...)
	}
}

func Success(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Success(format, args...)
	}
}

func Warning(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Warning(format, args...)
	}
}

func Failure(format string, args ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Failure(format, args...)
	}
}

func SetLevel(level LogLevel) {
	if DefaultLogger != nil {
		DefaultLogger.SetLevel(level)
	}
}

func SetPrefix(prefix string) {
	if DefaultLogger != nil {
		DefaultLogger.SetPrefix(prefix)
	}
}

func SetTimestamp(enabled bool) {
	if DefaultLogger != nil {
		DefaultLogger.SetTimestamp(enabled)
	}
}

func SetColored(enabled bool) {
	if DefaultLogger != nil {
		DefaultLogger.SetColored(enabled)
	}
}

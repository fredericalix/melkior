package logging

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type LogLevel int

const (
	DEBUG LogLevel = iota
	INFO
	WARN
	ERROR
)

type Logger struct {
	file     *os.File
	logger   *log.Logger
	mu       sync.Mutex
	level    LogLevel
	console  bool
	filePath string
}

var (
	defaultLogger *Logger
	once          sync.Once
)

func Init(logDir string, console bool) error {
	var err error
	once.Do(func() {
		if err := os.MkdirAll(logDir, 0755); err != nil {
			fmt.Printf("Failed to create log directory: %v\n", err)
			return
		}

		timestamp := time.Now().Format("20060102_150405")
		logPath := filepath.Join(logDir, fmt.Sprintf("nodectl_%s.log", timestamp))

		file, openErr := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if openErr != nil {
			err = openErr
			fmt.Printf("Failed to open log file: %v\n", openErr)
			return
		}

		defaultLogger = &Logger{
			file:     file,
			logger:   log.New(file, "", log.Ldate|log.Ltime|log.Lmicroseconds|log.Lshortfile),
			level:    DEBUG,
			console:  console,
			filePath: logPath,
		}

		defaultLogger.Info("=== NODECTL LOG STARTED ===")
		defaultLogger.Info("Log file: %s", logPath)
		defaultLogger.Info("Console output: %v", console)
	})

	if defaultLogger != nil {
		fmt.Printf("Debug log initialized: %s\n", defaultLogger.filePath)
	}

	return err
}

func (l *Logger) log(level LogLevel, format string, args ...interface{}) {
	if l == nil || level < l.level {
		return
	}

	l.mu.Lock()
	defer l.mu.Unlock()

	levelStr := ""
	switch level {
	case DEBUG:
		levelStr = "[DEBUG]"
	case INFO:
		levelStr = "[INFO]"
	case WARN:
		levelStr = "[WARN]"
	case ERROR:
		levelStr = "[ERROR]"
	}

	msg := fmt.Sprintf("%s %s", levelStr, fmt.Sprintf(format, args...))

	if l.logger != nil {
		l.logger.Output(2, msg)
	}

	if l.console {
		fmt.Printf("%s %s\n", time.Now().Format("15:04:05.000"), msg)
	}
}

func Debug(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(DEBUG, format, args...)
	}
}

func Info(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(INFO, format, args...)
	}
}

func Warn(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(WARN, format, args...)
	}
}

func Error(format string, args ...interface{}) {
	if defaultLogger != nil {
		defaultLogger.log(ERROR, format, args...)
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

func Close() {
	if defaultLogger != nil && defaultLogger.file != nil {
		defaultLogger.Info("=== NODECTL LOG CLOSED ===")
		defaultLogger.file.Close()
	}
}

func GetLogPath() string {
	if defaultLogger != nil {
		return defaultLogger.filePath
	}
	return ""
}
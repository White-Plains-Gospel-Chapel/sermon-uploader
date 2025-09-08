package logging

import (
	"fmt"
	"log"
	"os"
	"time"
)

type Logger struct {
	file *os.File
}

func NewLogger(filename string) (*Logger, error) {
	file, err := os.OpenFile(filename, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		return nil, err
	}
	return &Logger{file: file}, nil
}

func (l *Logger) Log(level, message string, args ...interface{}) {
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	logMessage := fmt.Sprintf("[%s] %s: %s", timestamp, level, fmt.Sprintf(message, args...))
	
	// Write to file
	if l.file != nil {
		fmt.Fprintln(l.file, logMessage)
	}
	
	// Also write to stdout
	log.Println(logMessage)
}

func (l *Logger) Info(message string, args ...interface{}) {
	l.Log("INFO", message, args...)
}

func (l *Logger) Error(message string, args ...interface{}) {
	l.Log("ERROR", message, args...)
}

func (l *Logger) Close() error {
	if l.file != nil {
		return l.file.Close()
	}
	return nil
}

type Config struct {
	LogDir        string
	MaxFileSize   int64
	RetentionDays int
}

func New(config *Config) (*Logger, error) {
	filename := fmt.Sprintf("%s/app.log", config.LogDir)
	return NewLogger(filename)
}
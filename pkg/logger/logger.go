package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
	"sync"
)

var (
	mu            sync.RWMutex
	defaultLogger *log.Logger
	logFile       *os.File
	initialized   bool
)

func InitLogger(logPath string, quiet bool) error {
	mu.Lock()
	defer mu.Unlock()

	if initialized && logFile != nil {
		logFile.Close()
		logFile = nil
	}

	var writers []io.Writer

	if logPath != "" {
		dir := filepath.Dir(logPath)
		if dir != "." && dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return err
			}
		}

		file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			return err
		}
		logFile = file
		writers = append(writers, file)
	}

	if !quiet {
		writers = append(writers, os.Stdout)
	}

	if len(writers) == 0 {
		writers = append(writers, io.Discard)
	}

	multiWriter := io.MultiWriter(writers...)
	defaultLogger = log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)
	initialized = true

	return nil
}

func Close() {
	mu.Lock()
	defer mu.Unlock()

	if logFile != nil {
		logFile.Close()
		logFile = nil
	}
	initialized = false
}

func getLogger() *log.Logger {
	mu.RLock()
	defer mu.RUnlock()

	if defaultLogger != nil {
		return defaultLogger
	}
	return log.Default()
}

func Printf(format string, v ...interface{}) {
	getLogger().Printf(format, v...)
}

func Println(v ...interface{}) {
	getLogger().Println(v...)
}

func Print(v ...interface{}) {
	getLogger().Print(v...)
}

func Fatal(v ...interface{}) {
	getLogger().Fatal(v...)
}

func Fatalf(format string, v ...interface{}) {
	getLogger().Fatalf(format, v...)
}

func Fatalln(v ...interface{}) {
	getLogger().Fatalln(v...)
}


package logger

import (
	"io"
	"log"
	"os"
	"path/filepath"
)

var (
	DefaultLogger *log.Logger
	LogFile      *os.File
)

func InitLogger(logPath string, quiet bool) error {
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
		LogFile = file
		writers = append(writers, file)
	}

	if !quiet {
		writers = append(writers, os.Stdout)
	}

	multiWriter := io.MultiWriter(writers...)
	DefaultLogger = log.New(multiWriter, "", log.LstdFlags|log.Lmicroseconds)

	return nil
}

func Close() {
	if LogFile != nil {
		LogFile.Close()
	}
}

func Printf(format string, v ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Printf(format, v...)
	} else {
		log.Printf(format, v...)
	}
}

func Println(v ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Println(v...)
	} else {
		log.Println(v...)
	}
}

func Fatal(v ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Fatal(v...)
	} else {
		log.Fatal(v...)
	}
}

func Fatalf(format string, v ...interface{}) {
	if DefaultLogger != nil {
		DefaultLogger.Fatalf(format, v...)
	} else {
		log.Fatalf(format, v...)
	}
}


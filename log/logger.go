package log

import (
	"log"
	"os"
)

var (
	debugLogger   log.Logger
	infoLogger    log.Logger
	warningLogger log.Logger
	errorLogger   log.Logger
	fatalLogger   log.Logger
)

func init() {
	debugLogger.SetOutput(os.Stderr)
	debugLogger.SetPrefix("[DEBUG]: ")

	infoLogger.SetOutput(os.Stderr)
	infoLogger.SetPrefix("[INFO]: ")

	warningLogger.SetOutput(os.Stderr)
	warningLogger.SetPrefix("[WARN]: ")

	errorLogger.SetOutput(os.Stderr)
	errorLogger.SetPrefix("[ERROR]: ")

	fatalLogger.SetOutput(os.Stderr)
	fatalLogger.SetPrefix("[FATAL]: ")
}

func Debug(v ...interface{}) {
	debugLogger.Println(v)
}

func Debugf(f string, v ...interface{}) {
	debugLogger.Printf(f, v)
}

func Info(v ...interface{}) {
	infoLogger.Println(v)
}

func Infof(f string, v ...interface{}) {
	infoLogger.Printf(f, v)
}

func Warn(v ...interface{}) {
	warningLogger.Println(v)
}

func Warnf(f string, v ...interface{}) {
	warningLogger.Printf(f, v)
}

func Error(v ...interface{}) {
	errorLogger.Println(v)
}

func Errorf(f string, v ...interface{}) {
	errorLogger.Printf(f, v)
}

func Fatal(v ...interface{}) {
	fatalLogger.Fatal(v)
}

func Fatalf(f string, v ...interface{}) {
	fatalLogger.Fatalf(f, v)
}

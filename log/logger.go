package log

import (
	"log"
	"sync"
)

const (
	debugPrefix = "[DEBUG]:"
	infoPrefix  = "[INFO]:"
	warnPrefix  = "[WARN]:"
	errorPrefix = "[ERROR]:"
	fatalPrefix = "[FATAL]:"
)

var pool = sync.Pool{
	New: func() interface{} {
		return make([]interface{}, 0, 2)
	},
}

func Debug(v ...interface{}) {
	buf := pool.Get().([]interface{})
	buf = append(buf, debugPrefix)
	buf = append(buf, v...)

	log.Println(buf...)

	pool.Put(buf[:0])
}

func Debugf(f string, v ...interface{}) {
	log.Printf(debugPrefix+" "+f, v...)
}

func Info(v ...interface{}) {
	buf := pool.Get().([]interface{})
	buf = append(buf, infoPrefix)
	buf = append(buf, v...)

	log.Println(buf...)

	pool.Put(buf[:0])
}

func Infof(f string, v ...interface{}) {
	log.Printf(debugPrefix+" "+f, v...)
}

func Warn(v ...interface{}) {
	buf := pool.Get().([]interface{})
	buf = append(buf, warnPrefix)
	buf = append(buf, v...)

	log.Println(buf...)

	pool.Put(buf[:0])
}

func Warnf(f string, v ...interface{}) {
	log.Printf(warnPrefix+" "+f, v...)
}

func Error(v ...interface{}) {
	buf := pool.Get().([]interface{})
	buf = append(buf, errorPrefix)
	buf = append(buf, v...)

	log.Println(buf...)

	pool.Put(buf[:0])
}

func Errorf(f string, v ...interface{}) {
	log.Printf(errorPrefix+" "+f, v...)
}

func Fatal(v ...interface{}) {
	buf := pool.Get().([]interface{})
	buf = append(buf, fatalPrefix)
	buf = append(buf, v...)

	log.Fatalln(buf...)

	pool.Put(buf[:0])
}

func Fatalf(f string, v ...interface{}) {
	log.Fatalf(fatalPrefix+" "+f, v)
}

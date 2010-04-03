// Logs information of peers
// Roger Pau Monné - 2010
// Distributed under the terms of the GNU GPLv3

package main

import(
	"sync"
	"os"
	"time"
	"fmt"
	)
	
type logger struct {
	fd *os.File
	mutex *sync.Mutex
}

func NewLogger(addr string) (l *logger, err os.Error) {
	l = new(logger)
	l.mutex = new(sync.Mutex)
	l.fd, err = os.Open("logs/"+addr, os.O_WRONLY | os.O_TRUNC | os.O_CREAT, 0666)
	return
}

func (l *logger) Output(v ...interface{}) {
	l.mutex.Lock()
	defer l.mutex.Unlock()
	actual := time.LocalTime()
	//l.fd.WriteString(fmt.Sprintln(actual.Year, "/", actual.Month, "/", actual.Day, " ", actual.Hour, ":", actual.Minute, ":", actual.Second, " ", v)) 
	l.fd.WriteString(actual.Format(time.ISO8601) + " " + fmt.Sprintln(v))
}

func (l *logger) Close() {
	l.fd.WriteString("Closing logger")
	l.fd.Close()
	l.mutex = nil
	l.fd = nil
}

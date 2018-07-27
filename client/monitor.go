package client

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
	"time"
)

type Monitor struct {
	tcpConnections  int32
	baseConnections int32
}

const reportFormant = `TCP connections: %d
Base connections: %d`

func (m *Monitor) report(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintf(reportFormant, atomic.LoadInt32(&m.tcpConnections), atomic.LoadInt32(&m.baseConnections)))
}

func (m *Monitor) start(addr string, port int) (err error) {
	// log.Println("monitor init:", http.ListenAndServe(net.JoinHostPort(addr, strconv.Itoa(port)), http.HandlerFunc(m.report)))
	go func() {
		err = http.ListenAndServe(net.JoinHostPort(addr, strconv.Itoa(port)), http.HandlerFunc(m.report))
	}()

	time.Sleep(time.Second)

	if err != nil {
		return fmt.Errorf("monitor init: %s", err)
	}

	return
}

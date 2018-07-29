package client

import (
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
)

type Monitor struct {
	tcpConnections  int32
	baseConnections int32
}

const reportFormat = `
TCP connections: %d
Base connections: %d
`

func (m *Monitor) report(w http.ResponseWriter, r *http.Request) {
	io.WriteString(w, fmt.Sprintf(reportFormat, atomic.LoadInt32(&m.tcpConnections), atomic.LoadInt32(&m.baseConnections)))
}

func (m *Monitor) start(addr string, port int) {
	go log.Println(http.ListenAndServe(net.JoinHostPort(addr, strconv.Itoa(port)), http.HandlerFunc(m.report)))
}

func (m *Monitor) updateMonitor(tcpConnChange, baseConnChange int32) {
	atomic.AddInt32(&m.tcpConnections, tcpConnChange)
	atomic.AddInt32(&m.baseConnections, baseConnChange)
}

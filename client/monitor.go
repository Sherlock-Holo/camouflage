package client

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"strconv"
	"sync/atomic"
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

func (m *Monitor) start(addr string, port int) {
	http.ListenAndServe(net.JoinHostPort(addr, strconv.Itoa(port)), http.HandlerFunc(m.report))
}

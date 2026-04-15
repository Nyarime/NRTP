package nrtp

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
)

// Fallback 统一回落处理
type Fallback struct {
	Mode        string
	Target      string
	HTTPHandler http.Handler
}

func (f *Fallback) Handle(conn net.Conn) {
	defer conn.Close()

	switch f.Mode {
	case "handler":
		if f.HTTPHandler != nil {
			serveHTTPOnConn(conn, f.HTTPHandler)
		}
	case "proxy":
		backend, err := net.Dial("tcp", f.Target)
		if err != nil { return }
		defer backend.Close()
		done := make(chan struct{}, 2)
		go func() { io.Copy(backend, conn); done <- struct{}{} }()
		go func() { io.Copy(conn, backend); done <- struct{}{} }()
		<-done
	default:
		PortalServeHTTP(conn)
	}
}

func serveHTTPOnConn(conn net.Conn, handler http.Handler) {
	br := bufio.NewReader(conn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil { return }

		rw := &connResponseWriter{
			conn:   conn,
			header: make(http.Header),
		}
		handler.ServeHTTP(rw, req)
		req.Body.Close()

		if req.Header.Get("Connection") == "close" { return }
	}
}

type connResponseWriter struct {
	conn        net.Conn
	header      http.Header
	statusCode  int
	wroteHeader bool
}

func (w *connResponseWriter) Header() http.Header { return w.header }

func (w *connResponseWriter) WriteHeader(code int) {
	if w.wroteHeader { return }
	w.wroteHeader = true
	w.statusCode = code

	resp := fmt.Sprintf("HTTP/1.1 %d %s\r\n", code, http.StatusText(code))
	for k, vs := range w.header {
		for _, v := range vs {
			resp += fmt.Sprintf("%s: %s\r\n", k, v)
		}
	}
	resp += "\r\n"
	w.conn.Write([]byte(resp))
}

func (w *connResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader { w.WriteHeader(200) }
	return w.conn.Write(b)
}

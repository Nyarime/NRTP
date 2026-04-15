package nrtp

import (
	"bufio"
	"io"
	"log"
	"net"
	"net/http"
)

// Fallback 统一回落处理
type Fallback struct {
	Mode        string       // portal / proxy / handler
	Target      string       // proxy: 反代目标
	HTTPHandler http.Handler // handler: 自定义HTTP handler (Lite的embed模板)
}

// Handle 处理非认证连接
func (f *Fallback) Handle(conn net.Conn) {
	defer conn.Close()

	switch f.Mode {
	case "handler":
		// 自定义HTTP handler (serve embed模板)
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

	default: // portal
		PortalServeHTTP(conn)
	}
}

// serveHTTPOnConn 在原始连接上serve HTTP
func serveHTTPOnConn(conn net.Conn, handler http.Handler) {
	br := bufio.NewReader(conn)
	for {
		req, err := http.ReadRequest(br)
		if err != nil { return }

		rw := &connResponseWriter{conn: conn, header: make(http.Header)}
		handler.ServeHTTP(rw, req)
		req.Body.Close()

		if req.Header.Get("Connection") == "close" { return }
	}
}

// connResponseWriter 适配net.Conn为http.ResponseWriter
type connResponseWriter struct {
	conn       net.Conn
	header     http.Header
	statusCode int
	wroteHeader bool
}

func (w *connResponseWriter) Header() http.Header { return w.header }

func (w *connResponseWriter) WriteHeader(code int) {
	if w.wroteHeader { return }
	w.wroteHeader = true
	w.statusCode = code

	statusText := http.StatusText(code)
	resp := "HTTP/1.1 " + http.StatusText(code) + "\r\n"
	_ = statusText
	resp = "HTTP/1.1 " + string(rune('0'+code/100)) + string(rune('0'+(code/10)%10)) + string(rune('0'+code%10)) + " " + http.StatusText(code) + "\r\n"

	for k, vs := range w.header {
		for _, v := range vs {
			resp += k + ": " + v + "\r\n"
		}
	}
	resp += "\r\n"
	w.conn.Write([]byte(resp))
}

func (w *connResponseWriter) Write(b []byte) (int, error) {
	if !w.wroteHeader { w.WriteHeader(200) }
	return w.conn.Write(b)
}

func init() {
	_ = log.Println
}

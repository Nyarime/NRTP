package nrtp

import (
	"log"
	"net"
	"sync"
	"time"

	"github.com/xtaci/smux"
)

// MuxListener 带smux的NRTP监听器
// Accept返回的是smux stream而不是TLS conn
type MuxListener struct {
	listener *Listener
	sessions []*smux.Session
	streams  chan net.Conn
	mu       sync.Mutex
}

// ListenMux 创建带mux的NRTP监听器
func ListenMux(addr string, cfg *Config) (*MuxListener, error) {
	listener, err := Listen(addr, cfg)
	if err != nil {
		return nil, err
	}
	ml := &MuxListener{
		listener: listener,
		streams:  make(chan net.Conn, 256),
	}
	go ml.acceptLoop()
	return ml, nil
}

func (ml *MuxListener) acceptLoop() {
	for {
		conn, err := ml.listener.Accept()
		if err != nil {
			return
		}
		go ml.handleConn(conn)
	}
}

func (ml *MuxListener) handleConn(conn net.Conn) {
	// 读MUX标记
	head := make([]byte, 4)
	n, err := conn.Read(head)
	if err != nil || n < 3 {
		conn.Close()
		return
	}

	if string(head[:3]) != "MUX" {
		conn.Close()
		return
	}
	conn.Write([]byte{0x01})

	cfg := smux.DefaultConfig()
	cfg.MaxReceiveBuffer = 16 * 1024 * 1024
	cfg.KeepAliveInterval = 10 * time.Second
	cfg.KeepAliveTimeout = 60 * time.Second

	session, err := smux.Server(conn, cfg)
	if err != nil {
		conn.Close()
		return
	}

	ml.mu.Lock()
	ml.sessions = append(ml.sessions, session)
	ml.mu.Unlock()

	log.Printf("[NRTP] mux session就绪")

	for {
		stream, err := session.AcceptStream()
		if err != nil {
			return
		}
		ml.streams <- stream
	}
}

// Accept 返回smux stream
func (ml *MuxListener) Accept() (net.Conn, error) {
	stream, ok := <-ml.streams
	if !ok {
		return nil, net.ErrClosed
	}
	return stream, nil
}

// Close 关闭监听器
func (ml *MuxListener) Close() error {
	close(ml.streams)
	return ml.listener.Close()
}

// Addr 返回监听地址
func (ml *MuxListener) Addr() net.Addr {
	return ml.listener.Addr()
}

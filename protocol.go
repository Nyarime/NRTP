package nrtp

import (
	"encoding/binary"
	"fmt"
	"io"
	"net"
)

const (
	ProtoVersion    = 0x01
	CmdTCPConnect   = 0x01
	CmdUDPAssociate = 0x03
	AddrTypeIPv4    = 0x01
	AddrTypeIPv6    = 0x02
	AddrTypeDomain  = 0x03
)

// EncodeTargetFrame 编码0-RTT目标帧
// [1B ver][1B cmd][1B atyp][2B port][...addr]
func EncodeTargetFrame(cmd byte, addr string) ([]byte, error) {
	host, portStr, err := net.SplitHostPort(addr)
	if err != nil { return nil, err }
	port := 0
	fmt.Sscanf(portStr, "%d", &port)

	buf := []byte{ProtoVersion, cmd}

	ip := net.ParseIP(host)
	if ip4 := ip.To4(); ip4 != nil {
		buf = append(buf, AddrTypeIPv4)
		portBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(portBuf, uint16(port))
		buf = append(buf, portBuf...)
		buf = append(buf, ip4...)
	} else if ip16 := ip.To16(); ip16 != nil {
		buf = append(buf, AddrTypeIPv6)
		portBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(portBuf, uint16(port))
		buf = append(buf, portBuf...)
		buf = append(buf, ip16...)
	} else {
		// Domain
		buf = append(buf, AddrTypeDomain)
		portBuf := make([]byte, 2)
		binary.BigEndian.PutUint16(portBuf, uint16(port))
		buf = append(buf, portBuf...)
		buf = append(buf, byte(len(host)))
		buf = append(buf, []byte(host)...)
	}
	return buf, nil
}

// ParseTargetFrame 解析0-RTT目标帧
func ParseTargetFrame(r io.Reader) (network, addr string, err error) {
	header := make([]byte, 3)
	if _, err = io.ReadFull(r, header); err != nil { return }
	if header[0] != ProtoVersion {
		return "", "", fmt.Errorf("unsupported version: %d", header[0])
	}

	cmd := header[1]
	atyp := header[2]

	portBuf := make([]byte, 2)
	if _, err = io.ReadFull(r, portBuf); err != nil { return }
	port := binary.BigEndian.Uint16(portBuf)

	var host string
	switch atyp {
	case AddrTypeIPv4:
		ip := make([]byte, 4)
		io.ReadFull(r, ip)
		host = net.IP(ip).String()
	case AddrTypeIPv6:
		ip := make([]byte, 16)
		io.ReadFull(r, ip)
		host = net.IP(ip).String()
	case AddrTypeDomain:
		lenBuf := make([]byte, 1)
		io.ReadFull(r, lenBuf)
		domain := make([]byte, lenBuf[0])
		io.ReadFull(r, domain)
		host = string(domain)
	default:
		return "", "", fmt.Errorf("unknown addr type: %d", atyp)
	}

	network = "tcp"
	if cmd == CmdUDPAssociate { network = "udp" }
	addr = fmt.Sprintf("%s:%d", host, port)
	return
}

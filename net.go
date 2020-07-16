// ----build !js

package lnd

import (
	"context"
	"fmt"
	"net"
	"strings"
	"syscall/js"

	"nhooyr.io/websocket"
)

var hostIP uint32 = 127*(16777216)

var uint8Array = js.Global().Get("Uint8Array")

// WSNet implements the tor.Net interface
type WSNet struct {
	ctx           context.Context
	lookupHostMap map[string]string
}

func NewWSNet() *WSNet {
	return &WSNet{
		ctx:           context.Background(),
		lookupHostMap: make(map[string]string),
	}
}

// Dial connects to the address on the named network.
func (w *WSNet) Dial(network, address string) (net.Conn, error) {
	addrParts := strings.Split(address, ":")
	if len(addrParts) > 1 {
		switch addrParts[1] {
		case "443":
			network = "wss"
		case "80":
			network = "ws"
		default:
		}
	} else {
		addrParts = append(addrParts, "80")
		network = "ws"
	}
	wsDial := fmt.Sprintf("%s://%s:%s", network, w.lookupHostMap[addrParts[0]], addrParts[1])
	ws, _, err :=  websocket.Dial(w.ctx, wsDial, &websocket.DialOptions{})
	if err != nil {
		return nil, err
	}
	ws.SetReadLimit(4500000)
	return websocket.NetConn(w.ctx, ws, websocket.MessageBinary), nil
}

// LookupHost does nothing for now
func (w *WSNet) LookupHost(host string) ([]string, error) {
	hostIP++
	octet4 := hostIP
	octet1 := octet4 / 16777216
	octet4 -= octet1 * 16777216
	octet2 := octet4 / 65536
	octet4 -= octet2 * 65536
	octet3 := octet4 / 256
	octet4 -= octet3 * 256
	ipString := fmt.Sprintf("%d.%d.%d.%d", octet1, octet2, octet3, octet4)
	w.lookupHostMap[ipString] = host
	return []string{ipString}, nil
}

// LookupSRV does nothing for now
func (w *WSNet) LookupSRV(service, proto, name string) (string, []*net.SRV, error) {
	ltndLog.Warnf("ws lookupsrv: %s %s %s", service, proto, name);
	return "", []*net.SRV{}, nil
}

// ResolveTCPAddr does nothing for now
func (w *WSNet) ResolveTCPAddr(network, address string) (*net.TCPAddr, error) {
	ltndLog.Warnf("ws resolvetcpaddr: %s %s", network, address);
	return nil, nil
}

// MCListener implements the net.Listener interface and provides a context dialer for GRPC
type MCListener struct {
	onMessage js.Func
	connect   chan js.Value
	dial      chan net.Conn
}

func NewMCListener(mc js.Value) (*MCListener, error) {
	m := &MCListener{connect: make(chan js.Value), dial: make(chan net.Conn)}
	m.onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			if len(args) > 0 {
				ports := args[0].Get("ports")
				if ports.Length() > 0 {
					m.connect <- ports.Index(0)
				}
			}
		}()
		return nil
	})
	mc.Set("onmessage", m.onMessage)
	return m, nil
}

func (m *MCListener) Accept() (net.Conn, error) {
	select {
	case connection := <-m.connect:
		return newMcConn(connection)
	case conn := <-m.dial:
		return conn, nil
	}
}

func (m *MCListener) Close() error {
	m.onMessage.Release()
	return nil
}

func (m *MCListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127,0,0,1), Port: 443}
}

// PipeDial implements a context dialer for GRPC proxy
func (m *MCListener) PipeDial(ctx context.Context, addr string) (net.Conn, error) {
	ourConn, theirConn := net.Pipe()
	m.dial <- theirConn
	return ourConn, nil
}

// mcConn implements the net.Conn interface
type mcConn struct {
	net.Conn

	ourConn   net.Conn
	onMessage js.Func
	mc        js.Value

}

func newMcConn(mc js.Value) (net.Conn, error) {
	ourConn, theirConn := net.Pipe()
	c := &mcConn{
		Conn:      theirConn,
		ourConn:   ourConn,
		mc:        mc,
	}
	c.onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			if len(args) > 0 {
				val := args[0].Get("data")
				length := val.Length()
				buf := make([]byte, length) 
				js.CopyBytesToGo(buf, val)
				c.ourConn.Write(buf)
			}
		}()
		return nil
	})
	mc.Set("onmessage", c.onMessage)
	go c.handler()
	return c, nil
}

func (c *mcConn) handler() {
	buf := make([]byte, 1024 * 1024 * 200) // see lnd.go restDialOpts and cmd/lncli/main.go maxMsgRecvSize
	for {
		n, err := c.ourConn.Read(buf)
		if err != nil { // connection closed, time to bail
			return
		}
		b := uint8Array.New(n)
		js.CopyBytesToJS(b, buf[:n])
		c.mc.Call("postMessage", b)
	}
}

func (c *mcConn) Close() error {
	c.ourConn.Close()
	return c.Conn.Close()
}

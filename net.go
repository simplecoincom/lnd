// ----build !js

package lnd

import (
	"context"
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall/js"

	"nhooyr.io/websocket"
)

var (
	hostIP uint32 = 127*(16777216)

	uint8Array = js.Global().Get("Uint8Array")

	ErrListenerClosed = fmt.Errorf("Listener has been closed")
)

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

// MCListener implements the net.Listener interface backed by a JS MessageChannel that accepts other MessagePorts as connections
type MCListener struct {
	quit      sync.Once
	onMessage js.Func
	connect   chan js.Value
	done      chan struct{}
}

func NewMCListener(mc js.Value) (*MCListener, error) {
	l := &MCListener{connect: make(chan js.Value), done: make(chan struct{})}
	l.onMessage = js.FuncOf(func(this js.Value, args []js.Value) interface{} {
		go func() {
			if len(args) > 0 {
				ports := args[0].Get("ports")
				if ports.Length() > 0 {
					l.connect <- ports.Index(0)
				}
			}
		}()
		return nil
	})
	mc.Set("onmessage", l.onMessage)
	return l, nil
}

func (l *MCListener) Accept() (net.Conn, error) {
	select {
	case <-l.done:
		return nil, ErrListenerClosed
	case connection := <-l.connect:
		return newMcConn(connection)
	}
}

func (l *MCListener) Close() error {
	l.onMessage.Release()
	l.quit.Do(func() {
		close(l.done)
	})
	return nil
}

func (l *MCListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127,0,0,1), Port: 443}
}

// PipeListener implements the net.Listener interface and provides a context dialer for GRPC
type PipeListener struct {
	octx   context.Context
	ctx    context.Context
	cancel func()
	dial   chan net.Conn
}

func NewPipeListener(ctx context.Context) (*PipeListener, error) {
	l := &PipeListener{octx: ctx, dial: make(chan net.Conn)}
	l.ctx, l.cancel = context.WithCancel(l.octx)
	return l, nil
}

func (l *PipeListener) Accept() (net.Conn, error) {
	select {
	case <-l.ctx.Done():
		return nil, ErrListenerClosed
	case conn := <-l.dial:
		return conn, nil
	}
}

func (l *PipeListener) Close() error {
	l.cancel()
	l.ctx, l.cancel = context.WithCancel(l.octx)
	return nil
}

func (l *PipeListener) Addr() net.Addr {
	return &net.TCPAddr{IP: net.IPv4(127,0,0,1), Port: 443}
}

// Dial implements a context dialer for GRPC proxy
func (l *PipeListener) Dial(ctx context.Context, addr string) (net.Conn, error) {
	ourConn, theirConn := net.Pipe()
	select {
	case <- ctx.Done():
		go func() {
			ourConn.Close()
			theirConn.Close()
		}()
		return nil, ErrListenerClosed
	case l.dial <- theirConn:
		return ourConn, nil
	}
}

// mcConn implements the net.Conn interface backed by a MessagePort passed over an MCListener
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

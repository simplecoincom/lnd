// ----build !js

package lnd

import (
	"context"
	"fmt"
	"net"
	"strings"

	"nhooyr.io/websocket"
)

var hostIP uint32 = 127*(16777216)

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
	fmt.Printf("Dialing %s %s\n", network, address)
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
	fmt.Printf("WS Dial: %s\n", wsDial)
	ws, _, err :=  websocket.Dial(w.ctx, wsDial, &websocket.DialOptions{})
	if err != nil {
		fmt.Println("dialfail")
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
	fmt.Printf("ws lookuphost: %s %s\n", host, ipString);
	w.lookupHostMap[ipString] = host
	return []string{ipString}, nil
}

// LookupSRV does nothing for now
func (w *WSNet) LookupSRV(service, proto, name string) (string, []*net.SRV, error) {
	fmt.Println("ws lookupsrv");
	return "", []*net.SRV{}, nil
}

// ResolveTCPAddr does nothing for now
func (w *WSNet) ResolveTCPAddr(network, address string) (*net.TCPAddr, error) {
	fmt.Printf("ws resolvetcpaddr: %s %s\n", network, address);
	return nil, nil
}

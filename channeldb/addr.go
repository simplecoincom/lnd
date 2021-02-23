package channeldb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"net"

	"github.com/lightningnetwork/lnd/tor"
	"github.com/davecgh/go-spew/spew"
)

// addressType specifies the network protocol and version that should be used
// when connecting to a node at a particular address.
type addressType uint8

const (
	// tcp4Addr denotes an IPv4 TCP address.
	tcp4Addr addressType = 0

	// tcp6Addr denotes an IPv6 TCP address.
	tcp6Addr addressType = 1

	// v2OnionAddr denotes a version 2 Tor onion service address.
	v2OnionAddr addressType = 2

	// v3OnionAddr denotes a version 3 Tor (prop224) onion service address.
	v3OnionAddr addressType = 3

	// wsAddr denotes a websocket (string) address.
	wsAddr addressType = 4
)

// encodeTCPAddr serializes a TCP address into its compact raw bytes
// representation.
func encodeTCPAddr(w io.Writer, addr *net.TCPAddr) error {
	var (
		addrType byte
		ip       []byte
	)

	if addr.IP.To4() != nil {
		addrType = byte(tcp4Addr)
		ip = addr.IP.To4()
	} else {
		addrType = byte(tcp6Addr)
		ip = addr.IP.To16()
	}

	if ip == nil {
		return fmt.Errorf("unable to encode IP %v", addr.IP)
	}

	if _, err := w.Write([]byte{addrType}); err != nil {
		return err
	}

	if _, err := w.Write(ip); err != nil {
		return err
	}

	var port [2]byte
	byteOrder.PutUint16(port[:], uint16(addr.Port))
	if _, err := w.Write(port[:]); err != nil {
		return err
	}

	return nil
}

// encodeOnionAddr serializes an onion address into its compact raw bytes
// representation.
func encodeOnionAddr(w io.Writer, addr *tor.OnionAddr) error {
	var suffixIndex int
	hostLen := len(addr.OnionService)
	switch hostLen {
	case tor.V2Len:
		if _, err := w.Write([]byte{byte(v2OnionAddr)}); err != nil {
			return err
		}
		suffixIndex = tor.V2Len - tor.OnionSuffixLen
	case tor.V3Len:
		if _, err := w.Write([]byte{byte(v3OnionAddr)}); err != nil {
			return err
		}
		suffixIndex = tor.V3Len - tor.OnionSuffixLen
	default:
		return errors.New("unknown onion service length")
	}

	suffix := addr.OnionService[suffixIndex:]
	if suffix != tor.OnionSuffix {
		return fmt.Errorf("invalid suffix \"%v\"", suffix)
	}

	host, err := tor.Base32Encoding.DecodeString(
		addr.OnionService[:suffixIndex],
	)
	if err != nil {
		return err
	}

	// Sanity check the decoded length.
	switch {
	case hostLen == tor.V2Len && len(host) != tor.V2DecodedLen:
		return fmt.Errorf("onion service %v decoded to invalid host %x",
			addr.OnionService, host)

	case hostLen == tor.V3Len && len(host) != tor.V3DecodedLen:
		return fmt.Errorf("onion service %v decoded to invalid host %x",
			addr.OnionService, host)
	}

	if _, err := w.Write(host); err != nil {
		return err
	}

	var port [2]byte
	byteOrder.PutUint16(port[:], uint16(addr.Port))
	if _, err := w.Write(port[:]); err != nil {
		return err
	}

	return nil
}

// encodeWSAddr encodes an address that can be passed to a browser's websocket api
func encodeWSAddr(w io.Writer, addr *tor.WSAddr) error {
	if _, err := w.Write([]byte{byte(wsAddr)}); err != nil {
		return err
	}

	switch addr.Network() {
	case "ws":
		if _, err := w.Write([]byte("w")); err != nil {
			return err
		}
	case "wss":
		if _, err := w.Write([]byte("s")); err != nil {
			return err
		}
	}

	var l [2]byte
	byteOrder.PutUint16(l[:], uint16(len(addr.String())))
	if _, err := w.Write(l[:]); err != nil {
		return err
	}

	if _, err := w.Write([]byte(addr.String())); err != nil {
		return err
	}

	return nil
}

// deserializeAddr reads the serialized raw representation of an address and
// deserializes it into the actual address. This allows us to avoid address
// resolution within the channeldb package.
func deserializeAddr(r io.Reader) (net.Addr, error) {
	var addrType [1]byte
	if _, err := r.Read(addrType[:]); err != nil {
		return nil, err
	}

	var address net.Addr
	switch addressType(addrType[0]) {
	case tcp4Addr:
		var ip [4]byte
		if _, err := r.Read(ip[:]); err != nil {
			return nil, err
		}

		var port [2]byte
		if _, err := r.Read(port[:]); err != nil {
			return nil, err
		}

		address = &net.TCPAddr{
			IP:   net.IP(ip[:]),
			Port: int(binary.BigEndian.Uint16(port[:])),
		}
	case tcp6Addr:
		var ip [16]byte
		if _, err := r.Read(ip[:]); err != nil {
			return nil, err
		}

		var port [2]byte
		if _, err := r.Read(port[:]); err != nil {
			return nil, err
		}

		address = &net.TCPAddr{
			IP:   net.IP(ip[:]),
			Port: int(binary.BigEndian.Uint16(port[:])),
		}
	case v2OnionAddr:
		var h [tor.V2DecodedLen]byte
		if _, err := r.Read(h[:]); err != nil {
			return nil, err
		}

		var p [2]byte
		if _, err := r.Read(p[:]); err != nil {
			return nil, err
		}

		onionService := tor.Base32Encoding.EncodeToString(h[:])
		onionService += tor.OnionSuffix
		port := int(binary.BigEndian.Uint16(p[:]))

		address = &tor.OnionAddr{
			OnionService: onionService,
			Port:         port,
		}
	case v3OnionAddr:
		var h [tor.V3DecodedLen]byte
		if _, err := r.Read(h[:]); err != nil {
			return nil, err
		}

		var p [2]byte
		if _, err := r.Read(p[:]); err != nil {
			return nil, err
		}

		onionService := tor.Base32Encoding.EncodeToString(h[:])
		onionService += tor.OnionSuffix
		port := int(binary.BigEndian.Uint16(p[:]))

		address = &tor.OnionAddr{
			OnionService: onionService,
			Port:         port,
		}
	case wsAddr:
		var n [1]byte
		if _, err := r.Read(n[:]); err != nil {
			return nil, err
		}

		var network string
		switch string(n[:]) {
		case "w":
			network = "ws"
		case "s":
			network = "wss"
		}

		var l [2]byte
		if _, err := r.Read(l[:]); err != nil {
			return nil, err
		}
		length := int(byteOrder.Uint16(l[:]))

		a := make([]byte, length)
		if _, err := r.Read(a[:]); err != nil {
			return nil, err
		}

		address = tor.NewWSAddr(network,string(a))
	default:
		return nil, ErrUnknownAddressType
	}

	return address, nil
}

// serializeAddr serializes an address into its raw bytes representation so that
// it can be deserialized without requiring address resolution.
func serializeAddr(w io.Writer, address net.Addr) error {
	fmt.Println(spew.Sdump(address))
	switch addr := address.(type) {
	case *net.TCPAddr:
		return encodeTCPAddr(w, addr)
	case *tor.OnionAddr:
		return encodeOnionAddr(w, addr)
	case *tor.WSAddr:
		return encodeWSAddr(w, addr)
	default:
		return ErrUnknownAddressType
	}
}

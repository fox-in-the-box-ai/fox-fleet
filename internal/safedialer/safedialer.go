package safedialer

import (
	"fmt"
	"net"
	"syscall"
	"time"
)

func New() *net.Dialer {
	return &net.Dialer{
		Timeout: 10 * time.Second,
		Control: checkAddr,
	}
}

func checkAddr(_, address string, _ syscall.RawConn) error {
	host, _, err := net.SplitHostPort(address)
	if err != nil {
		return fmt.Errorf("safedialer: invalid address %q: %w", address, err)
	}
	ip := net.ParseIP(host)
	if ip == nil {
		return fmt.Errorf("safedialer: resolved address %q is not an IP", host)
	}
	if !isPublic(ip) {
		return fmt.Errorf("safedialer: address %s is not a public IP", ip)
	}
	return nil
}

func isPublic(ip net.IP) bool {
	if ip.IsLoopback() || ip.IsPrivate() || ip.IsLinkLocalUnicast() ||
		ip.IsLinkLocalMulticast() || ip.IsUnspecified() {
		return false
	}
	if ip4 := ip.To4(); ip4 != nil {
		if ip4[0] == 100 && ip4[1] >= 64 && ip4[1] <= 127 {
			return false
		}
	}
	return true
}

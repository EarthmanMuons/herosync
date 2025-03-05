package config

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/miekg/dns"
)

// resolveGoPro returns a URL for connecting to the GoPro camera using the specified
// host and scheme. If host is empty, mDNS discovery is used.
func resolveGoPro(host, scheme string) (*url.URL, error) {
	// Use http by default
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("invalid scheme: %s; choose http or https", scheme)
	}

	resolvedHost, err := resolveHost(host)
	if err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme: scheme,
		Host:   resolvedHost,
	}, nil
}

// resolveHost takes a host string (which may include a port) and returns a resolved
// host:port string with an IPv4 address. If the host is empty, mDNS discovery is used.
func resolveHost(host string) (string, error) {
	// Parse as URL to handle port correctly
	u, err := url.Parse("//" + host)
	if err != nil {
		return "", fmt.Errorf("invalid host format: %v", err)
	}

	hostname := u.Hostname()
	port := u.Port()

	// Handle empty hostname (discovery), DNS lookup, or direct IP
	if hostname == "" {
		ip, err := findGoPro()
		if err != nil {
			return "", fmt.Errorf("auto-discovery failed: %v", err)
		}
		hostname = ip.String()
	} else if net.ParseIP(hostname) == nil {
		ip, err := resolveIPv4(hostname)
		if err != nil {
			return "", err
		}
		hostname = ip.String()
	}

	if port != "" {
		return net.JoinHostPort(hostname, port), nil
	}
	return hostname, nil
}

// resolveIPv4 looks up the first IPv4 address for a hostname.
func resolveIPv4(hostname string) (net.IP, error) {
	ips, err := net.LookupIP(hostname)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve hostname %s: %v", hostname, err)
	}
	for _, addr := range ips {
		if v4 := addr.To4(); v4 != nil {
			return v4, nil
		}
	}
	return nil, fmt.Errorf("no IPv4 address found for %s", hostname)
}

// findGoPro discovers a GoPro camera on the local network using mDNS.
// It searches for the _gopro-web._tcp.local. service and returns the camera's IP address.
func findGoPro() (net.IP, error) {
	multicastConn, err := net.ListenMulticastUDP("udp4", nil,
		&net.UDPAddr{
			IP:   net.ParseIP("224.0.0.251"),
			Port: 5353,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create multicast connection: %v", err)
	}
	defer multicastConn.Close()

	// Build and send mDNS query
	msg := new(dns.Msg)
	msg.SetQuestion("_gopro-web._tcp.local.", dns.TypePTR)
	msg.RecursionDesired = false

	buf, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack message: %v", err)
	}

	if _, err := multicastConn.WriteToUDP(buf, &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	}); err != nil {
		return nil, fmt.Errorf("failed to send query: %v", err)
	}

	// Wait for response
	response := make([]byte, 65536)
	multicastConn.SetReadDeadline(time.Now().Add(4 * time.Second))

	for {
		n, _, err := multicastConn.ReadFromUDP(response)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				return nil, fmt.Errorf("timeout waiting for response")
			}
			return nil, fmt.Errorf("error reading response: %v", err)
		}

		resp := new(dns.Msg)
		if err := resp.Unpack(response[:n]); err != nil {
			continue
		}

		// Return first A record found
		for _, answer := range append(resp.Answer, resp.Extra...) {
			if a, ok := answer.(*dns.A); ok {
				return a.A, nil
			}
		}
	}
}

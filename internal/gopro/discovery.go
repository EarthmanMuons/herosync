package gopro

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/miekg/dns"
)

// resolveGoPro determines the GoPro's IP address, using mDNS or DNS resolution if necessary.
func resolveGoPro(host, scheme string) (*url.URL, error) {
	// Ensure a valid scheme
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("invalid scheme: %s; choose http or https", scheme)
	}

	// Resolve host to an IP address if needed.
	resolvedHost, err := resolveHost(host)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve GoPro address: %w", err)
	}

	return &url.URL{
		Scheme: scheme,
		Host:   resolvedHost,
	}, nil
}

// resolveHost ensures the returned address is an IP while preserving the port.
func resolveHost(host string) (string, error) {
	if host == "" {
		// Auto-discover GoPro via mDNS and use the default API port.
		ip, err := findGoPro()
		if err != nil {
			return "", fmt.Errorf("auto-discovery failed: %w", err)
		}
		return net.JoinHostPort(ip.String(), "8080"), nil
	}

	// Parse as URL to extract hostname and port correctly.
	u, err := url.Parse("//" + host) // the prefix `//` allows parsing without a scheme
	if err != nil {
		return "", fmt.Errorf("invalid host format: %w", err)
	}

	hostname := u.Hostname()
	port := u.Port()

	// If host is a domain name, resolve it to an IPv4 address.
	if net.ParseIP(hostname) == nil {
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

// findGoPro discovers a GoPro camera on the local network via mDNS.
func findGoPro() (net.IP, error) {
	conn, err := net.ListenMulticastUDP("udp4", nil, &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: 5353})
	if err != nil {
		return nil, fmt.Errorf("failed to create multicast connection: %w", err)
	}
	defer conn.Close()

	// Build and send the mDNS query.
	msg := new(dns.Msg)
	msg.SetQuestion("_gopro-web._tcp.local.", dns.TypePTR)
	msg.RecursionDesired = false

	buf, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack message: %w", err)
	}

	if _, err := conn.WriteToUDP(buf, &net.UDPAddr{IP: net.ParseIP("224.0.0.251"), Port: 5353}); err != nil {
		return nil, fmt.Errorf("failed to send query: %w", err)
	}

	// Wait for the response.
	response := make([]byte, 65536)
	conn.SetReadDeadline(time.Now().Add(4 * time.Second))

	for {
		n, _, err := conn.ReadFromUDP(response)
		if err != nil {
			if e, ok := err.(net.Error); ok && e.Timeout() {
				return nil, fmt.Errorf("timeout waiting for response")
			}
			return nil, fmt.Errorf("error reading response: %w", err)
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

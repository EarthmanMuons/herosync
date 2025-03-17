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
		return nil, fmt.Errorf("invalid scheme: %q; choose http or https", scheme)
	}

	// Resolve host to an IP address if needed.
	resolvedHost, err := resolveHost(host)
	if err != nil {
		return nil, fmt.Errorf("could not resolve GoPro address: %w", err)
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
	multicastAddr := "224.0.0.251:5353"

	// Use a standard UDP socket for sending.
	dst, err := net.ResolveUDPAddr("udp4", multicastAddr)
	if err != nil {
		return nil, fmt.Errorf("failed to resolve multicast address: %w", err)
	}

	conn, err := net.ListenPacket("udp4", ":0") // bind to an ephemeral port
	if err != nil {
		return nil, fmt.Errorf("failed to open UDP socket: %w", err)
	}
	defer conn.Close()

	// Build the mDNS query.
	msg := new(dns.Msg)
	msg.SetQuestion("_gopro-web._tcp.local.", dns.TypePTR)
	msg.RecursionDesired = false

	buf, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack message: %w", err)
	}

	// Set up a channel for the response.
	resultChan := make(chan net.IP, 1)
	doneChan := make(chan struct{})

	// Listen for responses.
	go func() {
		response := make([]byte, 65536)
		conn.SetReadDeadline(time.Now().Add(6 * time.Second))

		for {
			n, _, err := conn.ReadFrom(response)
			if err != nil {
				close(doneChan)
				return
			}

			resp := new(dns.Msg)
			if err := resp.Unpack(response[:n]); err != nil {
				continue
			}

			// Look for A records.
			for _, answer := range append(resp.Answer, resp.Extra...) {
				if a, ok := answer.(*dns.A); ok {
					resultChan <- a.A
					close(doneChan)
					return
				}
			}
		}
	}()

	// Send query and retry up to 3 times, but stop if a response is received.
	for range 3 {
		select {
		case ip := <-resultChan:
			return ip, nil
		case <-time.After(500 * time.Millisecond):
			if _, err := conn.WriteTo(buf, dst); err != nil {
				return nil, fmt.Errorf("failed to send query: %w", err)
			}
		case <-doneChan:
			break
		}
	}

	// Final check in case response came in right before timeout.
	select {
	case ip := <-resultChan:
		return ip, nil
	default:
		return nil, fmt.Errorf("no response received after retries")
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

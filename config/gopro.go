package config

import (
	"fmt"
	"net"
	"net/url"
	"time"

	"github.com/miekg/dns"
)

func resolveGoPro(host, scheme string) (*url.URL, error) {
	// Default to http if not specified
	if scheme == "" {
		scheme = "http"
	}
	if scheme != "http" && scheme != "https" {
		return nil, fmt.Errorf("invalid scheme: %s; choose http or https", scheme)
	}

	// Get the host IP address (either via discovery or resolution)
	resolvedHost, err := resolveHost(host)
	if err != nil {
		return nil, err
	}

	return &url.URL{
		Scheme: scheme,
		Host:   resolvedHost,
	}, nil
}

func resolveHost(host string) (string, error) {
	// Parse the host (to handle any port info)
	u, err := url.Parse("//" + host)
	if err != nil {
		return "", fmt.Errorf("invalid host format: %v", err)
	}

	// Extract hostname and port
	hostname := u.Hostname()
	port := u.Port()

	// If hostname is empty, use mDNS discovery
	if hostname == "" {
		ip, err := findGoPro()
		if err != nil {
			return "", fmt.Errorf("auto-discovery failed: %v", err)
		}
		hostname = ip.String()
	} else if net.ParseIP(hostname) == nil {
		// If hostname isn't an IP address, resolve it
		ips, err := net.LookupIP(hostname)
		if err != nil {
			return "", fmt.Errorf("failed to resolve hostname %s: %v", hostname, err)
		}
		// Use first IPv4 address
		var ip net.IP
		for _, addr := range ips {
			if v4 := addr.To4(); v4 != nil {
				ip = v4
				break
			}
		}
		if ip == nil {
			return "", fmt.Errorf("no IPv4 address found for %s", hostname)
		}
		hostname = ip.String()
	}

	// Reconstruct host with resolved IP and port if specified
	if port != "" {
		return net.JoinHostPort(hostname, port), nil
	}
	return hostname, nil
}

func findGoPro() (net.IP, error) {
	// Create UDP connection for multicast
	multicastConn, err := net.ListenMulticastUDP("udp4", nil,
		&net.UDPAddr{
			IP:   net.ParseIP("224.0.0.251"),
			Port: 5353,
		})
	if err != nil {
		return nil, fmt.Errorf("failed to create multicast connection: %v", err)
	}
	defer multicastConn.Close()

	// Create query message
	msg := new(dns.Msg)
	msg.SetQuestion("_gopro-web._tcp.local.", dns.TypePTR)
	msg.RecursionDesired = false

	// Send query
	buf, err := msg.Pack()
	if err != nil {
		return nil, fmt.Errorf("failed to pack message: %v", err)
	}

	_, err = multicastConn.WriteToUDP(buf, &net.UDPAddr{
		IP:   net.ParseIP("224.0.0.251"),
		Port: 5353,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to send query: %v", err)
	}

	// Listen for response
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

		// Parse response
		resp := new(dns.Msg)
		if err := resp.Unpack(response[:n]); err != nil {
			continue
		}

		// Look for A record in response
		for _, answer := range append(resp.Answer, resp.Extra...) {
			if a, ok := answer.(*dns.A); ok {
				return a.A, nil
			}
		}
	}
}

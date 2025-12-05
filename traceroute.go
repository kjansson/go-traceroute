package traceroute

import (
	"fmt"
	"net"
	"strings"
	"sync"
	"syscall"
	"time"

	"golang.org/x/net/icmp"
	"golang.org/x/net/ipv4"
)

const UnexpectedICMPType = -1 // Represents an unexpected ICMP type

// Tracer struct holds configuration and result channel for asynchronous use
type Tracer struct {
	Address    string        // Trace target address
	Port       int           // Destination port
	StartTTL   int           // Starting TTL value
	MaxTTL     int           // Maximum TTL value
	Timeout    time.Duration // Timeout for each hop
	DNSLookup  bool          // Enable DNS host lookup for hop addresses
	ResultChan chan Hop      // Channel to send hop results asynchronously
}

// Type hop represents a single hop in a traceroute
type Hop struct {
	TTL       int     // Time To Live value for this hop
	Address   string  // IP address of the hop
	Host      string  // Resolved hostname of the hop
	Latency   float64 // Latency in milliseconds to reach this hop
	Reachable bool    // Whether the hop was reachable based on ICMP
}

// TraceResult holds the hops collected during a trace
type TraceResult struct {
	Hops []Hop
}

// New creates a new tracer instance with default settings and initialized result channel
func New() *Tracer {

	return &Tracer{
		Port:       33434,
		StartTTL:   1,
		MaxTTL:     30,
		Timeout:    3 * time.Second,
		DNSLookup:  true,
		ResultChan: make(chan Hop, 1024),
	}
}

// Trace performs the traceroute operation and returns the collected hops both synchronously and via the ResultChan
func (t *Tracer) Trace() (TraceResult, error) {

	if t.StartTTL < 1 {
		return TraceResult{}, fmt.Errorf("value of StartTTL must be at least 1")
	}
	if t.Address == "" {
		return TraceResult{}, fmt.Errorf("value of Address must be specified")
	}
	if t.Port < 1 || t.Port > 65535 {
		return TraceResult{}, fmt.Errorf("value of Port must be between 1 and 65535")
	}

	ttl := t.StartTTL
	wg := sync.WaitGroup{}
	cancelChan := make(chan bool, 1)
	traceResult := TraceResult{}

	for {
		resolvedAddr, err := net.ResolveUDPAddr("udp", fmt.Sprintf("%s:%d", t.Address, t.Port))
		if err != nil {
			return traceResult, fmt.Errorf("resolving error: %w", err)
		}

		outgoing, err := net.DialUDP("udp", nil, resolvedAddr) // Create UDP connection
		if err != nil {
			return traceResult, fmt.Errorf("dial error: %w", err)
		}

		rawOutgoing, err := outgoing.SyscallConn() // Get raw connection to be able to set TTL
		if err != nil {
			return traceResult, fmt.Errorf("syscall connection error: %w", err)
		}
		err = rawOutgoing.Control(func(fd uintptr) {
			err := syscall.SetsockoptInt(int(fd), syscall.IPPROTO_IP, syscall.IP_TTL, ttl) // Set TTL
			if err != nil {
				fmt.Printf("error setting socket option: %v\n", err)
			}
		})
		if err != nil {
			return traceResult, fmt.Errorf("syscall connection error: %w", err)
		}

		defer func() { // Ensure connection is closed
			err = outgoing.Close()
			if err != nil {
				fmt.Printf("error closing connection: %v\n", err)
			}
		}()

		wg.Add(1)
		startTime := time.Now()
		go func() { // Listen asynchronously for ICMP response
			defer wg.Done()
			var host []string
			hopAddr, response, err := t.receiveICMP()
			latency := time.Since(startTime).Seconds() * 1000
			if t.DNSLookup {
				host, _ = net.LookupAddr(hopAddr)
			}
			if response != UnexpectedICMPType { // Record response even on errors as long as we got a valid ICMP type
				hop := Hop{
					TTL:       ttl,
					Address:   hopAddr,
					Latency:   latency,
					Host:      strings.Join(host, ", "),
					Reachable: response == ipv4.ICMPTypeTimeExceeded,
				}
				traceResult.Hops = append(traceResult.Hops, hop) // Store hop result
				t.ResultChan <- hop                              // Send hop result to result channel for asynchronous processing
			}
			if err != nil || response == ipv4.ICMPTypeDestinationUnreachable { // Stop tracing if we hit an error or an unreachable destination
				cancelChan <- true
			}
		}()

		_, err = outgoing.Write([]byte{}) // Send empty UDP packet
		if err != nil {
			return traceResult, fmt.Errorf("write error: %w", err)
		}

		wg.Wait()
		ttl++
		if ttl > t.MaxTTL { // Stop if we reached MaxTTL
			break
		}
		select {
		case <-cancelChan: // Tracing done or we hit an error
			return traceResult, nil
		default:
		}
	}

	return traceResult, nil
}

// receiveICMP listens for incoming ICMP packets and returns the address, relevant ICMP type, and any error encountered
func (t *Tracer) receiveICMP() (string, ipv4.ICMPType, error) {

	c, err := icmp.ListenPacket("udp4", "0.0.0.0") // Set up connection for incoming ICMP packets
	if err != nil {
		return "*", 0, fmt.Errorf("listen packet error: %w", err)
	}
	defer func() {
		err := c.Close()
		if err != nil {
			fmt.Printf("error closing ICMP connection: %v\n", err)
		}
	}()

	err = c.SetReadDeadline(time.Now().Add(t.Timeout))
	if err != nil {
		return "*", 0, fmt.Errorf("set read deadline error: %w", err)
	}

	rb := make([]byte, 1024)
	n, peer, err := c.ReadFrom(rb) // Read packet
	if err != nil {
		return "*", 0, fmt.Errorf("read from error: %w", err)
	}

	rawMessage, err := icmp.ParseMessage(1, rb[:n])
	if err != nil {
		return "*", 0, fmt.Errorf("parse message error: %w", err)
	}
	p := strings.Split(peer.String(), ":")
	address := p[0]

	// Inspect ICMP message type, we are only interested in TimeExceeded and DestinationUnreachable
	switch rawMessage.Type {
	case ipv4.ICMPTypeTimeExceeded:
		return address, ipv4.ICMPTypeTimeExceeded, nil // This is the response we want, packet expired along the way

	case ipv4.ICMPTypeDestinationUnreachable: // This means we cant trace further
		return address, ipv4.ICMPTypeDestinationUnreachable, nil
	default:
		return "*", UnexpectedICMPType, fmt.Errorf("unexpected ICMP message type received")
	}
}

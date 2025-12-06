# go-traceroute
A minimalistic and simplistic library for running UDP route tracing both synchronously and asynchronously.

## Features

* Simple to use
* Can be run either synchronously or asynchronously for integration to real-time applications
* Does not rely on raw sockets so no elevated permissions are needed (runs as non-root)

## Usage

### Synchronous

```
package main

import (
	"fmt"
	tracer "github.com/kjansson/go-traceroute"
)

func main() {

	t := tracer.New()
	t.Address = "8.8.8.8"

	result, err := t.Trace()
	if err != nil {
		panic(err)
	}

	// Synchronous output of hops after trace is complete

	fmt.Println("Trace completed. Hops:")

	for _, hop := range result.Hops {
		fmt.Printf("Hop: %d, %s, Host: %s, Latency: %.2f ms\n", hop.TTL, hop.Address, hop.Host, hop.Latency)
	}
}
``` 

### Aynchronous 

```
package main

import (
	"fmt"
	"os"
	"os/signal"
	"syscall"

	tracer "github.com/kjansson/go-traceroute"
)

func main() {

	t := tracer.New()
	t.Address = "8.8.8.8"


	cancelChan := make(chan os.Signal, 1)
	signal.Notify(cancelChan, syscall.SIGTERM, syscall.SIGINT)

	go t.Trace() // Start trace in a go-routine
	for {
		select {
        // Collect realtime output
		case hop := <-tracer.ResultChan:
			fmt.Printf("Hop: %d, %s, Host: %s, Latency: %.2f ms, Reachable: %t\n", hop.TTL, hop.Address, hop.Host, hop.Latency, hop.Reachable)
			if !hop.Reachable {
				fmt.Println("Trace completed.")
				os.Exit(0)
			}
		case <-cancelChan:
			fmt.Println("Trace cancelled.")
			os.Exit(0)
		}
	}
}
```

## Configuration

The Tracer struct returned by the constructor New can be configured prior to executing a trace. The only required field for running a trace is the address of the target host.  
The following fields are configurable;

```
type Tracer struct {
	Address    string        // Trace target address or hostname, required
	Port       int           // Destination port, defaults to random port
	StartTTL   int           // Starting TTL value, default is 1
	MaxTTL     int           // Maximum TTL value, default is 30
	Timeout    time.Duration // Timeout for each hop, default is 3s 
	DNSLookup  bool          // Enable DNS host lookup for hop addresses, default is enabled
}
```

## Trace result

The Traceresult struct contains the trace result in the form of an ordered array of hops;  

```
type Hop struct {
	TTL       int     // Time To Live value for this hop
	Address   string  // IP address of the hop
	Host      string  // Resolved hostname of the hop
	Latency   float64 // Latency in milliseconds to reach this hop
	Reachable bool    // Whether the hop was reachable based on ICMP
}
```

Trace result can be read in two ways;  
- Reading theTraceResult struct returned after trace execution.
- Reading the ResultChan channel which continously publishes completed hops during execution.
# go-traceroute
A minimalistic library for running UDP route tracing both synchronously and asynchronously.

##
go-traceroute provides a simple way to perform UDP tracing in Go.  
It aims to be simple to use with minimal configuration. IT can be used either synchronously, i.e. call a trace and wait for the final result, or asynchronously by starting a trace in a goroutine and collect the results for integration in realtime applications.

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

	// Synchronous output of hops after trace is complete

	result, err := tracer.Trace()
	if err != nil {
		panic(err)
	}

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

    // Collect realtime output

	cancelChan := make(chan os.Signal, 1)
	signal.Notify(cancelChan, syscall.SIGTERM, syscall.SIGINT)

	go t.Trace()
	for {
		select {
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

The tracer struct returned by the constructor New can be configured prior to executing a trace. The following fields are configurable;

```
type Tracer struct {
	Address    string        // Trace target address
	Port       int           // Destination port
	StartTTL   int           // Starting TTL value
	MaxTTL     int           // Maximum TTL value
	Timeout    time.Duration // Timeout for each hop
	DNSLookup  bool          // Enable DNS host lookup for hop addresses
}
```
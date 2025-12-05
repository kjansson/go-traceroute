package traceroute

import (
	"testing"
	"time"
)

func TestTraceroute(t *testing.T) {
	tracer := New()
	tracer.Address = "localhost"
	tracer.MaxTTL = 5
	tracer.Timeout = 1 * time.Second
	tracer.DNSLookup = false

	result, err := tracer.Trace()
	if err != nil {
		t.Fatalf("Trace failed: %v", err)
	}

	if len(result.Hops) == 0 {
		t.Fatalf("Expected at least one hop, got zero")
	}
}

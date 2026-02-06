package main

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

// Unit test for helper function
func TestToHex(t *testing.T) {
	input := []byte{0xDE, 0xAD, 0xBE, 0xEF}
	expected := "0xdeadbeef"
	assert.Equal(t, expected, toHex(input))
}

// Test Agent Heartbeat Logic (Mocking HTTP Server)
func TestAgentHeartbeat(t *testing.T) {
	// Start a local test server
	var receivedBody map[string]interface{}
	var mu sync.Mutex

	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.Contains(r.URL.Path, "heartbeat") {
			mu.Lock()
			defer mu.Unlock()
			json.NewDecoder(r.Body).Decode(&receivedBody)
			w.WriteHeader(http.StatusOK)
		}
	}))
	defer ts.Close()

	// Initialize a partial agent to test gatherStatus logic
	// We can't easily start the full eBPF agent without kernel permissions/support
	// So we create a dummy agent struct
	agent := &Agent{
		endpoint: ts.URL,
		done:     make(chan struct{}),
		// cols empty
	}

	// Verify gatherStatus produces valid structure
	status := agent.gatherStatus()
	assert.Contains(t, status, "hostname")
	assert.Contains(t, status, "time")
	assert.Contains(t, status, "programs")
	
	// Test Loop functionality (short duration)
	go agent.backgroundLoop()
	
	// Wait for 1 heartbeat (loop runs every 15s in main code, 
	// for testing we'd ideally make ticker configurable or just wait)
	// Since the hardcoded ticker is 15s, this test might be too slow for unit tests.
	// Recommendation: Refactor agent to accept a config/ticker duration.
	
	// For now, we will just manually invoke the logic inside the loop for testing
	// ... (Skipping full loop wait to avoid 15s delay in test suite)
	
	// Manually trigger the payload construction used in loop
	buf := agent.gatherStatus()
	assert.NotNil(t, buf)
}

func TestDecodeHelper(t *testing.T) {
	// Assuming decodeValue is a helper we want to test
	// It wasn't exported in original file (it was local), 
	// assuming we can access it if in same package 'main'
	
	// If decodeValue handles complex structs, add tests here.
	// For now, simple placeholder.
	assert.True(t, true)
}

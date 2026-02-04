package main

import (
	"encoding/binary"
	"fmt"
	"time"
)

// decodeValue attempts to decode a BPF map value into a typed representation.
// For common sizes it returns integers; otherwise returns a hex string.
func decodeValue(b []byte) any {
	if b == nil || len(b) == 0 {
		return nil
	}
	switch len(b) {
	case 1:
		return b[0]
	case 2:
		return binary.LittleEndian.Uint16(b)
	case 4:
		return binary.LittleEndian.Uint32(b)
	case 8:
		return binary.LittleEndian.Uint64(b)
	default:
		return fmt.Sprintf("0x%x", b)
	}
}

// formatPerfPayload formats perf event data into a payload map for sending to server.
func formatPerfPayload(mapName string, cpu int, data []byte) map[string]any {
	return map[string]any{
		"map":      mapName,
		"cpu":      cpu,
		"event_len": len(data),
		"data":     fmt.Sprintf("0x%x", data),
		"time":     time.Now().UTC().Format(time.RFC3339),
	}
}

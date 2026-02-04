package main

import (
	"encoding/base64"
	"testing"
	"time"

	"github.com/cilium/ebpf/perf"
)

func TestMakePayload(t *testing.T) {
	s := []byte("hello-world")
	rec := perf.Record{RawSample: s}
	p := makePayload("mymap", rec, false)
	if p.Map != "mymap" {
		t.Fatalf("map mismatch: %s", p.Map)
	}
	if p.DataBase64 != base64.StdEncoding.EncodeToString(s) {
		t.Fatalf("data_base64 mismatch: %s", p.DataBase64)
	}
	if p.Data != "" {
		t.Fatalf("data should be empty when not sending hex: %s", p.Data)
	}
	if time.Since(time.Unix(0, p.Timestamp)) > 5*time.Second {
		t.Fatalf("timestamp looks wrong: %d", p.Timestamp)
	}

	// hex payload
	p2 := makePayload("mymap", rec, true)
	if p2.Data == "" || p2.Data[:2] != "0x" {
		t.Fatalf("expected hex data prefixed with 0x, got: %s", p2.Data)
	}
}

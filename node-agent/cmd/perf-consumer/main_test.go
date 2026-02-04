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
	p := makePayload("mymap", rec)
	if p.Map != "mymap" {
		t.Fatalf("map mismatch: %s", p.Map)
	}
	if p.Data != base64.StdEncoding.EncodeToString(s) {
		t.Fatalf("data mismatch: %s", p.Data)
	}
	if time.Since(time.Unix(0, p.Timestamp)) > 5*time.Second {
		t.Fatalf("timestamp looks wrong: %d", p.Timestamp)
	}
}

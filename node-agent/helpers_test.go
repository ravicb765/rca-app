package main

import (
	"reflect"
	"testing"
)

func TestDecodeValue(t *testing.T) {
	cases := []struct {
		in  []byte
		exp any
	}{
		{[]byte{0x01}, byte(1)},
		{[]byte{0x02, 0x00}, uint16(2)},
		{[]byte{0x03, 0x00, 0x00, 0x00}, uint32(3)},
		{[]byte{0x04, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00}, uint64(4)},
		{[]byte{0xde, 0xad, 0xbe, 0xef}, "0xdeadbeef"},
	}

	for _, c := range cases {
		got := decodeValue(c.in)
		if !reflect.DeepEqual(got, c.exp) {
			t.Fatalf("decodeValue(%v) = %v, want %v", c.in, got, c.exp)
		}
	}
}

func TestFormatPerfPayload(t *testing.T) {
	p := formatPerfPayload("mymap", 2, []byte{0xde, 0xad})
	if p["map"] != "mymap" {
		t.Fatalf("unexpected map: %v", p["map"])
	}
	if p["cpu"] != 2 {
		t.Fatalf("unexpected cpu: %v", p["cpu"])
	}
	if p["event_len"] != 2 {
		t.Fatalf("unexpected len: %v", p["event_len"])
	}
	if p["data"] != "0xdead" {
		t.Fatalf("unexpected data: %v", p["data"])
	}
}

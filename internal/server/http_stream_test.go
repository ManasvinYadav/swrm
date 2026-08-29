package server

import (
	"context"
	"testing"
)

func TestParseRange(t *testing.T) {
	a, b, e := parseRange("bytes=10-19", 100)
	if e != nil || a != 10 || b != 19 {
		t.Fatalf("%d %d %v", a, b, e)
	}
	a, b, e = parseRange("bytes=-10", 100)
	if e != nil || a != 90 || b != 99 {
		t.Fatalf("%d %d %v", a, b, e)
	}
	if _, _, e = parseRange("bytes=100-", 100); e == nil {
		t.Fatal("expected invalid range")
	}
	if _, _, e = parseRange("", 0); e == nil {
		t.Fatal("expected empty resource to reject range")
	}
}

func TestStreamServerCannotStartTwice(t *testing.T) {
	s := NewStreamServer(nil, 0)
	if err := s.Start(); err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	if err := s.Start(); err == nil {
		t.Fatal("second Start should fail")
	}
}

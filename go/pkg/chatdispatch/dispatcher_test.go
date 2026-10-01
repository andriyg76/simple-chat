package chatdispatch

import (
	"context"
	"testing"
	"time"
)

func TestDispatcherCombinesDebouncedMessages(t *testing.T) {
	d := New(time.Millisecond)
	got := make(chan string, 1)
	h := func(_ context.Context, body string) { got <- body }
	if id := d.Submit("chat", "1", "one", h); id != "1" {
		t.Fatalf("first batch ID = %q", id)
	}
	if id := d.Submit("chat", "2", "two", h); id != "1" {
		t.Fatalf("combined batch ID = %q", id)
	}
	select {
	case body := <-got:
		if body != "one\ntwo" {
			t.Fatalf("body = %q", body)
		}
	case <-time.After(time.Second):
		t.Fatal("dispatcher did not flush")
	}
}

func TestDispatcherBuffersWhileBusy(t *testing.T) {
	d := New(time.Millisecond)
	started := make(chan struct{})
	release := make(chan struct{})
	got := make(chan string, 2)
	d.Submit("chat", "1", "first", func(_ context.Context, body string) {
		got <- body
		close(started)
		<-release
	})
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("first turn did not start")
	}
	h := func(_ context.Context, body string) { got <- body }
	d.Submit("chat", "2", "second", h)
	d.Submit("chat", "3", "third", h)
	close(release)
	if body := <-got; body != "first" {
		t.Fatalf("first body = %q", body)
	}
	select {
	case body := <-got:
		if body != "second\nthird" {
			t.Fatalf("buffered body = %q", body)
		}
	case <-time.After(time.Second):
		t.Fatal("buffered turn did not run")
	}
}

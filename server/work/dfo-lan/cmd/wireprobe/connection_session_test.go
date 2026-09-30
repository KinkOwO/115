package main

import "testing"

func TestConnectionSessionOwnsLifecycle(t *testing.T) {
	plain := newConnectionSession(false)
	if plain.moonTicks() != nil {
		t.Fatal("disabled moon timer must not create a channel")
	}
	select {
	case <-plain.done:
		t.Fatal("connection starts open")
	default:
	}
	plain.close()
	plain.close()
	select {
	case <-plain.done:
	default:
		t.Fatal("closing a connection must stop its reader")
	}

	moon := newConnectionSession(true)
	if moon.moonTicks() == nil {
		t.Fatal("enabled moon timer must expose a channel")
	}
	moon.close()
}

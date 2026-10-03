package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestConnectionSessionOwnsLifecycle(t *testing.T) {
	plain := newConnectionSession(false)
	t.Cleanup(plain.close)
	require.Nil(t, plain.moonTicks(), "disabled moon timer must not create a channel")
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
	t.Cleanup(moon.close)
	require.NotNil(t, moon.moonTicks(), "enabled moon timer must expose a channel")
	moon.close()
}

package main

import "testing"

func TestEquipmentRequestKeyKeepsRetryIdentity(t *testing.T) {
	s := equipmentSession{initialized: true, nonce: [16]byte{1, 2, 3}}
	key, err := s.requestKey([]byte("abc"))
	const want = "01020300000000000000000000000000:ba7816bf8f01cfea414140de5dae2223b00361a396177a9cb410ff61f20015ad"
	if err != nil || key != want {
		t.Fatalf("key=%q err=%v", key, err)
	}
	var session equipmentSession
	first, err := session.requestKey([]byte("abc"))
	if err != nil {
		t.Fatal(err)
	}
	again, err := session.requestKey([]byte("abc"))
	if err != nil || again != first {
		t.Fatalf("retry identity changed: %q / %q (%v)", first, again, err)
	}
	changed, err := session.requestKey([]byte("abd"))
	if err != nil || changed == first {
		t.Fatalf("different request reused key: %q (%v)", changed, err)
	}
	var other equipmentSession
	separate, err := other.requestKey([]byte("abc"))
	if err != nil || separate == first {
		t.Fatalf("different session reused key: %q (%v)", separate, err)
	}
}

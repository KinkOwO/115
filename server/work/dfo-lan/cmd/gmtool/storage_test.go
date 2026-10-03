package main

import (
	"encoding/json"
	"net"
	"testing"
)

func TestStartStorageWithPostgresOnly(t *testing.T) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	for _, legacy := range []string{"", `,"redis_address":"invalid","redis_bin":"missing","redis_password":"obsolete"`} {
		var cfg storageConfig
		input := `{"postgres_dsn":"postgres://user:pass@` + listener.Addr().String() + `/dfo_lan"` + legacy + `}`
		if err := json.Unmarshal([]byte(input), &cfg); err != nil {
			t.Fatal(err)
		}
		// No binaries or storage directories exist; an available PostgreSQL port is sufficient.
		note, err := startStorage(cfg, t.TempDir())
		if err != nil {
			t.Fatalf("startStorage: %v", err)
		}
		if note != "" {
			t.Fatalf("unexpected startup: %q", note)
		}
	}
}

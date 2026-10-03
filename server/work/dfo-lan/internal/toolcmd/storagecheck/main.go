package storagecheck

import (
	"context"
	"dfolan/internal/storage"
	"flag"
	"log"
	"time"
)

func Run() {
	path := flag.String("config", "runtime/storage/local.json", "storage config")
	flag.Parse()
	c, e := storage.LoadConfig(*path)
	if e != nil {
		log.Fatal(e)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	s, e := storage.Open(ctx, c)
	if e != nil {
		log.Fatal(e)
	}
	defer s.Close()
	if e = s.Migrate(ctx); e != nil {
		log.Fatal(e)
	}
	id, e := s.DevelopmentAccount(ctx, "probe")
	if e != nil {
		log.Fatal(e)
	}
	chars, e := s.Characters(ctx, id)
	if e != nil {
		log.Fatal(e)
	}
	log.Printf("POSTGRES_READY development_account=%d characters=%d", id, len(chars))
	for _, role := range chars {
		log.Printf("CHARACTER id=%d name=%s profession=%d config_version=%s", role.WireID, role.Name, role.Profession, role.ConfigVersion)
	}
}

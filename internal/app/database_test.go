package app

import (
	"testing"
)

func TestConfigureDbUsesURI(t *testing.T) {
	pool, err := configureDb("postgres://test:password@localhost:6543/config_test?sslmode=disable")
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	cfg := pool.Config().ConnConfig
	if cfg.Host != "localhost" || cfg.Port != 6543 || cfg.Database != "config_test" || cfg.User != "test" {
		t.Fatalf("pool ignored connection URI: host=%s port=%d database=%s user=%s", cfg.Host, cfg.Port, cfg.Database, cfg.User)
	}
}

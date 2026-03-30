package config

import "flag"

// Config holds CLI-only options. Runtime settings are persisted in DB.
type Config struct {
	DBPath     string
	ListenAddr string
}

func Parse() *Config {
	cfg := &Config{}
	flag.StringVar(&cfg.DBPath, "db", "bangou.db", "SQLite database path")
	flag.StringVar(&cfg.ListenAddr, "listen", ":8080", "HTTP listen address")
	flag.Parse()
	return cfg
}

package config

import (
	"flag"
	"os"
)

// Config содержит адреса HTTP-сервера, PostgreSQL и сервиса начислений.
type Config struct {
	RunAddress     string
	DatabaseURI    string
	AccrualAddress string
}

// Parse читает RUN_ADDRESS, DATABASE_URI и ACCRUAL_SYSTEM_ADDRESS и флаги -a, -d, -r.
// Флаги имеют приоритет; ошибки и flag.ErrHelp возвращаются после вывода сообщения в stderr.
func Parse(args []string) (Config, error) {
	cfg := Config{
		RunAddress:     os.Getenv("RUN_ADDRESS"),
		DatabaseURI:    os.Getenv("DATABASE_URI"),
		AccrualAddress: os.Getenv("ACCRUAL_SYSTEM_ADDRESS"),
	}
	if cfg.RunAddress == "" {
		cfg.RunAddress = ":8080"
	}
	if cfg.DatabaseURI == "" {
		cfg.DatabaseURI = "postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable"
	}

	flags := flag.NewFlagSet("gofermart", flag.ContinueOnError)
	flags.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "Адрес и порт HTTP-сервера")
	flags.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "Строка подключения к PostgreSQL")
	flags.StringVar(&cfg.AccrualAddress, "r", cfg.AccrualAddress, "Адрес сервиса начисления")
	err := flags.Parse(args)
	return cfg, err
}

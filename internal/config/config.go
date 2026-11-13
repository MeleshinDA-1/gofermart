package config

import (
	"errors"
	"flag"
	"fmt"
	"os"
)

// Config содержит конфигурацию сервиса.
type Config struct {
	RunAddress     string
	DatabaseURI    string
	AccrualAddress string
	JWTSecret      string
}

// Parse читает переменные окружения и флаги -a, -d, -r.
// Флаги имеют приоритет; ошибки и flag.ErrHelp возвращаются после вывода сообщения в stderr.
func Parse(args []string) (Config, error) {
	cfg := Config{
		RunAddress: ":8080",
		JWTSecret:  os.Getenv("JWT_SECRET"),
	}
	if value, ok := os.LookupEnv("RUN_ADDRESS"); ok {
		cfg.RunAddress = value
	}
	if value, ok := os.LookupEnv("DATABASE_URI"); ok {
		cfg.DatabaseURI = value
	}
	if value, ok := os.LookupEnv("ACCRUAL_SYSTEM_ADDRESS"); ok {
		cfg.AccrualAddress = value
	}

	flags := flag.NewFlagSet("gofermart", flag.ContinueOnError)
	flags.StringVar(&cfg.RunAddress, "a", cfg.RunAddress, "Адрес и порт HTTP-сервера")
	flags.StringVar(&cfg.DatabaseURI, "d", cfg.DatabaseURI, "Строка подключения к PostgreSQL")
	flags.StringVar(&cfg.AccrualAddress, "r", cfg.AccrualAddress, "Адрес сервиса начисления")
	if err := flags.Parse(args); err != nil {
		return cfg, err
	}
	if cfg.DatabaseURI == "" {
		err := errors.New("задайте DATABASE_URI или флаг -d")
		fmt.Fprintln(flags.Output(), err)
		return cfg, err
	}
	if len(cfg.JWTSecret) < 32 {
		err := errors.New("JWT_SECRET должен содержать минимум 32 байта")
		fmt.Fprintln(flags.Output(), err)
		return cfg, err
	}
	return cfg, nil
}

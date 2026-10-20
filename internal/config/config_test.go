package config

import (
	"errors"
	"flag"
	"testing"
)

func TestParseConfig(t *testing.T) {
	defaults := Config{
		RunAddress:  ":8080",
		DatabaseURI: "postgres://gofermart:password@localhost:5434/gofermart?sslmode=disable",
	}
	env := Config{
		RunAddress:     "127.0.0.1:9090",
		DatabaseURI:    "postgres://test:env-password@localhost:5435/envdb",
		AccrualAddress: "http://localhost:9091",
	}
	fromFlags := Config{
		RunAddress:     "127.0.0.1:9190",
		DatabaseURI:    "postgres://test:flag-password@localhost:5436/flagdb",
		AccrualAddress: "http://localhost:9191",
	}
	args := []string{"-a", fromFlags.RunAddress, "-d", fromFlags.DatabaseURI, "-r", fromFlags.AccrualAddress}
	for _, test := range []struct {
		name string
		env  Config
		args []string
		want Config
	}{
		{name: "defaults", want: defaults},
		{name: "environment", env: env, want: env},
		{name: "flags", args: args, want: fromFlags},
		{name: "flags override environment", env: env, args: args, want: fromFlags},
		{name: "one flag keeps other environment values", env: env, args: []string{"-a", fromFlags.RunAddress},
			want: Config{RunAddress: fromFlags.RunAddress, DatabaseURI: env.DatabaseURI, AccrualAddress: env.AccrualAddress}},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("RUN_ADDRESS", test.env.RunAddress)
			t.Setenv("DATABASE_URI", test.env.DatabaseURI)
			t.Setenv("ACCRUAL_SYSTEM_ADDRESS", test.env.AccrualAddress)
			got, err := Parse(test.args)
			if err != nil {
				t.Fatal(err)
			}
			if got != test.want {
				t.Fatalf("Config = %+v, want %+v", got, test.want)
			}
		})
	}
}

func TestParseConfigErrors(t *testing.T) {
	t.Setenv("RUN_ADDRESS", "")
	t.Setenv("DATABASE_URI", "")
	t.Setenv("ACCRUAL_SYSTEM_ADDRESS", "")
	for _, args := range [][]string{{"-unknown"}, {"-a"}, {"-d"}} {
		if _, err := Parse(args); err == nil {
			t.Fatalf("expected flag error for %v", args)
		}
	}
	if _, err := Parse([]string{"-h"}); !errors.Is(err, flag.ErrHelp) {
		t.Fatalf("help error = %v, want flag.ErrHelp", err)
	}
}

package config

import (
	"errors"
	"flag"
	"os"
	"strings"
	"testing"
)

func TestParseConfig(t *testing.T) {
	const secret = "0123456789abcdef0123456789abcdef"
	t.Setenv("JWT_SECRET", secret)
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
		name    string
		env     *Config
		args    []string
		want    Config
		wantErr string
	}{
		{name: "missing database URI", wantErr: "DATABASE_URI"},
		{name: "default run address", args: []string{"-d", fromFlags.DatabaseURI},
			want: Config{RunAddress: ":8080", DatabaseURI: fromFlags.DatabaseURI}},
		{name: "environment", env: &env, want: env},
		{name: "empty environment", env: &Config{}, wantErr: "DATABASE_URI"},
		{name: "empty optional environment", env: &Config{DatabaseURI: env.DatabaseURI}, want: Config{DatabaseURI: env.DatabaseURI}},
		{name: "flags", args: args, want: fromFlags},
		{name: "flags override environment", env: &env, args: args, want: fromFlags},
		{name: "flags override empty environment", env: &Config{}, args: args, want: fromFlags},
		{name: "empty database flag overrides environment", env: &env, args: []string{"-d", ""}, wantErr: "DATABASE_URI"},
		{name: "empty optional flags override environment", env: &env, args: []string{"-a", "", "-r", ""}, want: Config{DatabaseURI: env.DatabaseURI}},
		{name: "one flag keeps other environment values", env: &env, args: []string{"-a", fromFlags.RunAddress},
			want: Config{RunAddress: fromFlags.RunAddress, DatabaseURI: env.DatabaseURI, AccrualAddress: env.AccrualAddress}},
	} {
		t.Run(test.name, func(t *testing.T) {
			test.want.JWTSecret = secret
			for _, name := range []string{"RUN_ADDRESS", "DATABASE_URI", "ACCRUAL_SYSTEM_ADDRESS"} {
				t.Setenv(name, "")
				if err := os.Unsetenv(name); err != nil {
					t.Fatal(err)
				}
			}
			if test.env != nil {
				t.Setenv("RUN_ADDRESS", test.env.RunAddress)
				t.Setenv("DATABASE_URI", test.env.DatabaseURI)
				t.Setenv("ACCRUAL_SYSTEM_ADDRESS", test.env.AccrualAddress)
			}
			got, err := Parse(test.args)
			if test.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), test.wantErr) {
					t.Fatalf("Parse error = %v, want error mentioning %s", err, test.wantErr)
				}
				return
			}
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
	t.Setenv("JWT_SECRET", "")
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

func TestParseJWTSecret(t *testing.T) {
	t.Setenv("DATABASE_URI", "postgres://localhost/testdb")
	for _, test := range []struct {
		name    string
		secret  string
		wantErr bool
	}{
		{name: "missing", wantErr: true},
		{name: "too short", secret: strings.Repeat("x", 31), wantErr: true},
		{name: "minimum", secret: strings.Repeat("x", 32)},
		{name: "longer", secret: strings.Repeat("x", 64)},
		{name: "length in bytes", secret: strings.Repeat("я", 16)},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Setenv("JWT_SECRET", test.secret)
			cfg, err := Parse(nil)
			if (err != nil) != test.wantErr {
				t.Fatalf("Parse error = %v, want error = %v", err, test.wantErr)
			}
			if test.wantErr {
				if !strings.Contains(err.Error(), "JWT_SECRET") {
					t.Fatalf("error must identify JWT_SECRET: %v", err)
				}
				return
			}
			t.Setenv("JWT_SECRET", "changed-after-startup")
			if cfg.JWTSecret != test.secret {
				t.Fatal("configuration did not retain the startup secret")
			}
		})
	}
}

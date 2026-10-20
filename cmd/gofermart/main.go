// Команда gofermart запускает HTTP API системы лояльности.
// Запуск из корня проекта: go run ./cmd/gofermart.
package main

import (
	"errors"
	"flag"
	"log"
	"os"

	"gofermart/internal/app"
	"gofermart/internal/config"
)

func main() {
	cfg, err := config.Parse(os.Args[1:])
	if err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			os.Exit(2)
		}
		return
	}
	if err := app.Run(cfg); err != nil {
		log.Print(err)
	}
}

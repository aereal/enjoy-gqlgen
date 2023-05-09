package main

import (
	"fmt"
	"os"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(2)
	}
}

func run() error {
	cfg, err := config.LoadConfigFromDefaultLocations()
	if err != nil {
		return fmt.Errorf("config.LoadConfigFromDefaultLocations: %w", err)
	}
	var opts []api.Option
	if err := api.Generate(cfg, opts...); err != nil {
		return fmt.Errorf("api.Generate: %w", err)
	}
	return nil
}

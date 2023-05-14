package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/aereal/enjoy-gqlgen/tracing"
	"go.opentelemetry.io/otel"
)

func main() {
	if err := run(); err != nil {
		fmt.Fprintf(os.Stderr, "%+v\n", err)
		os.Exit(2)
	}
}

var (
	timeout time.Duration
)

func init() {
	flag.DurationVar(&timeout, "timeout", time.Second*10, "timeout duration")
}

func run() error {
	flag.Parse()
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	shutdown, err := tracing.Setup(ctx)
	if err != nil {
		return fmt.Errorf("tracing.Setup(): %w", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), timeout)
		defer cancel()
		shutdown(ctx)
	}()

	tracer := otel.GetTracerProvider().Tracer("tools/codegen")
	ctx, span := tracer.Start(ctx, "main")
	defer func() { tracing.FinishSpan(span, nil) }()

	var cfg *config.Config
	{
		var err error
		_, span := tracer.Start(ctx, "config.Load")
		cfg, err = config.LoadConfigFromDefaultLocations()
		tracing.FinishSpan(span, err)
		if err != nil {
			return fmt.Errorf("config.LoadConfigFromDefaultLocations: %w", err)
		}
	}
	{
		_, span := tracer.Start(ctx, "api.Generate")
		var opts []api.Option
		err := api.Generate(cfg, opts...)
		tracing.FinishSpan(span, err)
		if err != nil {
			return fmt.Errorf("api.Generate: %w", err)
		}
	}
	return nil
}

package main

import (
	"context"
	_ "embed"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/99designs/gqlgen/api"
	"github.com/99designs/gqlgen/codegen/config"
	"github.com/99designs/gqlgen/codegen/templates"
	"github.com/99designs/gqlgen/plugin"
	"github.com/99designs/gqlgen/plugin/modelgen"
	"github.com/aereal/enjoy-gqlgen/tracing"
	"github.com/vektah/gqlparser/v2/ast"
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
		opts = append(opts, api.ReplacePlugin(newCustomEnumGeneratorPlugin(modelgen.New().(*modelgen.Plugin))))
		err := api.Generate(cfg, opts...)
		tracing.FinishSpan(span, err)
		if err != nil {
			return fmt.Errorf("api.Generate: %w", err)
		}
	}
	return nil
}

//go:embed enums.go.gotpl
var enumsTemplate string

func newCustomEnumGeneratorPlugin(mg *modelgen.Plugin) *customEnumGenerator {
	return &customEnumGenerator{mg: mg}
}

type customEnumGenerator struct {
	mg *modelgen.Plugin
}

var _ interface {
	plugin.Plugin
	plugin.ConfigMutator
} = (*customEnumGenerator)(nil)

func (p *customEnumGenerator) Name() string { return p.mg.Name() }

func (p *customEnumGenerator) MutateConfig(cfg *config.Config) error {
	data := struct{ Enums []*enum }{}
	for _, schemaType := range cfg.Schema.Types {
		if schemaType.Kind != ast.Enum || cfg.Models.UserDefined(schemaType.Name) {
			continue
		}
		x := &enum{Name: schemaType.Name, Description: schemaType.Description}
		for _, v := range schemaType.EnumValues {
			x.Values = append(x.Values, &enumValue{Name: v.Name, Description: v.Description})
		}
		data.Enums = append(data.Enums, x)

		// hide enums from original modelgen plugin
		cfg.Models.Add(schemaType.Name, cfg.Model.ImportPath()+"."+templates.ToGo(schemaType.Name))
	}

	opts := templates.Options{
		PackageName:     cfg.Model.Package,
		Filename:        filepath.Join(filepath.Dir(cfg.Model.Filename), "enums_gen.go"),
		Data:            data,
		GeneratedHeader: true,
		Packages:        cfg.Packages,
		Template:        enumsTemplate,
	}
	if err := templates.Render(opts); err != nil {
		return fmt.Errorf("templates.Render: %w", err)
	}
	cfg.ReloadAllPackages()

	if err := p.mg.MutateConfig(cfg); err != nil {
		return err
	}
	return nil
}

type enum struct {
	Name        string
	Description string
	Values      []*enumValue
}

type enumValue struct {
	Name        string
	Description string
}

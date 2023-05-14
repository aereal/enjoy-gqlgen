package tracing

import (
	"context"
	"errors"
	"net/url"
	"path"
	"reflect"
	"time"

	"github.com/hashicorp/go-multierror"
	"go.opentelemetry.io/contrib/propagators/b3"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.18.0"
	"go.opentelemetry.io/otel/trace"
)

func FinishSpan(span trace.Span, err error) {
	code := codes.Ok
	var desc string
	if err != nil {
		code = codes.Error
		desc = err.Error()
		var merr *multierror.Error
		if errors.As(err, &merr) && merr != nil {
			occurredAt := time.Now()
			for _, e := range merr.Errors {
				span.RecordError(e, trace.WithTimestamp(occurredAt))
			}
		} else {
			span.RecordError(err)
		}
	}
	span.SetStatus(code, desc)
	span.End()
}

func Setup(ctx context.Context) (func(context.Context), error) {
	opts := []sdktrace.TracerProviderOption{sdktrace.WithSampler(sdktrace.AlwaysSample())}
	var merr *multierror.Error
	{
		exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithInsecure())
		if err != nil {
			merr = multierror.Append(merr, err)
		} else {
			opts = append(opts, sdktrace.WithBatcher(exporter, sdktrace.WithMaxQueueSize(3)))
		}
	}
	{
		res, err := prepareResource(ctx)
		if err != nil {
			merr = multierror.Append(merr, err)
		} else {
			opts = append(opts, sdktrace.WithResource(res))
		}
	}
	if err := merr.ErrorOrNil(); err != nil {
		return nil, err
	}
	tp := sdktrace.NewTracerProvider(opts...)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(b3.New())
	shutdown := func(ctx context.Context) {
		_ = tp.Shutdown(ctx)
	}
	return shutdown, nil
}

var (
	serviceName    string
	serviceVersion = "latest"
	env            = "current"
)

func prepareResource(ctx context.Context) (*resource.Resource, error) {
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceVersion(serviceVersion),
			semconv.ServiceName(serviceName),
			semconv.DeploymentEnvironment(env),
		),
		resource.WithSchemaURL(semconv.SchemaURL))
	if err != nil {
		return nil, err
	}
	return res, nil
}

type aux struct{}

func init() {
	parsed, err := url.Parse(reflect.TypeOf(aux{}).PkgPath())
	if err != nil {
		panic(err)
	}
	up := path.Dir(parsed.Path)
	parsed.Path = up
	serviceName = parsed.String()
}

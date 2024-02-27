package tracing

import (
	"context"
	"io"
	"log"
	"log/slog"
	"os"
	"strings"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
)

var Tracer = otel.Tracer("dagger")

func Init() io.Closer {
	var tracingEnabled bool
	for _, env := range os.Environ() {
		if strings.HasPrefix(env, "OTEL_") {
			tracingEnabled = true
			break
		}
	}

	if !tracingEnabled {
		log.Println("!!! TRACING NOT ENABLED")
		return &nopCloser{}
	}

	log.Println("!!! TRACING INDEED ENABLED")
	slog.Debug("setting up tracing")

	exp, err := otlptracehttp.New(context.TODO())
	if err != nil {
		panic(err)
	}

	tp := tracesdk.NewTracerProvider(
		// Always be sure to batch in production.
		tracesdk.WithBatcher(exp,
			// The engine doesn't actually process enough traffic for it to be worth
			// batching; better to set it to 1 for instant dev feedback.
			tracesdk.WithMaxExportBatchSize(1)),
		// Record information about this application in an Resource.
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceNameKey.String("dagger"),
		)),
		tracesdk.WithRawSpanLimits(tracesdk.SpanLimits{
			AttributeValueLengthLimit:   -1,
			AttributeCountLimit:         -1,
			EventCountLimit:             -1,
			LinkCountLimit:              -1,
			AttributePerEventCountLimit: -1,
			AttributePerLinkCountLimit:  -1,
		}),
	)

	// Register our TracerProvider as the global so any imported
	// instrumentation in the future will default to using it.
	otel.SetTracerProvider(tp)

	closer := providerCloser{
		TracerProvider: tp,
	}

	return closer
}

type providerCloser struct {
	*tracesdk.TracerProvider
}

func (t providerCloser) Close() error {
	ctx, cancel := context.WithTimeout(context.Background(), time.Second*5)
	defer cancel()
	return t.Shutdown(ctx)
}

type nopCloser struct {
}

func (*nopCloser) Close() error {
	return nil
}

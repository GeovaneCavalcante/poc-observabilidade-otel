package main

import (
	"context"
	"fmt"
	"math/rand"
	"net/http"
	"time"

	"github.com/prometheus/client_golang/prometheus/promhttp"
	runtimemetrics "go.opentelemetry.io/contrib/instrumentation/runtime"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetrichttp"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

func main() {
	ctx := context.Background()
	configureOpentelemetry()

	meter := global.MeterProvider().Meter("example")
	counter, err := meter.Int64Counter(
		"test.my_counter",
		instrument.WithDescription("Just a test counter"),
	)

	if err != nil {
		panic(err)
	}

	for {
		n := rand.Intn(1000)
		time.Sleep(time.Duration(n) * time.Millisecond)

		counter.Add(ctx, 1)
	}
}

func configureOpentelemetry() {
	if err := runtimemetrics.Start(); err != nil {
		panic(err)
	}
	configureMetrics()

	http.Handle("/metrics", promhttp.Handler())
	fmt.Println("listenening on http://localhost:8088/metrics")

	go func() {
		_ = http.ListenAndServe(":8088", nil)
	}()
}

func configureMetrics() {
	// exporter, err := prometheus.New()
	// if err != nil {
	// 	log.Fatal(err)
	// }
	// provider := metric.NewMeterProvider(metric.WithReader(exporter))

	// global.SetMeterProvider(provider)

	ctx := context.Background()
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceNameKey.String("example"),
			semconv.DeploymentEnvironmentKey.String("1"),
		),
	)

	if err != nil {
		fmt.Println(err)
	}

	opts := []otlpmetrichttp.Option{otlpmetrichttp.WithEndpoint("localhost:4317")}

	exp, err := otlpmetrichttp.New(ctx, opts...)
	if err != nil {
		fmt.Println(err)
	}

	reader := metric.NewPeriodicReader(exp)

	p := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)

	global.SetMeterProvider(p)

	meter := global.MeterProvider().Meter("example")

	counter, err := meter.Int64Counter(
		"test.my_counter",
		instrument.WithDescription("Just a test counter"),
	)

	if err != nil {
		fmt.Println(err)
	}

	counter.Add(ctx, 1)
}

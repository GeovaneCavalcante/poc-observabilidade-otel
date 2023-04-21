package main

import (
	"context"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"os/signal"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/exporters/otlp/otlpmetric/otlpmetricgrpc"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/metric/global"
	"go.opentelemetry.io/otel/metric/instrument"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/metric"
	"go.opentelemetry.io/otel/sdk/resource"
	tracesdk "go.opentelemetry.io/otel/sdk/trace"

	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
)

type PaymentRequest struct {
	ProductID    string `json:"product_id"`
	PaymentToken string `json:"payment_token"`
}

type ProductInfo struct {
	ID    string  `json:"id"`
	Name  string  `json:"name"`
	Price float64 `json:"price"`
}

const (
	service     = "ms-payment"
	environment = "production"
	id          = 1
)

func tracerProvider(url string) (*tracesdk.TracerProvider, error) {
	exp, err := zipkin.New(
		url,
	)
	if err != nil {
		return nil, err
	}
	tp := tracesdk.NewTracerProvider(
		tracesdk.WithSampler(tracesdk.AlwaysSample()),
		tracesdk.WithBatcher(exp),
		tracesdk.WithResource(resource.NewWithAttributes(
			semconv.SchemaURL,
			semconv.ServiceName(service),
		)),
	)
	otel.SetTracerProvider(tp)
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}))
	return tp, nil
}

func metricProvider() (func(context.Context) error, error) {
	ctx := context.Background()

	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(service),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}
	ctx, cancel := context.WithTimeout(ctx, time.Second)
	defer cancel()
	conn, err := grpc.DialContext(ctx, "0.0.0.0:4317",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}
	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}
	reader := metric.NewPeriodicReader(metricExporter)
	metricProvider := metric.NewMeterProvider(
		metric.WithResource(res),
		metric.WithReader(reader),
	)
	global.SetMeterProvider(metricProvider)
	return metricProvider.Shutdown, nil
}

func main() {
	tp, err := tracerProvider("http://localhost:9411/api/v2/spans")
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := tp.Shutdown(context.Background()); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}()

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
	defer cancel()

	shutdown, err := metricProvider()
	if err != nil {
		log.Fatal(err)
	}
	defer func() {
		if err := shutdown(ctx); err != nil {
			log.Fatal("failed to shutdown TracerProvider: %w", err)
		}
	}()

	router := gin.Default()
	router.Use(otelgin.Middleware(service))

	router.POST("/process_payment", processPayment)

	router.Run(":8080")
}

func processPayment(c *gin.Context) {

	ctx := c.Request.Context()

	meter := global.MeterProvider().Meter("payment")

	counterInit, _ := meter.Int64Counter(
		"init-payment",
		instrument.WithDescription("Pagamentos iniciados"),
		instrument.WithUnit("0"),
	)

	counterError, _ := meter.Int64Counter(
		"error-payment",
		instrument.WithDescription("Pagamentos com erro"),
		instrument.WithUnit("0"),
	)

	var paymentRequest PaymentRequest

	if err := c.ShouldBindJSON(&paymentRequest); err != nil {
		counterError.Add(ctx, 1)
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := getProductInfo(ctx, paymentRequest.ProductID)
	if err != nil {
		counterError.Add(ctx, 1)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch product"})
		return
	}

	authorized, err := authorizePayment(ctx, paymentRequest.PaymentToken, product.Price)
	if err != nil {
		counterError.Add(ctx, 1)
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not authorize payment"})
		return
	}

	if !authorized {
		counterError.Add(ctx, 1)
		c.JSON(http.StatusForbidden, gin.H{"status": "Payment not authorized"})
		return
	}

	tr := otel.Tracer("payment-handler")
	ctx = baggage.ContextWithoutBaggage(ctx)
	_, span := tr.Start(ctx, "process file")
	// time.Sleep(2 * time.Second)
	span.SetStatus(codes.Ok, "Payment successful")
	span.End()

	counterInit.Add(ctx, 1)
	c.JSON(http.StatusOK, gin.H{"status": "Payment successful"})
}

func getProductInfo(ctx context.Context, productID string) (*ProductInfo, error) {
	productInfoURL := "http://127.0.0.1:3333/get_product?product_id=" + productID
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	req, err := http.NewRequestWithContext(ctx, "GET", productInfoURL, nil)
	if err != nil {
		return nil, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}

	var productInfo ProductInfo
	if err := json.Unmarshal(body, &productInfo); err != nil {
		return nil, err
	}

	return &productInfo, nil
}

func authorizePayment(ctx context.Context, paymentToken string, amount float64) (bool, error) {

	url := "http://localhost:8081/authorize"
	client := http.Client{Transport: otelhttp.NewTransport(http.DefaultTransport)}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return false, err
	}
	resp, err := client.Do(req)
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()

	return resp.StatusCode == http.StatusOK, nil
}

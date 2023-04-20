package main

import (
	"context"
	"encoding/json"
	"io/ioutil"
	"net/http"
	"time"

	"github.com/gin-gonic/gin"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel"

	"go.opentelemetry.io/otel/baggage"
	"go.opentelemetry.io/otel/exporters/zipkin"
	"go.opentelemetry.io/otel/propagation"
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

// func initProvider() (func(context.Context) error, error) {
// 	ctx := context.Background()

// 	res, err := resource.New(ctx,
// 		resource.WithAttributes(
// 			// the service name used to display traces in backends
// 			semconv.ServiceName("test-service"),
// 		),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create resource: %w", err)
// 	}

// 	// If the OpenTelemetry Collector is running on a local cluster (minikube or
// 	// microk8s), it should be accessible through the NodePort service at the
// 	// `localhost:30080` endpoint. Otherwise, replace `localhost` with the
// 	// endpoint of your cluster. If you run the app inside k8s, then you can
// 	// probably connect directly to the service through dns.
// 	ctx, cancel := context.WithTimeout(ctx, time.Second)
// 	defer cancel()
// 	conn, err := grpc.DialContext(ctx, "localhost:30080",
// 		// Note the use of insecure transport here. TLS is recommended in production.
// 		grpc.WithTransportCredentials(insecure.NewCredentials()),
// 		grpc.WithBlock(),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
// 	}

// 	// Set up a trace exporter
// 	traceExporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
// 	}

// 	// Register the trace exporter with a TracerProvider, using a batch
// 	// span processor to aggregate spans before export.
// 	bsp := sdktrace.NewBatchSpanProcessor(traceExporter)
// 	tracerProvider := sdktrace.NewTracerProvider(
// 		sdktrace.WithSampler(sdktrace.AlwaysSample()),
// 		sdktrace.WithResource(res),
// 		sdktrace.WithSpanProcessor(bsp),
// 	)
// 	otel.SetTracerProvider(tracerProvider)

// 	// set global propagator to tracecontext (the default is no-op).
// 	otel.SetTextMapPropagator(propagation.TraceContext{})

// 	// Shutdown will flush any remaining spans and shut down the exporter.
// 	return tracerProvider.Shutdown, nil
// }

// func main() {
// 	tp, err := tracerProvider("http://localhost:9411/api/v2/spans")
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer func() {
// 		if err := tp.Shutdown(context.Background()); err != nil {
// 			log.Printf("Error shutting down tracer provider: %v", err)
// 		}
// 	}()

// 	router := gin.Default()
// 	router.Use(otelgin.Middleware(service))

// 	router.POST("/process_payment", processPayment)

// 	router.Run(":8080")
// }

func processPayment(c *gin.Context) {

	ctx := c.Request.Context()

	// counter, err := Meter.Int64Counter(
	// 	"test.my_counter",
	// 	instrument.WithUnit("1"),
	// 	instrument.WithDescription("Just a test counter"),
	// )

	// fmt.Println(err)

	// counter.Add(ctx, 1, attribute.String("foo", "bar"))
	// counter.Add(ctx, 10, attribute.String("hello", "world"))

	var paymentRequest PaymentRequest

	if err := c.ShouldBindJSON(&paymentRequest); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	product, err := getProductInfo(ctx, paymentRequest.ProductID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not fetch product"})
		return
	}

	authorized, err := authorizePayment(ctx, paymentRequest.PaymentToken, product.Price)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Could not authorize payment"})
		return
	}

	if !authorized {
		c.JSON(http.StatusForbidden, gin.H{"status": "Payment not authorized"})
		return
	}

	tr := otel.Tracer("payment-handler")
	ctx = baggage.ContextWithoutBaggage(ctx)
	_, span := tr.Start(ctx, "process file")
	time.Sleep(2 * time.Second)
	span.End()

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

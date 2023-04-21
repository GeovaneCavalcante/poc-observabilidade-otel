package main

// func initProvider() (func(context.Context) error, error) {
// 	ctx := context.Background()

// 	res, err := resource.New(ctx,
// 		resource.WithAttributes(
// 			semconv.ServiceName("test-service"),
// 		),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create resource: %w", err)
// 	}
// 	ctx, cancel := context.WithTimeout(ctx, time.Second)
// 	defer cancel()
// 	conn, err := grpc.DialContext(ctx, "0.0.0.0:4317",
// 		grpc.WithTransportCredentials(insecure.NewCredentials()),
// 		grpc.WithBlock(),
// 	)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
// 	}
// 	metricExporter, err := otlpmetricgrpc.New(ctx, otlpmetricgrpc.WithGRPCConn(conn))
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
// 	}
// 	reader := metric.NewPeriodicReader(metricExporter)
// 	metricProvider := metric.NewMeterProvider(
// 		metric.WithResource(res),
// 		metric.WithReader(reader),
// 	)
// 	global.SetMeterProvider(metricProvider)
// 	return metricProvider.Shutdown, nil
// }

// func main() {
// 	log.Printf("Waiting for connection...")

// 	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt)
// 	defer cancel()

// 	shutdown, err := initProvider()
// 	if err != nil {
// 		log.Fatal(err)
// 	}
// 	defer func() {
// 		if err := shutdown(ctx); err != nil {
// 			log.Fatal("failed to shutdown TracerProvider: %w", err)
// 		}
// 	}()

// 	meter := global.MeterProvider().Meter("example")

// 	counter, err := meter.Int64Counter(
// 		"test.my_counter",
// 		instrument.WithDescription("Just a test counter"),
// 	)

// 	for i := 0; i < 10; i++ {
// 		fmt.Println("aq")

// 		if err != nil {
// 			fmt.Println(err)
// 		}

// 		counter.Add(ctx, 1)
// 	}

// 	log.Printf("Done!")
// }

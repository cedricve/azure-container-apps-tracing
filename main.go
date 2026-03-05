package main

import (
	"context"
	cryptorand "crypto/rand"
	"log"
	"math/rand"
	"os"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gin-gonic/gin/otelgin"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.27.0"
	"go.opentelemetry.io/otel/trace"
)

type APIResponse struct {
	Status      string         `json:"status"`
	Timestamp   string         `json:"timestamp"`
	Delay       int            `json:"delay_ms"`
	TraceID     string         `json:"trace_id"`
	Endpoint    string         `json:"endpoint"`
	ServiceName string         `json:"service_name,omitempty"`
	Payload     map[string]any `json:"payload"`
}

type traceStep struct {
	Name  string
	Delay time.Duration
	Attrs []attribute.KeyValue
}

func main() {
	// Load environment variables from .env.local
	if err := godotenv.Load(".env.local"); err != nil {
		log.Printf("Warning: Error loading .env.local file: %v", err)
	}

	serviceName := getEnv("SERVICE_NAME", "aca-tracer")

	shutdown, err := initTracer(context.Background(), serviceName)
	if err != nil {
		log.Fatalf("Failed to initialize tracing: %v", err)
	}
	defer func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdown(ctx); err != nil {
			log.Printf("Failed to shut down tracing: %v", err)
		}
	}()

	// Create Gin router
	router := gin.Default()
	router.Use(otelgin.Middleware(serviceName))

	tracer := otel.Tracer("aca-tracer")

	createFixedHandler := func(endpoint string, steps []traceStep, payload func(*rand.Rand) map[string]any) gin.HandlerFunc {
		return func(c *gin.Context) {
			ctx, span := tracer.Start(contextWithParent(c), endpoint)
			defer span.End()

			totalDelay := time.Duration(0)
			for _, step := range steps {
				_, stepSpan := tracer.Start(ctx, step.Name)
				if len(step.Attrs) > 0 {
					stepSpan.SetAttributes(step.Attrs...)
				}
				time.Sleep(step.Delay)
				stepSpan.End()
				totalDelay += step.Delay
			}

			rng := rand.New(rand.NewSource(time.Now().UnixNano()))
			response := APIResponse{
				Status:      "ok",
				Timestamp:   time.Now().Format(time.RFC3339),
				Delay:       int(totalDelay.Milliseconds()),
				TraceID:     span.SpanContext().TraceID().String(),
				Endpoint:    endpoint,
				ServiceName: serviceName,
				Payload:     payload(rng),
			}

			c.JSON(200, response)
		}
	}

	router.GET("/api/users/:id", createFixedHandler(
		"/api/users/:id",
		[]traceStep{
			{Name: "cache.lookup", Delay: 40 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("cache.key", "user")}},
			{Name: "db.query", Delay: 80 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("db.system", "postgres")}},
		},
		func(rng *rand.Rand) map[string]any {
			tiers := []string{"free", "pro", "enterprise"}
			return map[string]any{
				"user_id":   rng.Intn(9000) + 1000,
				"tier":      tiers[rng.Intn(len(tiers))],
				"last_seen": time.Now().Add(-time.Duration(rng.Intn(72)) * time.Hour).Format(time.RFC3339),
			}
		},
	))

	router.GET("/api/orders/:id", createFixedHandler(
		"/api/orders/:id",
		[]traceStep{
			{Name: "db.query", Delay: 140 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("db.system", "postgres")}},
			{Name: "fraud.check", Delay: 80 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("fraud.provider", "synthetic")}},
		},
		func(rng *rand.Rand) map[string]any {
			return map[string]any{
				"order_id": rng.Intn(90000) + 10000,
				"items":    rng.Intn(5) + 1,
				"total":    20 + rng.Intn(250),
				"currency": "USD",
			}
		},
	))

	router.GET("/api/catalog/search", createFixedHandler(
		"/api/catalog/search",
		[]traceStep{
			{Name: "search.index", Delay: 120 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("search.engine", "vector")}},
			{Name: "ranking", Delay: 60 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("ranking.model", "v2")}},
		},
		func(rng *rand.Rand) map[string]any {
			return map[string]any{
				"query":        "laptops",
				"results":      rng.Intn(20) + 5,
				"latency_tier": "warm",
			}
		},
	))

	router.POST("/api/checkout", createFixedHandler(
		"/api/checkout",
		[]traceStep{
			{Name: "inventory.reserve", Delay: 140 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("warehouse", "west")}},
			{Name: "payment.authorize", Delay: 220 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("payment.provider", "sandbox")}},
			{Name: "shipping.quote", Delay: 90 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("carrier", "mock")}},
		},
		func(rng *rand.Rand) map[string]any {
			return map[string]any{
				"checkout_id": rng.Intn(900000) + 100000,
				"status":      "authorized",
				"eta_days":    rng.Intn(4) + 1,
			}
		},
	))

	router.GET("/api/recommendations", createFixedHandler(
		"/api/recommendations",
		[]traceStep{
			{Name: "feature.fetch", Delay: 120 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("store", "redis")}},
			{Name: "model.inference", Delay: 200 * time.Millisecond, Attrs: []attribute.KeyValue{attribute.String("model", "rec-v1")}},
		},
		func(rng *rand.Rand) map[string]any {
			return map[string]any{
				"recommendations": []int{rng.Intn(1000), rng.Intn(1000), rng.Intn(1000)},
				"strategy":        "similarity",
			}
		},
	))

	// Start server
	port := getEnv("PORT", "8081")
	log.Printf("Starting server on port %s", port)

	if err := router.Run(":" + port); err != nil {
		log.Fatal("Failed to start server: ", err)
	}
}

// getEnv retrieves an environment variable or returns a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func initTracer(ctx context.Context, serviceName string) (func(context.Context) error, error) {
	exporter, err := otlptracehttp.New(ctx)
	if err != nil {
		return nil, err
	}

	res, err := resource.New(
		ctx,
		resource.WithSchemaURL(semconv.SchemaURL),
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
		),
	)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
	)

	otel.SetTracerProvider(tracerProvider)
	otel.SetTextMapPropagator(
		propagation.NewCompositeTextMapPropagator(propagation.TraceContext{}, propagation.Baggage{}),
	)

	return tracerProvider.Shutdown, nil
}

func contextWithParent(c *gin.Context) context.Context {
	ctx := c.Request.Context()
	if trace.SpanContextFromContext(ctx).IsValid() {
		return ctx
	}

	traceIDHex := c.Query("trace_id")
	if traceIDHex == "" {
		return ctx
	}

	traceID, err := trace.TraceIDFromHex(traceIDHex)
	if err != nil {
		return ctx
	}

	spanIDHex := c.Query("parent_span_id")
	spanID, err := spanIDFromHexOrRandom(spanIDHex)
	if err != nil {
		return ctx
	}

	parent := trace.NewSpanContext(trace.SpanContextConfig{
		TraceID:    traceID,
		SpanID:     spanID,
		TraceFlags: trace.FlagsSampled,
		Remote:     true,
	})

	return trace.ContextWithSpanContext(ctx, parent)
}

func spanIDFromHexOrRandom(hexValue string) (trace.SpanID, error) {
	if hexValue != "" {
		return trace.SpanIDFromHex(hexValue)
	}

	for i := 0; i < 2; i++ {
		var spanID trace.SpanID
		_, _ = cryptorand.Read(spanID[:])
		if spanID.IsValid() {
			return spanID, nil
		}
	}

	return trace.SpanID{}, nil
}

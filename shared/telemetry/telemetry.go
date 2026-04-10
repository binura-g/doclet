package telemetry

import (
	"context"
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.30.0"
)

const (
	defaultExportTimeout = 10 * time.Second
	defaultSampleRatio   = 1.0
)

type Options struct {
	DefaultServiceName string
	ServiceNameEnvVar  string
}

type Config struct {
	ServiceName           string
	ServiceVersion        string
	DeploymentEnvironment string
	Endpoint              string
	Headers               map[string]string
	Insecure              bool
	ExportTimeout         time.Duration
	SampleRatio           float64
	Enabled               bool
}

func Setup(ctx context.Context, opts Options) (func(context.Context) error, error) {
	otel.SetTextMapPropagator(newPropagator())

	cfg, err := loadConfig(opts)
	if err != nil {
		return nil, err
	}
	if !cfg.Enabled {
		return func(context.Context) error { return nil }, nil
	}

	exporterOpts := []otlptracegrpc.Option{
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
		otlptracegrpc.WithTimeout(cfg.ExportTimeout),
	}
	if cfg.Insecure {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithInsecure())
	}
	if len(cfg.Headers) > 0 {
		exporterOpts = append(exporterOpts, otlptracegrpc.WithHeaders(cfg.Headers))
	}

	exporter, err := otlptracegrpc.New(ctx, exporterOpts...)
	if err != nil {
		return nil, fmt.Errorf("create otlp exporter: %w", err)
	}

	res, err := newResource(ctx, cfg)
	if err != nil {
		return nil, err
	}

	tracerProvider := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter),
		sdktrace.WithResource(res),
		sdktrace.WithSampler(sdktrace.ParentBased(sdktrace.TraceIDRatioBased(cfg.SampleRatio))),
	)
	otel.SetTracerProvider(tracerProvider)

	return tracerProvider.Shutdown, nil
}

func InjectNATSHeaders(ctx context.Context, header nats.Header) {
	if header == nil {
		return
	}
	newPropagator().Inject(ctx, natsHeaderCarrier(header))
}

func ExtractNATSContext(ctx context.Context, header nats.Header) context.Context {
	if header == nil {
		return ctx
	}
	return newPropagator().Extract(ctx, natsHeaderCarrier(header))
}

func loadConfig(opts Options) (Config, error) {
	serviceName := strings.TrimSpace(os.Getenv(opts.ServiceNameEnvVar))
	if serviceName == "" {
		serviceName = strings.TrimSpace(os.Getenv("OTEL_SERVICE_NAME"))
	}
	if serviceName == "" {
		serviceName = opts.DefaultServiceName
	}

	endpoint := firstNonEmpty(
		os.Getenv("DOCLET_OTEL_EXPORTER_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_ENDPOINT"),
		os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"),
	)

	insecure, err := parseBool(firstNonEmpty(
		os.Getenv("DOCLET_OTEL_EXPORTER_INSECURE"),
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_INSECURE"),
		os.Getenv("OTEL_EXPORTER_OTLP_INSECURE"),
	), false)
	if err != nil {
		return Config{}, fmt.Errorf("parse OTEL insecure flag: %w", err)
	}

	timeout, err := parseDurationMillis(firstNonEmpty(
		os.Getenv("DOCLET_OTEL_EXPORT_TIMEOUT"),
		os.Getenv("OTEL_EXPORTER_OTLP_TRACES_TIMEOUT"),
		os.Getenv("OTEL_EXPORTER_OTLP_TIMEOUT"),
	), defaultExportTimeout)
	if err != nil {
		return Config{}, fmt.Errorf("parse OTEL export timeout: %w", err)
	}

	sampleRatio, err := parseSampleRatio(os.Getenv("DOCLET_OTEL_SAMPLER_RATIO"))
	if err != nil {
		return Config{}, fmt.Errorf("parse OTEL sample ratio: %w", err)
	}

	enabled, err := parseBool(os.Getenv("DOCLET_OTEL_ENABLED"), endpoint != "")
	if err != nil {
		return Config{}, fmt.Errorf("parse OTEL enabled flag: %w", err)
	}

	return Config{
		ServiceName:           serviceName,
		ServiceVersion:        strings.TrimSpace(os.Getenv("DOCLET_OTEL_SERVICE_VERSION")),
		DeploymentEnvironment: strings.TrimSpace(os.Getenv("DOCLET_OTEL_ENVIRONMENT")),
		Endpoint:              strings.TrimSpace(endpoint),
		Headers:               parseHeaders(firstNonEmpty(os.Getenv("DOCLET_OTEL_EXPORTER_HEADERS"), os.Getenv("OTEL_EXPORTER_OTLP_TRACES_HEADERS"), os.Getenv("OTEL_EXPORTER_OTLP_HEADERS"))),
		Insecure:              insecure,
		ExportTimeout:         timeout,
		SampleRatio:           sampleRatio,
		Enabled:               enabled,
	}, nil
}

func newResource(ctx context.Context, cfg Config) (*resource.Resource, error) {
	attrs := []attribute.KeyValue{
		semconv.ServiceName(cfg.ServiceName),
	}
	if cfg.ServiceVersion != "" {
		attrs = append(attrs, semconv.ServiceVersion(cfg.ServiceVersion))
	}
	if cfg.DeploymentEnvironment != "" {
		attrs = append(attrs, semconv.DeploymentEnvironmentName(cfg.DeploymentEnvironment))
	}

	res, err := resource.New(
		ctx,
		resource.WithFromEnv(),
		resource.WithTelemetrySDK(),
		resource.WithAttributes(attrs...),
	)
	if err != nil {
		return nil, fmt.Errorf("build otel resource: %w", err)
	}
	return res, nil
}

func newPropagator() propagation.TextMapPropagator {
	return propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	)
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}

func parseBool(value string, fallback bool) (bool, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(trimmed)
	if err != nil {
		return false, err
	}
	return parsed, nil
}

func parseDurationMillis(value string, fallback time.Duration) (time.Duration, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return fallback, nil
	}
	millis, err := strconv.Atoi(trimmed)
	if err != nil {
		return 0, err
	}
	return time.Duration(millis) * time.Millisecond, nil
}

func parseSampleRatio(value string) (float64, error) {
	trimmed := strings.TrimSpace(value)
	if trimmed == "" {
		return defaultSampleRatio, nil
	}
	ratio, err := strconv.ParseFloat(trimmed, 64)
	if err != nil {
		return 0, err
	}
	if ratio < 0 || ratio > 1 {
		return 0, fmt.Errorf("must be between 0 and 1")
	}
	return ratio, nil
}

func parseHeaders(value string) map[string]string {
	headers := make(map[string]string)
	for _, part := range strings.Split(value, ",") {
		key, rawValue, ok := strings.Cut(strings.TrimSpace(part), "=")
		if !ok {
			continue
		}
		key = strings.TrimSpace(key)
		rawValue = strings.TrimSpace(rawValue)
		if key == "" || rawValue == "" {
			continue
		}
		headers[key] = rawValue
	}
	if len(headers) == 0 {
		return nil
	}
	return headers
}

type natsHeaderCarrier nats.Header

func (c natsHeaderCarrier) Get(key string) string {
	return nats.Header(c).Get(key)
}

func (c natsHeaderCarrier) Set(key, value string) {
	nats.Header(c).Set(key, value)
}

func (c natsHeaderCarrier) Keys() []string {
	headers := nats.Header(c)
	keys := make([]string, 0, len(headers))
	for key := range headers {
		keys = append(keys, key)
	}
	return keys
}

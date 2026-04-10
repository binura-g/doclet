package collab

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	"doclet/shared/telemetry"

	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type NatsBroker struct {
	nc *nats.Conn
}

var natsTracer = otel.Tracer("doclet/services/collab/nats")

func NewNatsBroker(url string) (*NatsBroker, error) {
	nc, err := nats.Connect(url)
	if err != nil {
		return nil, err
	}
	return &NatsBroker{nc: nc}, nil
}

func (b *NatsBroker) Close() {
	if b.nc != nil {
		b.nc.Close()
	}
}

func (b *NatsBroker) Publish(ctx context.Context, subject string, msg Message) {
	ctx, span := natsTracer.Start(ctx, "nats publish",
		trace.WithAttributes(
			attribute.String("messaging.system", "nats"),
			attribute.String("messaging.destination.name", subject),
			attribute.String("messaging.operation.type", "send"),
			attribute.String("doclet.message.type", msg.Type),
			attribute.String("doclet.document_id", msg.DocumentID),
		),
	)
	defer span.End()

	data, err := json.Marshal(msg)
	if err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "marshal message")
		log.Printf("nats marshal error: %v", err)
		return
	}

	natsMsg := nats.NewMsg(subject)
	natsMsg.Data = data
	telemetry.InjectNATSHeaders(ctx, natsMsg.Header)

	if err := b.nc.PublishMsg(natsMsg); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, "publish message")
		log.Printf("nats publish error: %v", err)
	}
}

func (b *NatsBroker) Subscribe(subject string, handler func(context.Context, Message)) (*nats.Subscription, error) {
	return b.nc.Subscribe(subject, func(msg *nats.Msg) {
		ctx := telemetry.ExtractNATSContext(context.Background(), msg.Header)
		ctx, span := natsTracer.Start(ctx, "nats receive",
			trace.WithAttributes(
				attribute.String("messaging.system", "nats"),
				attribute.String("messaging.destination.name", msg.Subject),
				attribute.String("messaging.operation.type", "process"),
			),
		)
		defer span.End()

		var payload Message
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "decode message")
			log.Printf("nats decode error: %v", err)
			return
		}
		span.SetAttributes(
			attribute.String("doclet.message.type", payload.Type),
			attribute.String("doclet.document_id", payload.DocumentID),
		)
		handler(ctx, payload)
	})
}

func SubjectForDocument(docID, suffix string) string {
	clean := strings.TrimSpace(docID)
	if clean == "" {
		clean = "unknown"
	}
	return "doclet.documents." + clean + "." + suffix
}

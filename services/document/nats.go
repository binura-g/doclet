package document

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"log"
	"time"

	"doclet/shared/telemetry"

	"github.com/google/uuid"
	"github.com/nats-io/nats.go"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

type SnapshotMessage struct {
	DocumentID string `json:"document_id"`
	Content    string `json:"content"`
	Payload    string `json:"payload"`
}

var snapshotConsumerTracer = otel.Tracer("doclet/services/document/nats")

func StartSnapshotConsumer(ctx context.Context, store *Store, natsURL string) (*nats.Conn, error) {
	nc, err := nats.Connect(natsURL)
	if err != nil {
		return nil, err
	}

	subject := "doclet.documents.*.snapshots"
	_, err = nc.Subscribe(subject, func(msg *nats.Msg) {
		messageCtx := telemetry.ExtractNATSContext(ctx, msg.Header)
		messageCtx, span := snapshotConsumerTracer.Start(messageCtx, "nats consume snapshot",
			trace.WithAttributes(
				attribute.String("messaging.system", "nats"),
				attribute.String("messaging.destination.name", msg.Subject),
				attribute.String("messaging.operation.type", "process"),
			),
		)
		defer span.End()

		var payload SnapshotMessage
		if err := json.Unmarshal(msg.Data, &payload); err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "decode snapshot")
			log.Printf("nats snapshot decode error: %v", err)
			return
		}
		docID, err := uuid.Parse(payload.DocumentID)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid document id")
			log.Printf("nats snapshot invalid document_id: %v", err)
			return
		}
		span.SetAttributes(attribute.String("doclet.document_id", docID.String()))
		encoded := payload.Content
		if encoded == "" {
			encoded = payload.Payload
		}
		content, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			span.RecordError(err)
			span.SetStatus(codes.Error, "invalid snapshot content")
			log.Printf("nats snapshot invalid content: %v", err)
			return
		}

		updateCtx, cancel := context.WithTimeout(messageCtx, 5*time.Second)
		defer cancel()
		if err := store.UpdateContent(updateCtx, docID, content); err != nil {
			if IsNotFound(err) {
				log.Printf("nats snapshot ignored missing document: %s", docID)
				return
			}
			span.RecordError(err)
			span.SetStatus(codes.Error, "update snapshot")
			log.Printf("nats snapshot update error: %v", err)
		}
	})
	if err != nil {
		nc.Close()
		return nil, err
	}

	return nc, nil
}

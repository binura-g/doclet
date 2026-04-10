package main

import (
	"context"
	"log"
	"net/http"
	"os/signal"
	"syscall"
	"time"

	"doclet/services/document"
	"doclet/shared/telemetry"
)

func main() {
	cfg := document.LoadConfig()
	if cfg.DatabaseURL == "" {
		log.Fatal("DOCLET_DATABASE_URL is required")
	}

	shutdownTracing, err := telemetry.Setup(context.Background(), telemetry.Options{
		DefaultServiceName: "doclet-document",
		ServiceNameEnvVar:  "DOCLET_DOCUMENT_OTEL_SERVICE_NAME",
	})
	if err != nil {
		log.Fatalf("tracing setup failed: %v", err)
	}
	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := shutdownTracing(shutdownCtx); err != nil {
			log.Printf("tracing shutdown failed: %v", err)
		}
	}()

	db, err := document.OpenDatabase(cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("database connection failed: %v", err)
	}
	if err := document.RunMigrations(db); err != nil {
		log.Fatalf("database migration failed: %v", err)
	}

	store := document.NewStore(db)

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer cancel()

	nc, err := document.StartSnapshotConsumer(ctx, store, cfg.NATSURL)
	if err != nil {
		log.Fatalf("nats connection failed: %v", err)
	}
	defer nc.Close()

	srv := &http.Server{
		Addr:              cfg.HTTPAddr,
		Handler:           document.NewServer(store).Router(),
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("document service listening on %s", cfg.HTTPAddr)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("document service failed: %v", err)
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := signal.NotifyContext(context.Background(), syscall.SIGTERM, syscall.SIGINT)
	defer shutdownCancel()
	_ = srv.Shutdown(shutdownCtx)
}

package main

import (
	"context"
	"flag"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"go.temporal.io/sdk/client"

	"github.com/example/scte224service/internal/config"
	"github.com/example/scte224service/internal/httpapi"
	"github.com/example/scte224service/internal/service"
	"github.com/example/scte224service/internal/temporal"
)

func main() {
	configPath := flag.String("config", "", "path to YAML config")
	flag.Parse()

	cfg, err := config.Load(*configPath)
	if err != nil {
		log.Fatalf("load config: %v", err)
	}

	temporalClient, err := client.Dial(client.Options{
		HostPort:  cfg.Temporal.HostPort,
		Namespace: cfg.Temporal.Namespace,
	})
	if err != nil {
		log.Fatalf("connect to Temporal: %v", err)
	}
	defer temporalClient.Close()

	svc := service.New(temporalClient, cfg.Temporal.TaskQueue, cfg.Audiences, os.Stdout)
	worker := temporal.NewWorker(temporalClient, svc, cfg.Temporal.TaskQueue)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	workerStop := make(chan struct{})
	go func() {
		if err := worker.Run(workerStop); err != nil {
			log.Printf("temporal worker exited: %v", err)
		}
	}()

	server := &http.Server{
		Addr:    cfg.HTTP.Address,
		Handler: httpapi.New(svc),
	}

	go func() {
		<-ctx.Done()
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := server.Shutdown(shutdownCtx); err != nil {
			log.Printf("http shutdown: %v", err)
		}
		close(workerStop)
		worker.Stop()
	}()

	log.Printf("HTTP listening on %s", cfg.HTTP.Address)
	if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
		log.Fatalf("server error: %v", err)
	}
}

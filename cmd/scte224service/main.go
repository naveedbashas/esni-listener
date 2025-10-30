package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/example/scte224service/internal/domain"
	"github.com/example/scte224service/internal/service"
)

func main() {
	ctx := context.Background()

	file, err := os.Open("SCTE-224_example.xml")
	if err != nil {
		log.Fatalf("unable to open SCTE-224 example: %v", err)
	}
	defer file.Close()

	srv := service.New(service.Config{
		Output: os.Stdout,
	})
	srv.Start()
	defer srv.Stop()

	media, err := srv.IngestMedia(ctx, file)
	if err != nil {
		log.Fatalf("ingest failed: %v", err)
	}

	fmt.Printf("Ingested media %s with %d media points\n", media.ID, len(media.MediaPoints))

	// Allow scheduler to trigger immediate past events.
	time.Sleep(500 * time.Millisecond)

	// Simulate signal for WANF primary FAST stream start (eventId 0x1234).
	srv.ProcessSignal(&domain.SCTE35Event{
		EventID:          "0x1234",
		SegmentationType: "0x11",
		ArrivedAt:        time.Now().UTC(),
	})

	// Simulate signal for WANF primary FAST stream stop (segmentationType 0x10).
	srv.ProcessSignal(&domain.SCTE35Event{
		EventID:             "0x1234",
		SegmentationEventID: "0x1234",
		SegmentationType:    "0x10",
		ArrivedAt:           time.Now().UTC().Add(2 * time.Second),
	})

	// Simulate high priority local override.
	srv.ProcessSignal(&domain.SCTE35Event{
		PrivateIndicator: "PrioritySwitch",
		ArrivedAt:        time.Now().UTC().Add(3 * time.Second),
	})

	// Give goroutines time to process.
	time.Sleep(2 * time.Second)

	for audience, decision := range srv.Snapshot() {
		fmt.Printf("Audience %s manifest -> alt=%s fallback=%s action=%s priority=%d\n",
			audience, decision.AlternateURI, decision.FallbackURI, decision.Action, decision.Priority)
	}
}

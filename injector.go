package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	cl, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Redpanda: %v", err))
	}
	defer cl.Close()

	ctx := context.Background()
	fmt.Println("SRE-Pilot Log Injector: Active. Pumping traffic to Redpanda...")

	pods := []string{"gateway-01", "auth-service-02", "db-primary", "worker-node-alt"}

	for {
		pod := pods[rand.Intn(len(pods))]
		isError := rand.Float32() > 0.7 

		var logLine string
		timestamp := time.Now().Unix()

		if isError {
			errors := []string{
				"LEVEL=FATAL SOURCE=api-gateway-01 MSG=SSL_CERT_EXPIRED",
				"LEVEL=ERROR SOURCE=db-primary MSG=connection_pool_exhausted",
				"LEVEL=FATAL SOURCE=worker-node MSG=OOM_Killed",
				"LEVEL=ERROR SOURCE=auth-service MSG=invalid_token_signature_spike",
			}
			msg := errors[rand.Intn(len(errors))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
			fmt.Printf(">>> INJECTED CRITICAL: %s\n", logLine)
		} else {
			normals := []string{
				"LEVEL=INFO MSG=request_processed_200",
				"LEVEL=INFO MSG=cache_hit",
				"LEVEL=DEBUG MSG=background_sync_complete",
			}
			msg := normals[rand.Intn(len(normals))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
		}

		record := &kgo.Record{Topic: "health-events", Value: []byte(logLine)}
		if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
			fmt.Printf("Produce error: %v\n", err)
		}

		time.Sleep(1500 * time.Millisecond)
	}
}
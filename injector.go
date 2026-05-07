package main

import (
	"context"
	"fmt"
	"math/rand"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	// Seed randomness so every run is different
	rand.Seed(time.Now().UnixNano())

	cl, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
	)
	if err != nil {
		panic(fmt.Sprintf("Failed to connect to Redpanda: %v", err))
	}
	defer cl.Close()

	ctx := context.Background()
	fmt.Println("SRE-Pilot Log Injector: Active. Pumping RGB traffic to Redpanda...")

	pods := []string{"gateway-01", "auth-service-02", "db-primary", "worker-node-alt", "cache-redis-01"}

	for {
		pod := pods[rand.Intn(len(pods))]
		timestamp := time.Now().Unix()
		var logLine string

		// Probability Distribution:
		// 15% Critical (Red), 25% Warning (Green), 60% Normal (Blue)
		chance := rand.Float32()

		if chance > 0.85 {
			// CRITICAL (Red in Galaxy)
			errors := []string{
				"LEVEL=FATAL MSG=SSL_CERT_EXPIRED",
				"LEVEL=ERROR MSG=connection_pool_exhausted",
				"LEVEL=FATAL MSG=OOM_Killed",
				"LEVEL=ERROR MSG=invalid_token_signature_spike",
			}
			msg := errors[rand.Intn(len(errors))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
			fmt.Printf(">>> [RED] INJECTED CRITICAL: %s\n", logLine)

		} else if chance > 0.60 {
			// WARNING (Green in Galaxy)
			warnings := []string{
				"LEVEL=WARN MSG=high_p99_latency_detected",
				"LEVEL=WARN MSG=disk_usage_reaching_85_percent",
				"LEVEL=WARN MSG=slow_query_detected_on_primary",
				"LEVEL=WARN MSG=upstream_dependency_timeout_retry",
				"LEVEL=WARN MSG=rate_limit_soft_cap_reached",
			}
			msg := warnings[rand.Intn(len(warnings))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
			fmt.Printf(">>> [GREEN] INJECTED WARNING: %s\n", logLine)

		} else {
			// NORMAL (Blue in Galaxy)
			normals := []string{
				"LEVEL=INFO MSG=request_processed_200",
				"LEVEL=INFO MSG=cache_hit",
				"LEVEL=DEBUG MSG=background_sync_complete",
				"LEVEL=INFO MSG=health_check_passed",
			}
			msg := normals[rand.Intn(len(normals))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
		}

		record := &kgo.Record{Topic: "health-events", Value: []byte(logLine)}
		if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
			fmt.Printf("Produce error: %v\n", err)
		}

		// Faster heartbeat (800ms) to see the Galaxy fill up quickly
		time.Sleep(800 * time.Millisecond)
	}
}
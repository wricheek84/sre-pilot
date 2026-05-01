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
	fmt.Println(" Log Injector Online. Pumping live traffic to Redpanda...")


	pods := []string{"pod-1", "pod-2", "pod-3", "pod-99"}

	for {
		
		pod := pods[rand.Intn(len(pods))]
		
		
		isError := rand.Float32() > 0.85 

		var logLine string
		timestamp := time.Now().Unix()

		if isError {
			errors := []string{
				"LEVEL=ERROR MSG=db_connection_fail_critical",
				"LEVEL=FATAL MSG=OOM_Killed_memory_limit_exceeded",
				"LEVEL=ERROR MSG=CPU_spike_detected_throttling",
				"LEVEL=FATAL MSG=health_check_timeout_node_dead",
			}
			msg := errors[rand.Intn(len(errors))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
			fmt.Printf("🔥 INJECTED ERROR: %s\n", logLine)
		} else {
			normals := []string{
				"LEVEL=INFO MSG=request_processed_200",
				"LEVEL=INFO MSG=cache_hit_successful",
				"LEVEL=DEBUG MSG=garbage_collection_run_ok",
			}
			msg := normals[rand.Intn(len(normals))]
			logLine = fmt.Sprintf("TIME=%d %s SOURCE=%s", timestamp, msg, pod)
			
		}

		
		record := &kgo.Record{Topic: "health-events", Value: []byte(logLine)}
		if err := cl.ProduceSync(ctx, record).FirstErr(); err != nil {
			fmt.Printf("Produce error: %v\n", err)
		}

		
		time.Sleep(2 * time.Second)
	}
}
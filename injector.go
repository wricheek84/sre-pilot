package main

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	client, err := kgo.NewClient(kgo.SeedBrokers("localhost:9092"))
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	ctx := context.Background()
	
	
	errors := []string{
		"auth_service: invalid token checksum",
		"db_node_1: connection pool exhausted",
		"api_gateway: rate limit exceeded for user_id_99",
	}

	fmt.Println("Chaos Injector Running... Flooding 'health-events' topic.")

	for {
		
		for _, errStr := range errors {
			for i := 0; i < 50; i++ {
				
				podID := fmt.Sprintf("pod-%d", i)
				payload := fmt.Sprintf("TIME=%d LEVEL=ERROR MSG=%s SOURCE=%s", time.Now().Unix(), errStr, podID)

				record := &kgo.Record{Topic: "health-events", Value: []byte(payload)}
				client.Produce(ctx, record, nil)
			}
		}
		fmt.Printf("Batch of 150 logs injected at %v\n", time.Now().Format("15:04:05"))
		time.Sleep(1 * time.Second)
	}
}
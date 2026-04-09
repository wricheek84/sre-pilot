package main

import (
	"context"
	"fmt"
	"log"

	"github.com/twmb/franz-go/pkg/kgo"
)

func main() {
	ctx := context.Background()

	client, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
		kgo.ConsumeTopics("health-events"),
	)
	if err != nil {
		log.Fatal(err)
	}
	defer client.Close()

	fmt.Println("🧠 Orchestrator online... listening for chaos.")

	for {
		fetches := client.PollFetches(ctx)

		if fetches.IsClientClosed() {
			log.Println("Client closed, exiting loop.")
			break
		}

		if errs := fetches.Errors(); len(errs) > 0 {
			for _, err := range errs {
				fmt.Println("Error:", err)
			}
			continue
		}

		for _, record := range fetches.Records() {
			fmt.Printf("Alert Received: %s\n", string(record.Value))
		}
	}
}
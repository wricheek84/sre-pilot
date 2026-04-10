package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"net/http"
	"os"
	"time"

	"github.com/twmb/franz-go/pkg/kgo"
)


type SREEvent[T any] struct {
	Type      string `json:"type"`
	Service   string `json:"service"`
	Timestamp int64  `json:"ts"`
	Data      T      `json:"data"`
}


type HealthData struct {
	Status int `json:"status"`
}

type ChaosData struct {
	Action string `json:"action"`
	Error  string `json:"error,omitempty"`
}


var kafkaClient *kgo.Client

func main() {

	client, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
	)
	if err != nil {
		log.Fatalf("❌ Failed to start Redpanda client: %v", err)
	}
	kafkaClient = client
	defer kafkaClient.Close()

	rand.Seed(time.Now().UnixNano())



	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		sendEvent("heartbeat", HealthData{Status: 200})
		
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "I'm alive!")
	})

	http.HandleFunc("/chaos", func(w http.ResponseWriter, r *http.Request) {
		randomNum := rand.Intn(3) + 1
		
		switch randomNum {
		case 1:
			fmt.Println("CRITICAL: Hard Crash triggered.")
			sendEvent("critical_failure", ChaosData{Action: "crash", Error: "os.Exit(1)"})
		    kafkaClient.Flush(context.Background()) 
			os.Exit(1)

		case 2:
			fmt.Println("📈 LOAD: Simulating CPU spike...")
			sendEvent("performance_alert", ChaosData{Action: "cpu_spike"})
			
			sum := 0
			for i := 0; i < 500000000; i++ {
				sum += i
			}
			fmt.Fprintf(w, "Spike complete. Sum: %d", sum)

		case 3:
			fmt.Println(" ERROR: Simulating 500 Internal Server Error.")
			sendEvent("api_error", ChaosData{Action: "500_error", Error: "Custom SRE Simulation Error"})
			
			http.Error(w, "Custom SRE Simulation Error", http.StatusInternalServerError)
		}
	})

	fmt.Println("🚀 Patient App starting on port 8080...")
	log.Fatal(http.ListenAndServe(":8080", nil))
}


func sendEvent(eventType string, data any) {
	event := SREEvent[any]{
		Type:      eventType,
		Service:   "target-app",
		Timestamp: time.Now().UnixMilli(),
		Data:      data,
	}

	payload, err := json.Marshal(event)
	if err != nil {
		log.Printf("❌ Marshal error: %v", err)
		return
	}

	
	kafkaClient.Produce(context.Background(), &kgo.Record{
		Topic: "health-events",
		Value: payload,
	}, func(_ *kgo.Record, err error) {
		if err != nil {
			log.Printf("❌ Failed to deliver message: %v", err)
		}
	})
}
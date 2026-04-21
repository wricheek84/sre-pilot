package main

import (
	"context"
	"crypto/sha256" 
	"encoding/binary" 
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/wrich/sre-pilot/proto"
)

func main() {
	ctx := context.Background()

	// 1. Setup C++ Engine
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to C++ Engine: %v", err)
	}
	defer conn.Close()
	inferenceClient := pb.NewInferenceEngineClient(conn)

	// 2. Setup Redis (The Guard)
	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	// 3. Setup Qdrant (The Memory)
	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   "localhost",
		Port:   6334,
		UseTLS: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qClient.Close()

	// 3.5. Ensure Qdrant Collection exists (FIXED LOGIC)
	collectionName := "incident_vectors"
	collections, err := qClient.ListCollections(ctx)
	if err != nil {
		log.Fatalf("Failed to list Qdrant collections: %v", err)
	}

	exists := false
	for _, col := range collections {
		if col == collectionName {
			exists = true
			break
		}
	}

	if !exists {
		fmt.Printf("Creating Qdrant collection: %s...\n", collectionName)
		err = qClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     384, // Must match C++ model output
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			log.Fatalf("Failed to create Qdrant collection: %v", err)
		}
		fmt.Println("Collection created successfully.")
	}

	// 4. Setup Kafka (The Nervous System)
	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
		kgo.ConsumeTopics("health-events"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redpanda: %v", err)
	}
	defer kafkaClient.Close()

	fmt.Println(" Orchestrator online... Everything connected.")

	// 5. THE LOOP
	for {
		fetches := kafkaClient.PollFetches(ctx)
		if fetches.IsClientClosed() {
			break
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			logLine := string(record.Value)

			fmt.Printf("\n[Stream] New Log Received: %s\n", logLine)
			processWithAI(ctx, inferenceClient, rdb, qClient, logLine)
		}
	}
}

func processWithAI(ctx context.Context, client pb.InferenceEngineClient, rdb *redis.Client, qClient *qdrant.Client, logLine string) {
	ctx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()

	resp, err := client.RunInference(ctx, &pb.InferenceRequest{
		ModelId: "embedder",
		LogLine: logLine,
	})

	if err != nil {
		log.Printf("AI Engine Error: %v", err)
		return
	}

	if len(resp.Embedding) >= 3 {
		incidentID := generateIncidentHash(resp.Embedding)
        fmt.Printf("Incident Identity: %s\n", incidentID)
		sample := resp.Embedding[:3]
		fmt.Printf("AI Fingerprint: [Len: %v] | Sample: %.4f, %.4f, %.4f...\n", len(resp.Embedding), sample[0], sample[1], sample[2])
	} else {
		fmt.Printf("AI Fingerprint: Received unexpected empty vector\n")
	}
}

func generateIncidentHash(embedding []float32) string {
	
	buf := make([]byte, len(embedding)*4)
	
	for i, f := range embedding {
		
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}


	hash := sha256.Sum256(buf)


	return hex.EncodeToString(hash[:])
}
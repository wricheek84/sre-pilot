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
	traceDB := InitTraceDB()
	defer traceDB.Close()
	
	conn, err := grpc.NewClient("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("Failed to connect to C++ Engine: %v", err)
	}
	defer conn.Close()
	inferenceClient := pb.NewInferenceEngineClient(conn)

	rdb := redis.NewClient(&redis.Options{
		Addr: "localhost:6379",
	})
	if err := rdb.Ping(ctx).Err(); err != nil {
		log.Fatalf("Failed to connect to Redis: %v", err)
	}

	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   "localhost",
		Port:   6334,
		UseTLS: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qClient.Close()

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
		err = qClient.CreateCollection(ctx, &qdrant.CreateCollection{
			CollectionName: collectionName,
			VectorsConfig: qdrant.NewVectorsConfig(&qdrant.VectorParams{
				Size:     384,
				Distance: qdrant.Distance_Cosine,
			}),
		})
		if err != nil {
			log.Fatalf("Failed to create Qdrant collection: %v", err)
		}
	}

	kafkaClient, err := kgo.NewClient(
		kgo.SeedBrokers("localhost:9092"),
		kgo.ConsumeTopics("health-events"),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redpanda: %v", err)
	}
	defer kafkaClient.Close()

	cache := NewResilienceCache(5000)

	logChan := make(chan string, 100)
	for i := 0; i < 16; i++ {
        go worker(ctx, logChan, inferenceClient, rdb, qClient, cache)
    }

	fmt.Println("Orchestrator online... Everything connected.")

	for {
		fetches := kafkaClient.PollFetches(ctx)
		if fetches.IsClientClosed() {
			break
		}
		if(fetches.Empty()){
			time.Sleep(100 * time.Millisecond)
			continue
		}

		iter := fetches.RecordIter()
		for !iter.Done() {
			record := iter.Next()
			logLine := string(record.Value)

			fmt.Printf("\n[Stream] New Log Received: %s\n", logLine)
			logChan <- logLine
		}
	}
}

func processWithAI(ctx context.Context, client pb.InferenceEngineClient, rdb *redis.Client, qClient *qdrant.Client, logLine string, embedding []float32) []float32 {
	if embedding == nil {
		ctxIn, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		resp, err := client.RunInference(ctxIn, &pb.InferenceRequest{
			ModelId: "embedder",
			LogLine: logLine,
		})

		if err != nil {
			log.Printf("AI Engine Error: %v", err)
			return nil
		}
		embedding = resp.Embedding
	}

	if len(embedding) >= 384 {
		incidentID := generateIncidentHash(embedding)
		qID := fmt.Sprintf("%s-%s-%s-%s-%s",
			incidentID[0:8], incidentID[8:12], incidentID[12:16], incidentID[16:20], incidentID[20:32])

		searchRes, err := qClient.Query(ctx, &qdrant.QueryPoints{
			CollectionName: "incident_vectors",
			Query:          qdrant.NewQuery(embedding...),
			Limit:          ptrUint64(1),
			WithPayload:    qdrant.NewWithPayload(true),
		})

		var similarity float32 = 0.0
		if err == nil && len(searchRes) > 0 {
			similarity = searchRes[0].Score
		}

		podID := "pod-unknown"
		if len(logLine) > 10 {
			podID = "pod-a1"
		}

		blastKey := fmt.Sprintf("blast:%s", incidentID)
		rdb.SAdd(ctx, blastKey, podID)
		rdb.Expire(ctx, blastKey, 10*time.Minute)

		blastRadius, _ := rdb.SCard(ctx, blastKey).Result()
		if blastRadius == 0 {
			blastRadius = 1
		}

		clusterRPSStr, errC := rdb.Get(ctx, "metrics:cluster_rps").Result()
		podRPSStr, errP := rdb.Get(ctx, "metrics:pod_rps:"+podID).Result()

		var clusterRPS, podRPS float64
		if errC != nil {
			clusterRPS = 1000
		} else {
			fmt.Sscanf(clusterRPSStr, "%f", &clusterRPS)
		}
		if errP != nil {
			podRPS = 100
		} else {
			fmt.Sscanf(podRPSStr, "%f", &podRPS)
		}

		capacityLoss := (podRPS / clusterRPS) * 100
		historySuccess := 0.95
		trustScore := (float64(similarity) * historySuccess) / float64(blastRadius)
		if similarity == 0 {
			trustScore = 1.0 / float64(blastRadius)
		}

		if similarity > 0.98 {
			fmt.Printf("DUPLICATE | Trust: %.2f | Blast: %d | Loss: %.1f%%\n", trustScore, blastRadius, capacityLoss)
			return embedding
		}

		_, err = qClient.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "incident_vectors",
			Points: []*qdrant.PointStruct{
				{
					Id:      qdrant.NewID(qID),
					Vectors: qdrant.NewVectors(embedding...),
					Payload: qdrant.NewValueMap(map[string]any{
						"log":           logLine,
						"created_at":    time.Now().Format(time.RFC3339),
						"capacity_loss": capacityLoss,
						"trust_score":   trustScore,
						"blast_radius":  blastRadius,
					}),
				},
			},
		})

		if err == nil {
			fmt.Printf("NEW EVENT | Loss: %.1f%% | Trust: %.2f | ID: %s\n", capacityLoss, trustScore, qID)
		}
	}
	return embedding
}

func worker(ctx context.Context, logChan <-chan string, client pb.InferenceEngineClient, rdb *redis.Client, qClient *qdrant.Client, cache *ResilienceCache) {
	for logLine := range logChan {
		if cachedEmbedding, found := cache.GetEmbedding(logLine); found {
			fmt.Printf("[CACHE HIT] Skipping AI Engine for: %s\n", logLine)
			processWithAI(ctx, client, rdb, qClient, logLine, cachedEmbedding)
			continue
		}
		newEmbedding := processWithAI(ctx, client, rdb, qClient, logLine, nil)
		if newEmbedding != nil {
			cache.Add(logLine, newEmbedding)
		}
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

func ptrUint64(u uint64) *uint64 {
	return &u
}
package main

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"math"
	"os"
	"regexp"
	"strings"
	"time"

	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"github.com/twmb/franz-go/pkg/kgo"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"

	pb "github.com/wrich/sre-pilot/proto"
)

var podRegex = regexp.MustCompile(`SOURCE=([^\s]+)`)

func main() {
	ctx := context.Background()
	slackURL := os.Getenv("SLACK_WEBHOOK_URL")

	if slackURL == "" {
		log.Println("[Warning] SLACK_WEBHOOK_URL not found. Alerts will be local-only.")
	} else {
		log.Println("[System] Slack Online.")
	}

	traceDB := InitTraceDB()
	defer traceDB.Close()
	

	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
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

	investigator := NewInvestigator(rdb)

	qClient, err := qdrant.NewClient(&qdrant.Config{
		Host:   "localhost",
		Port:   6334,
		UseTLS: false,
	})
	if err != nil {
		log.Fatalf("Failed to connect to Qdrant: %v", err)
	}
	defer qClient.Close()
	go StartAPI(traceDB, qClient)

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
		kgo.ConsumeResetOffset(kgo.NewOffset().AtEnd()),
	)
	if err != nil {
		log.Fatalf("Failed to connect to Redpanda: %v", err)
	}
	defer kafkaClient.Close()

	cache := NewResilienceCache(5000)
	logChan := make(chan string, 100)

	for i := 0; i < 16; i++ {
		go worker(ctx, logChan, inferenceClient, rdb, qClient, traceDB, cache, slackURL, investigator)
	}

	fmt.Println("Orchestrator online... Everything connected.")

	for {
		fetches := kafkaClient.PollFetches(ctx)
		if fetches.IsClientClosed() {
			break
		}

		if fetches.Empty() {
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

func ExtractPodID(logLine string) string {
	matches := podRegex.FindStringSubmatch(logLine)
	if len(matches) > 1 {
		return matches[1]
	}
	return "pod-unknown"
}

func processWithAI(
	ctx context.Context,
	client pb.InferenceEngineClient,
	rdb *redis.Client,
	qClient *qdrant.Client,
	traceDB *sql.DB,
	logLine string,
	embedding []float32,
	inX, inY, inZ float32,
	slackURL string,
	investigator *Investigator,
) ([]float32, float32, float32, float32) {
	var x, y, z float32 = inX, inY, inZ

	if embedding == nil {
		ctxIn, cancel := context.WithTimeout(ctx, 2*time.Second)
		defer cancel()

		resp, err := client.RunInference(ctxIn, &pb.InferenceRequest{
			ModelId: "embedder",
			LogLine: logLine,
		})

		if err != nil {
			log.Printf("AI Engine Error: %v", err)
			return nil, 0, 0, 0
		}
		embedding = resp.Embedding
		x, y, z = resp.X, resp.Y, resp.Z
		fmt.Printf("[Spatial] Log Mapped to: X:%.2f, Y:%.2f, Z:%.2f\n", x, y, z)
	}

	if len(embedding) < 384 {
		return nil, 0, 0, 0
	}

	incidentID := generateIncidentHash(embedding)
	qID := fmt.Sprintf("%s-%s-%s-%s-%s",
		incidentID[0:8], incidentID[8:12], incidentID[12:16], incidentID[16:20], incidentID[20:32],
	)

	searchRes, err := qClient.Query(ctx, &qdrant.QueryPoints{
		CollectionName: "incident_vectors",
		Query:          qdrant.NewQuery(embedding...),
		Limit:          ptrUint64(1),
		WithPayload:    qdrant.NewWithPayload(true),
	})

	var similarity float32
	if err == nil && len(searchRes) > 0 {
		similarity = searchRes[0].Score
	}

	podID := ExtractPodID(logLine)
	blastKey := fmt.Sprintf("blast:%s", qID)
	rdb.SAdd(ctx, blastKey, podID)
	rdb.Expire(ctx, blastKey, 10*time.Minute)

	blastRadius, _ := rdb.SCard(ctx, blastKey).Result()
	if blastRadius == 0 {
		blastRadius = 1
	}

	var clusterRPS float64 = 1000
	var podRPS float64 = 100
	if val, err := rdb.Get(ctx, "metrics:cluster_rps").Float64(); err == nil {
		clusterRPS = val
	}
	if val, err := rdb.Get(ctx, "metrics:pod_rps:"+podID).Float64(); err == nil {
		podRPS = val
	}

	capacityLoss := (podRPS / clusterRPS) * 100
	trustScore := (float64(similarity) * 0.95) / float64(blastRadius)
	if similarity == 0 {
		trustScore = 1.0 / float64(blastRadius)
	}
	level := ""
	upperLog := strings.ToUpper(logLine)

    if strings.Contains(upperLog, "FATAL") {		
	    level = "FATAL"
    } else if strings.Contains(upperLog, "ERROR") {
	    level = "ERROR"
    } else if strings.Contains(upperLog, "WARN") {
	    level = "WARN"
    } else if strings.Contains(upperLog, "INFO") {
	    level = "INFO"
    } else if strings.Contains(upperLog, "DEBUG") {
	    level = "DEBUG"
    } else {
	    level = "INFO"
	}


	_, err = qClient.Upsert(ctx, &qdrant.UpsertPoints{
		CollectionName: "incident_vectors",
		Points: []*qdrant.PointStruct{
			{
				Id:      qdrant.NewID(qID),
				Vectors: qdrant.NewVectors(embedding...),
				Payload: qdrant.NewValueMap(map[string]any{
					"log":           logLine,
					"level":         level,
					"created_at":    time.Now().Format(time.RFC3339),
					"capacity_loss": capacityLoss,
					"trust_score":   trustScore,
					"blast_radius":  blastRadius,
					"x":             x,
					"y":             y,
					"z":             z,

				}),
			},
		},
	})

	if err == nil {
		fmt.Printf("UPSERTED | ID: %s\n", qID)
	}

	throttleKey := "throttle:" + qID
	isNewAlert, _ := rdb.SetNX(ctx, throttleKey, "active", 5*time.Minute).Result()
	if !isNewAlert {
		fmt.Printf("Throttled Alert for %s (ID: %s)\n", logLine, qID)
		return embedding, x, y, z
	}

	if strings.Contains(logLine, "LEVEL=ERROR") || strings.Contains(logLine, "LEVEL=FATAL") || (similarity >= 0.85 && similarity <= 0.98) {
		fmt.Printf("ASYNC_TRIGGER | Dispatching agent for %s...\n", podID)
		go func() {
			trace := investigator.Run(qID, podID)
			stepsJSON, _ := json.Marshal(trace.Steps)
			fmt.Printf("[SUCCESS] Agent finished for %s: %s\n", podID, trace.Conclusion)
			finalStatus := "AI_ANALYZED"
			if strings.Contains(strings.ToUpper(trace.Conclusion), "FIXED") {

				finalStatus = "AGENT_FIXED"
			}
			RecordIncident(traceDB, qID, logLine, trustScore, int(blastRadius), capacityLoss, finalStatus, string(stepsJSON), trace.MTTR_ms, 0)
			SendSlackAlert(slackURL, logLine, trustScore, int(blastRadius), trace.Conclusion)
		}()
		return embedding, x, y, z
	} else if similarity > 0.99 {
		fmt.Printf("DUPLICATE | Trust: %.2f | Blast: %d | Loss: %.1f%%\n", trustScore, blastRadius, capacityLoss)
		RecordIncident(traceDB, qID, logLine, trustScore, int(blastRadius), capacityLoss, "CACHED_DUPLICATE", "[]", 0, 0)
		go SendSlackAlert(slackURL, logLine, trustScore, int(blastRadius), "DUPLICATE_ALERT")
		return embedding, x, y, z
	}

	fmt.Printf("NEW EVENT | Loss: %.1f%% | Trust: %.2f | ID: %s\n", capacityLoss, trustScore, qID)
	RecordIncident(traceDB, qID, logLine, trustScore, int(blastRadius), capacityLoss, "AI_PROCESSED", "[]", 0, 0)
	go SendSlackAlert(slackURL, logLine, trustScore, int(blastRadius), "NEW_INCIDENT_DETECTED")

	return embedding, x, y, z
}

func worker(
	ctx context.Context,
	logChan <-chan string,
	client pb.InferenceEngineClient,
	rdb *redis.Client,
	qClient *qdrant.Client,
	traceDB *sql.DB,
	cache *ResilienceCache,
	slackURL string,
	investigator *Investigator,
) {
	for logLine := range logChan {
		if cachedEmbedding, cx, cy, cz, found := cache.GetEmbedding(logLine); found {
			fmt.Printf("[CACHE HIT] Using stored spatial data for: %s\n", logLine)
			processWithAI(ctx, client, rdb, qClient, traceDB, logLine, cachedEmbedding, cx, cy, cz, slackURL, investigator)
			continue
		}

		newEmbedding, nx, ny, nz := processWithAI(ctx, client, rdb, qClient, traceDB, logLine, nil, 0, 0, 0, slackURL, investigator)
		if newEmbedding != nil {
			cache.Add(logLine, newEmbedding, nx, ny, nz)
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
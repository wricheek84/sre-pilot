package main

import (
	"bufio"
	"context"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"log"
	"math"
	"os"
	"strings"

	"github.com/qdrant/go-client/qdrant"
	pb "github.com/wrich/sre-pilot/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

func main() {
	traceDB := InitTraceDB()
	defer traceDB.Close()
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		log.Fatalf("C++ Engine Offline: %v", err)
	}
	defer conn.Close()
	client := pb.NewInferenceEngineClient(conn)

	qClient, err := qdrant.NewClient(&qdrant.Config{Host: "localhost", Port: 6334, UseTLS: false})
	if err != nil {
		log.Fatalf("Qdrant Offline: %v", err)
	}
	defer qClient.Close()

	file, err := os.Open("seed_logs.txt")
	if err != nil {
		log.Fatal("seed_logs.txt not found.")
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	ctx := context.Background()
	count := 0

	fmt.Println("Starting Qdrant insertions from seed_logs.txt")

	for scanner.Scan() {
		logLine := scanner.Text()

		level := "INFO"
		if strings.Contains(logLine, "FATAL") || strings.Contains(logLine, "ERROR") {
			level = "FATAL"

		} else if strings.Contains(logLine, "WARN") {
			level = "WARN"
		}

		resp, err := client.RunInference(ctx, &pb.InferenceRequest{
			ModelId: "embedder",
			LogLine: logLine,
		})

		if err != nil {
			fmt.Printf("Inference Error at line %d: %v\n", count, err)
			continue
		}
		incidentID := generateSeedHash(resp.Embedding)
		qID := fmt.Sprintf("%s-%s-%s-%s-%s",
			incidentID[0:8], incidentID[8:12], incidentID[12:16], incidentID[16:20], incidentID[20:32],
		)

		_, err = qClient.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "incident_vectors",
			Points: []*qdrant.PointStruct{
				{
					Id:      qdrant.NewID(qID),
					Vectors: qdrant.NewVectors(resp.Embedding...),
					Payload: qdrant.NewValueMap(map[string]any{
						"log":   logLine,
						"level": level,
						"x":     float64(resp.X),
						"y":     float64(resp.Y),
						"z":     float64(resp.Z),
					}),
				},
			},
		})
		RecordIncident(traceDB, qID, logLine, 0.99, 1, 0, "SEEDED_LOG", "[]", 0, 0)

		if err != nil {
			log.Fatalf("Qdrant Upsert Failed at count %d: %v", count, err)
		}

		count++
		if count%100 == 0 {
			fmt.Printf("Successfully saved %d logs...\n", count)
		}
	}
	fmt.Printf("Done: %d logs are physically in Qdrant.\n", count)
}
func generateSeedHash(embedding []float32) string {
	buf := make([]byte, len(embedding)*4)
	for i, f := range embedding {
		bits := math.Float32bits(f)
		binary.LittleEndian.PutUint32(buf[i*4:], bits)
	}
	hash := sha256.Sum256(buf)
	return hex.EncodeToString(hash[:])
}

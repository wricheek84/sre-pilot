package main

import (
	"bufio"
	"context"
	"fmt"
	"log"
	"os"

	"github.com/qdrant/go-client/qdrant"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	pb "github.com/wrich/sre-pilot/proto"
)

func main() {
	conn, err := grpc.Dial("localhost:50051", grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil { log.Fatalf("C++ Engine Offline: %v", err) }
	defer conn.Close()
	client := pb.NewInferenceEngineClient(conn)

	qClient, err := qdrant.NewClient(&qdrant.Config{Host: "localhost", Port: 6334, UseTLS: false})
	if err != nil { log.Fatalf("Qdrant Offline: %v", err) }
	defer qClient.Close()

	file, err := os.Open("seed_logs.txt")
	if err != nil { log.Fatal("seed_logs.txt not found.") }
	defer file.Close()

	scanner := bufio.NewScanner(file)
	ctx := context.Background()
	count := 0

	fmt.Println("Starting Qdrant Hydration...")

	for scanner.Scan() {
		logLine := scanner.Text()
		
		resp, err := client.RunInference(ctx, &pb.InferenceRequest{
			ModelId: "embedder", 
			LogLine: logLine,
		})
		if err != nil {
			fmt.Printf("Inference Error at line %d: %v\n", count, err)
			continue 
		}
		
		// FIXED: Use NewIDNum with an integer instead of a string UUID
		_, err = qClient.Upsert(ctx, &qdrant.UpsertPoints{
			CollectionName: "incident_vectors",
			Points: []*qdrant.PointStruct{
				{
					Id:      qdrant.NewIDNum(uint64(count)), 
					Vectors: qdrant.NewVectors(resp.Embedding...),
					Payload: qdrant.NewValueMap(map[string]any{"log": logLine}),
				},
			},
		})
		
		if err != nil {
			log.Fatalf("Qdrant Upsert Failed at count %d: %v", count, err)
		}

		count++
		if count%100 == 0 { fmt.Printf("Successfully saved %d logs...\n", count) }
	}
	fmt.Printf("Done: %d logs are physically in Qdrant.\n", count)
}
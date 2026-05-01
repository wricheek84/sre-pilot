package main

import (
	"bytes"
	"encoding/binary"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
)

type ScrollResponse struct {
	Result struct {
		Points []struct {
			Vector []float32 `json:"vector"`
		} `json:"points"`
	} `json:"result"`
}

func main() {
	// 1. Request the data via the REST API (Port 6333)
	url := "http://localhost:6333/collections/incident_vectors/points/scroll"
	query := []byte(`{"limit": 2000, "with_vector": true}`)
	
	resp, err := http.Post(url, "application/json", bytes.NewBuffer(query))
	if err != nil {
		fmt.Printf("HTTP Error: %v\n", err)
		return
	}
	defer resp.Body.Close()

	// 2. Parse the JSON
	var data ScrollResponse
	if err := json.NewDecoder(resp.Body).Decode(&data); err != nil {
		fmt.Printf("JSON Error: %v\n", err)
		return
	}

	// 3. Write to binary file
	f, _ := os.Create("logs.bin")
	defer f.Close()

	count := 0
	for _, p := range data.Result.Points {
		if len(p.Vector) > 0 {
			binary.Write(f, binary.LittleEndian, p.Vector)
			count++
		}
	}

	fmt.Printf("Success: Dumped %d vectors to logs.bin\n", count)
}
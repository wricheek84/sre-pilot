package main

import (
	"fmt"
	"log"       
	"math/rand"
	"net/http"
	"os"
	"time"
)

func main() {
	
	rand.Seed(time.Now().UnixNano())

	http.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintf(w, "I'm alive!")
	})

	http.HandleFunc("/chaos", func(w http.ResponseWriter, r *http.Request) {
		randomNum := rand.Intn(3) + 1
		
		switch randomNum {
		case 1:
			fmt.Println("CRITICAL: Hard Crash triggered.")
			os.Exit(1)
		case 2:
			fmt.Println("LOAD: Simulating CPU spike...")
			
			sum := 0
			for i := 0; i < 500000000; i++ {
				sum += i
			}
			fmt.Fprintf(w, "Spike complete. Calculated sum: %d", sum)
		case 3:
			fmt.Println("ERROR: Simulating 500 Internal Server Error.")
			http.Error(w, "Custom SRE Simulation Error", http.StatusInternalServerError)
		}
	})

	fmt.Println("Server starting on port 8080...")
	
	
	log.Fatal(http.ListenAndServe(":8080", nil))
}
package main

import (
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"
)

type APIIncident struct {
	ID          int     `json:"id"`
	IncidentID  string  `json:"incident_id"`
	LogLine     string  `json:"log_line"`
	TrustScore  float64 `json:"trust_score"`
	BlastRadius int     `json:"blast_radius"`
	ActionTaken string  `json:"action_taken"`
	Steps       string  `json:"steps"`
	MTTR        int     `json:"mttr"`
}

func StartAPI(db *sql.DB) {
	mux := http.NewServeMux()

	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" { return }

		rows, err := db.Query("SELECT id, incident_id, log_line, trust_score, blast_radius, action_taken, steps, mttr FROM audit_logs ORDER BY id DESC LIMIT 50")
		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		var incidents []APIIncident
		for rows.Next() {
			var inc APIIncident
			rows.Scan(&inc.ID, &inc.IncidentID, &inc.LogLine, &inc.TrustScore, &inc.BlastRadius, &inc.ActionTaken, &inc.Steps, &inc.MTTR)
			incidents = append(incidents, inc)
		}
		
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(incidents)
	})

	mux.HandleFunc("/incident/", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" { return }

		id := strings.TrimPrefix(r.URL.Path, "/incident/")
		var inc APIIncident
		err := db.QueryRow("SELECT id, incident_id, log_line, trust_score, blast_radius, action_taken, steps, mttr FROM audit_logs WHERE incident_id = ?", id).
			Scan(&inc.ID, &inc.IncidentID, &inc.LogLine, &inc.TrustScore, &inc.BlastRadius, &inc.ActionTaken, &inc.Steps, &inc.MTTR)
		
		if err != nil {
			http.Error(w, "Incident Not Found", http.StatusNotFound)
			return
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(inc)
	})

	log.Println("[API] Doorbell online at :8082")
	log.Fatal(http.ListenAndServe(":8082", mux))
}

func enableCORS(w http.ResponseWriter) {
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}
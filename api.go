package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"log"
	"net/http"
	"strings"

	"github.com/qdrant/go-client/qdrant"
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

func StartAPI(db *sql.DB, qClient *qdrant.Client) {
	mux := http.NewServeMux()

	mux.HandleFunc("/incidents", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			return
		}

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

	mux.HandleFunc("/spatial", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			return
		}

		ctx := context.Background()
		res, err := qClient.Scroll(ctx, &qdrant.ScrollPoints{
			CollectionName: "incident_vectors",
			Limit:          ptrUint32(2200),
			WithPayload:    qdrant.NewWithPayload(true),
			WithVectors:    qdrant.NewWithVectors(false),
		})

		if err != nil {
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}

		var points []map[string]interface{}
		for _, p := range res {
			payload := p.GetPayload()
			if payload == nil {
				continue
			}

			var x, y, z float64
			if val, ok := payload["x"]; ok {
				x = val.GetDoubleValue()
			}
			if val, ok := payload["y"]; ok {
				y = val.GetDoubleValue()
			}
			if val, ok := payload["z"]; ok {
				z = val.GetDoubleValue()
			}
			level := ""
            if val, ok := payload["level"]; ok {
                level = val.GetStringValue()
            }

			points = append(points, map[string]interface{}{
				"id": p.Id.GetUuid(),
				"x":  x,
				"y":  y,
				"z":  z,
				"log": payload["log"].GetStringValue(),
				"level": level,
			})
		}

		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(points)
	})

	mux.HandleFunc("/incident/", func(w http.ResponseWriter, r *http.Request) {
		enableCORS(w)
		if r.Method == "OPTIONS" {
			return
		}

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
	w.Header().Set("Access-Control-Allow-Origin", "http://localhost:5173")
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
}

func ptrUint32(u uint32) *uint32 {
	return &u
}
package main


import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite" // The underscore means we import it silently so database/sql can use it
)

// IncidentTrace represents one row in our database
type IncidentTrace struct {
	ID           int
	LogLine      string
	TrustScore   float64
	BlastRadius  int
	CapacityLoss float64
	ActionTaken  string
}


func InitTraceDB() *sql.DB {
	
	db, err := sql.Open("sqlite", "audit.db")
	if err != nil {
		log.Fatalf("failed to open SQLite database: %v", err)
	}

	
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		log_line TEXT NOT NULL,
		trust_score REAL,
		blast_radius INTEGER,
		capacity_loss REAL,
		action_taken TEXT,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	log.Println("[Database] audit.db online and table verified.")
	return db
}
func RecordIncident(db *sql.DB, logLine string, trustScore float64, blastRadius int, capacityLoss float64, action string) {
	insertSQL := `
	INSERT INTO audit_logs (log_line, trust_score, blast_radius, capacity_loss, action_taken) 
	VALUES (?, ?, ?, ?, ?)`
	
	
	_, err := db.Exec(insertSQL, logLine, trustScore, blastRadius, capacityLoss, action)
	if err != nil {
		log.Printf("[DB Error] Failed to trace incident: %v", err)
	}
}
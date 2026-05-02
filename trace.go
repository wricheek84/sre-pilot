package main


import (
	"database/sql"
	"log"

	_ "modernc.org/sqlite" 
)

// IncidentTrace represents one row in our database
type IncidentTrace struct {
	ID           int
	IncidentID   string
	LogLine      string
	TrustScore   float64
	BlastRadius  int
	CapacityLoss float64
	ActionTaken  string
	Steps        string
	MTTR         int
	MTTD         int
}


func InitTraceDB() *sql.DB {
	
	db, err := sql.Open("sqlite", "audit.db")
	if err != nil {
		log.Fatalf("failed to open SQLite database: %v", err)
	}
	pragmas := `
	PRAGMA journal_mode=WAL;
	PRAGMA busy_timeout=5000;
	PRAGMA synchronous=NORMAL;
	`

	_, err = db.Exec(pragmas)
	if err != nil {
		log.Fatalf("Failed to set SQLite pragmas: %v", err)
	}
	db.SetMaxOpenConns(1)

	createTableSQL := `
	CREATE TABLE IF NOT EXISTS audit_logs (
		id INTEGER PRIMARY KEY AUTOINCREMENT,
		incident_id TEXT,
		log_line TEXT NOT NULL,
		trust_score REAL,
		blast_radius INTEGER,
		capacity_loss REAL,
		action_taken TEXT,
		steps TEXT,
		mttr INTEGER,
		mttd INTEGER,
		created_at DATETIME DEFAULT CURRENT_TIMESTAMP
	);`

	
	_, err = db.Exec(createTableSQL)
	if err != nil {
		log.Fatalf("Failed to create table: %v", err)
	}

	log.Println("[Database] audit.db online and table verified.")
	return db
}
func RecordIncident(db *sql.DB, incidentID string, logLine string, trustScore float64, blastRadius int, capacityLoss float64, action string, steps string, mttr int, mttd int) {
	insertSQL := `
	INSERT INTO audit_logs (incident_id, log_line, trust_score, blast_radius, capacity_loss, action_taken,steps, mttr, mttd) 
	VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?);`
	
	
	_, err := db.Exec(insertSQL, incidentID, logLine, trustScore, blastRadius, capacityLoss, action, steps, mttr, mttd)
	if err != nil {
		log.Printf("[DB Error] Failed to trace incident: %v", err)
	}
}
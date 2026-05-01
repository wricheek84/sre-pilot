package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"
	"text/tabwriter"

	_ "modernc.org/sqlite"
)

func main() {
	db, err := sql.Open("sqlite", "audit.db")
	if err != nil {
		log.Fatalf("Failed to open DB: %v", err)
	}
	defer db.Close()

	
	query := `
	SELECT id, action_taken, trust_score, blast_radius, substr(log_line, 1, 45) 
	FROM audit_logs 
	WHERE action_taken = 'AI_PROCESSED'
	ORDER BY id DESC 
	LIMIT 15;`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to query DB: %v", err)
	}
	defer rows.Close()

	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tACTION TAKEN\tTRUST\tBLAST\tLOG PREVIEW")
	fmt.Fprintln(w, "--\t------------\t-----\t-----\t-----------")

	for rows.Next() {
		var id, blast int
		var action, logPreview string
		var trust float64
		
		if err := rows.Scan(&id, &action, &trust, &blast, &logPreview); err != nil {
			log.Fatal(err)
		}
		fmt.Fprintf(w, "%d\t%s\t%.2f\t%d\t%s...\n", id, action, trust, blast, logPreview)
	}
	w.Flush()
}
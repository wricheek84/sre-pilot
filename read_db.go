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
	SELECT id, incident_id, action_taken, mttr, steps 
	FROM audit_logs 
	ORDER BY id DESC 
	LIMIT 10;`

	rows, err := db.Query(query)
	if err != nil {
		log.Fatalf("Failed to query DB: %v", err)
	}
	defer rows.Close()

	
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 3, ' ', 0)
	fmt.Fprintln(w, "ID\tINCIDENT_ID\tACTION\tMTTR(ms)\tSTEPS_JSON")
	fmt.Fprintln(w, "--\t-----------\t------\t--------\t----------")

	for rows.Next() {
		var id, mttr int
		var incidentID, action, steps string
		
		if err := rows.Scan(&id, &incidentID, &action, &mttr, &steps); err != nil {
			log.Fatal(err)
		}
		
		shortID := incidentID[:8]
		shortSteps := steps
		if len(steps) > 50 {
			shortSteps = steps[:47] + "..."
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%d\t%s...\n", id, shortID, action, mttr, shortSteps)
	}
	w.Flush()
}
package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)


type SlackMessage struct {
	Attachments []Attachment `json:"attachments"`
}

type Attachment struct {
	Color  string  `json:"color"` 
	Title  string  `json:"title"`
	Text   string  `json:"text"`
	Fields []Field `json:"fields"`
	Footer string  `json:"footer"`
}

type Field struct {
	Title string `json:"title"`
	Value string `json:"value"`
	Short bool   `json:"short"`
}


func SendSlackAlert(webhookURL string, logLine string, trust float64, blast int, action string) error {
	
	color := "#36a64f" 
	if action == "AI_PROCESSED" {
		color = "#f2c744" 
	}
	if trust < 0.70 {
		color = "#ff0000" 
	}

	msg := SlackMessage{
		Attachments: []Attachment{
			{
				Color: color,
				Title: " SRE Overwatch: Incident Detected",
				Text:  fmt.Sprintf("```%s```", logLine), 
				Fields: []Field{
					{Title: "Action", Value: action, Short: true},
					{Title: "Trust Score", Value: fmt.Sprintf("%.2f", trust), Short: true},
					{Title: "Blast Radius", Value: fmt.Sprintf("%d Pods", blast), Short: true},
				},
				Footer: fmt.Sprintf("Ryzen-3500U-Node | %s", time.Now().Format(time.Kitchen)),
			},
		},
	}

	payload, _ := json.Marshal(msg)
	resp, err := http.Post(webhookURL, "application/json", bytes.NewBuffer(payload))
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("slack responded with status: %d", resp.StatusCode)
	}

	return nil
}
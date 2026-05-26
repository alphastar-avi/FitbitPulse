package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"
	"strings"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	"encoding/json"
	"os"
)

func main() {
	b, err := os.ReadFile("token.json")
	if err != nil {
		log.Fatalf("Unable to read client secret file: %v", err)
	}

	tok := &oauth2.Token{}
	json.Unmarshal(b, tok)

	config := &oauth2.Config{
		ClientID:     "dummy",
		ClientSecret: "dummy",
		Endpoint:     google.Endpoint,
	}
	client := config.Client(context.Background(), tok)

	testRange := func(days int) {
		startT := time.Now().AddDate(0, 0, -days)
		
		bodyPayload := fmt.Sprintf(`{
			"range": {
				"start": {
					"date": {"year": %d, "month": %d, "day": %d},
					"time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0}
				},
				"end": {
					"date": {"year": %d, "month": %d, "day": %d},
					"time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0}
				}
			}
		}`, startT.Year(), int(startT.Month()), startT.Day(), time.Now().AddDate(0, 0, 1).Year(), int(time.Now().AddDate(0, 0, 1).Month()), time.Now().AddDate(0, 0, 1).Day())

		apiURL := "https://health.googleapis.com/v1/users/me/dataTypes/steps/dataPoints:dailyRollUp"
		req, _ := http.NewRequest("POST", apiURL, strings.NewReader(bodyPayload))
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			log.Fatal(err)
		}
		defer resp.Body.Close()
		bodyBytes, _ := io.ReadAll(resp.Body)
		fmt.Printf("Range: %d days. Code: %d, Response: %s\n", days, resp.StatusCode, string(bodyBytes))
	}
	testRange(90)
	testRange(30)
}

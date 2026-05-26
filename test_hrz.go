package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"net/url"
	"time"

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

	startT := time.Now().AddDate(0, 0, -3)
	startDate := startT.Format("2006-01-02")
	filter := fmt.Sprintf("daily_heart_rate_zones.date >= \"%s\"", startDate)

	u, _ := url.Parse("https://health.googleapis.com/v1/users/me/dataTypes/daily-heart-rate-zones/dataPoints")
	q := u.Query()
	q.Set("filter", filter)
	u.RawQuery = q.Encode()

	resp, err := client.Get(u.String())
	if err != nil {
		log.Fatal(err)
	}
	defer resp.Body.Close()
	bodyBytes, _ := io.ReadAll(resp.Body)
	fmt.Printf("Code: %d\nResponse: %s\n", resp.StatusCode, string(bodyBytes))
}

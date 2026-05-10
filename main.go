package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	tokenFile  = "token.json"
	apiBaseURL = "https://health.googleapis.com/v4"
)

var (
	oauthConfig *oauth2.Config
	oauthState  = "random_state_string_for_security"
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Println("Note: .env file not found or failed to load, reading from environment variables")
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set in .env")
	}

	// Google Health API OAuth2 configuration
	oauthConfig = &oauth2.Config{
		RedirectURL:  "http://localhost:8080/callback",
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Scopes: []string{
			"https://www.googleapis.com/auth/googlehealth.activity_and_fitness.readonly",
			"https://www.googleapis.com/auth/googlehealth.health_metrics_and_measurements.readonly",
			"https://www.googleapis.com/auth/googlehealth.sleep.readonly",
		},
		Endpoint: google.Endpoint,
	}

	http.HandleFunc("/", handleHome)
	http.HandleFunc("/login", handleLogin)
	http.HandleFunc("/callback", handleCallback)
	http.HandleFunc("/api/raw", handleRawData)

	fmt.Println("Server listening on http://localhost:8080")
	
	// Check if we already have a saved token
	if _, err := tokenFromFile(tokenFile); err == nil {
		fmt.Println("Token found! You are already authenticated. You can view raw data at:")
		fmt.Println(" - http://localhost:8080/api/raw?type=steps")
		fmt.Println(" - http://localhost:8080/api/raw?type=sleep")
	} else {
		fmt.Println("Please visit http://localhost:8080/login to authenticate with Google Health (Fitbit)")
	}

	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		fmt.Fprintf(w, "Welcome to FitbitPulse! You are not authenticated yet. <a href='/login'>Authorize with Google Health</a>")
		return
	}

	w.Header().Set("Content-Type", "text/html")
	fmt.Fprintf(w, `
		<h1>FitbitPulse Dashboard (Backend Running)</h1>
		<p>Successfully authenticated!</p>
		<ul>
			<li><a href="/api/raw?type=steps">View Steps Raw Data</a></li>
			<li><a href="/api/raw?type=sleep">View Sleep Raw Data</a></li>
			<li><a href="/api/raw?type=daily-resting-heart-rate">View Resting Heart Rate Raw Data</a></li>
		</ul>
		<p>Next steps: We will use this raw data to build a premium visual dashboard!</p>
		<p><em>Token expires at: %s</em></p>
	`, tok.Expiry.Format(time.RFC822))
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// Request offline access to ensure we get a Refresh Token
	url := oauthConfig.AuthCodeURL(oauthState, oauth2.AccessTypeOffline, oauth2.ApprovalForce)
	http.Redirect(w, r, url, http.StatusTemporaryRedirect)
}

func handleCallback(w http.ResponseWriter, r *http.Request) {
	state := r.FormValue("state")
	if state != oauthState {
		http.Error(w, "Invalid state", http.StatusBadRequest)
		return
	}

	code := r.FormValue("code")
	token, err := oauthConfig.Exchange(r.Context(), code)
	if err != nil {
		http.Error(w, fmt.Sprintf("Code exchange failed: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Save the token to a file
	err = saveToken(tokenFile, token)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to save token: %s", err.Error()), http.StatusInternalServerError)
		return
	}

	// Redirect to home page
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func handleRawData(w http.ResponseWriter, r *http.Request) {
	// CORS setup
	w.Header().Set("Access-Control-Allow-Origin", "*")
	if r.Method == http.MethodOptions {
		w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS, POST")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		w.WriteHeader(http.StatusOK)
		return
	}

	dataType := r.URL.Query().Get("type")
	if dataType == "" {
		http.Error(w, "Missing 'type' parameter (e.g. ?type=steps)", http.StatusBadRequest)
		return
	}

	tok, err := tokenFromFile(tokenFile)
	if err != nil {
		http.Error(w, "Not authenticated. Go to /login first.", http.StatusUnauthorized)
		return
	}

	client := oauthConfig.Client(r.Context(), tok)
	filterName := strings.ReplaceAll(dataType, "-", "_")

	var finalResponse map[string]interface{}

	if dataType == "steps" {
		var allPoints []interface{}
		var lastRespObj map[string]interface{}
		
		// 90 days in 3 chunks of 30 days to bypass the 33-day limit on dailyRollUp
		for i := 2; i >= 0; i-- {
			startT := time.Now().AddDate(0, 0, -30*(i+1))
			endT := time.Now().AddDate(0, 0, -30*i)
			if i == 0 {
				endT = time.Now().AddDate(0, 0, 1) // up to tomorrow
			}

			apiURL := fmt.Sprintf("%s/users/me/dataTypes/steps/dataPoints:dailyRollUp", apiBaseURL)
			bodyPayload := fmt.Sprintf(`{
				"range": {
					"start": { "date": {"year": %d, "month": %d, "day": %d}, "time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0} },
					"end": { "date": {"year": %d, "month": %d, "day": %d}, "time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0} }
				}
			}`, startT.Year(), int(startT.Month()), startT.Day(), endT.Year(), int(endT.Month()), endT.Day())

			req, _ := http.NewRequest("POST", apiURL, strings.NewReader(bodyPayload))
			req.Header.Set("Content-Type", "application/json")
			resp, err := client.Do(req)
			if err != nil {
				http.Error(w, "API error", http.StatusInternalServerError)
				return
			}
			var chunk map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&chunk)
			resp.Body.Close()

			if resp.StatusCode == 200 {
				if pts, ok := chunk["rollupDataPoints"].([]interface{}); ok {
					allPoints = append(allPoints, pts...)
				}
				lastRespObj = chunk
			} else {
				if lastRespObj == nil { lastRespObj = chunk }
			}
		}
		if lastRespObj != nil {
			lastRespObj["rollupDataPoints"] = allPoints
			finalResponse = lastRespObj
		}
	} else {
		// 90 days for other endpoints, handle pagination
		startT := time.Now().AddDate(0, 0, -90)
		startTime := startT.Format(time.RFC3339)
		
		var filter string
		if dataType == "sleep" {
			filter = fmt.Sprintf("sleep.interval.end_time >= \"%s\"", startTime)
		} else if strings.HasPrefix(dataType, "daily-") {
			startDate := startT.Format("2006-01-02")
			filter = fmt.Sprintf("%s.date >= \"%s\"", filterName, startDate)
		} else {
			filter = fmt.Sprintf("%s.interval.start_time >= \"%s\"", filterName, startTime)
		}

		u, _ := url.Parse(fmt.Sprintf("%s/users/me/dataTypes/%s/dataPoints", apiBaseURL, dataType))
		q := u.Query()
		q.Set("filter", filter)
		
		var allPoints []interface{}
		var lastRespObj map[string]interface{}
		pageToken := ""

		for page := 0; page < 20; page++ { // max 20 pages
			if pageToken != "" {
				q.Set("pageToken", pageToken)
			} else if page > 0 {
				break
			}
			u.RawQuery = q.Encode()

			resp, err := client.Get(u.String())
			if err != nil { break }
			
			var chunk map[string]interface{}
			json.NewDecoder(resp.Body).Decode(&chunk)
			resp.Body.Close()

			if resp.StatusCode == 200 {
				if pts, ok := chunk["dataPoints"].([]interface{}); ok {
					allPoints = append(allPoints, pts...)
				}
				lastRespObj = chunk
				if token, ok := chunk["nextPageToken"].(string); ok && token != "" {
					pageToken = token
				} else {
					break
				}
			} else {
				if lastRespObj == nil { lastRespObj = chunk }
				break
			}
		}

		if lastRespObj != nil {
			lastRespObj["dataPoints"] = allPoints
			delete(lastRespObj, "nextPageToken")
			finalResponse = lastRespObj
		}
	}

	w.Header().Set("Content-Type", "application/json")
	if finalResponse == nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte(`{"error": "Failed to fetch data"}`))
		return
	}
	json.NewEncoder(w).Encode(finalResponse)
}

// Helpers for token persistence

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

func tokenFromFile(path string) (*oauth2.Token, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	tok := &oauth2.Token{}
	err = json.NewDecoder(f).Decode(tok)
	return tok, err
}

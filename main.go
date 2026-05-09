package main

import (
	"encoding/json"
	"fmt"
	"io"
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

// handleRawData fetches raw data points from health.googleapis.com
func handleRawData(w http.ResponseWriter, r *http.Request) {
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

	// Create a client that automatically refreshes the token if expired
	client := oauthConfig.Client(r.Context(), tok)

	// Determine snake_case filter name from kebab-case endpoint type
	filterName := strings.ReplaceAll(dataType, "-", "_")

	// Query for the last 7 days of data
	startTime := time.Now().AddDate(0, 0, -7).Format(time.RFC3339)
	filter := fmt.Sprintf("%s.interval.start_time >= \"%s\"", filterName, startTime)

	// Construct URL with proper encoding
	u, err := url.Parse(fmt.Sprintf("%s/users/me/dataTypes/%s/dataPoints", apiBaseURL, dataType))
	if err != nil {
		http.Error(w, "Failed to parse URL", http.StatusInternalServerError)
		return
	}
	q := u.Query()
	q.Set("filter", filter)
	u.RawQuery = q.Encode()
	apiURL := u.String()

	resp, err := client.Get(apiURL)
	if err != nil {
		http.Error(w, fmt.Sprintf("Failed to fetch from Google Health API: %s", err.Error()), http.StatusInternalServerError)
		return
	}
	defer resp.Body.Close()

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(resp.StatusCode)
	io.Copy(w, resp.Body)
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

package main

import (
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/joho/godotenv"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var (
	oauthConfig *oauth2.Config
	oauthState  = "random_state_string_for_security" // In a real app, generate this randomly per session
)

func main() {
	// Load .env file
	err := godotenv.Load()
	if err != nil {
		log.Fatal("Error loading .env file")
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatal("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set in .env")
	}

	// Google Health API OAuth2 configuration
	// Using generic Google endpoints and health-specific scopes
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

	fmt.Println("Server listening on http://localhost:8080")
	fmt.Println("Visit http://localhost:8080/login to authenticate with Google Health (Fitbit)")
	log.Fatal(http.ListenAndServe(":8080", nil))
}

func handleHome(w http.ResponseWriter, r *http.Request) {
	fmt.Fprintf(w, "Welcome to FitbitPulse! Go to /login to authorize with Google Health.")
}

func handleLogin(w http.ResponseWriter, r *http.Request) {
	// Request offline access so we get a refresh token
	url := oauthConfig.AuthCodeURL(oauthState, oauth2.AccessTypeOffline)
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

	// We have the token!
	fmt.Fprintf(w, "Successfully authenticated!\n\nAccess Token: %s\n\nRefresh Token: %s", token.AccessToken, token.RefreshToken)
	
	// TODO: Save this token and start the background fetching routine using health.googleapis.com
}

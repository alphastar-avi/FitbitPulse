package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/joho/godotenv"
	"github.com/modelcontextprotocol/go-sdk/mcp"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

const (
	apiBaseURL = "https://health.googleapis.com/v4"
)

var (
	oauthConfig *oauth2.Config
	tokenPath   string
)

// savingTokenSource wraps an oauth2.TokenSource to write refreshed tokens to disk.
type savingTokenSource struct {
	source    oauth2.TokenSource
	tokenFile string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	// Try to compare with existing token on disk to prevent redundant writes
	existing, err := tokenFromFile(s.tokenFile)
	if err != nil || existing.AccessToken != tok.AccessToken {
		log.Printf("[MCP] Token refreshed, saving to %s\n", s.tokenFile)
		if errSave := saveToken(s.tokenFile, tok); errSave != nil {
			log.Printf("[MCP] Failed to save token: %v\n", errSave)
		}
	}
	return tok, nil
}

func main() {
	// Send logs to stderr so stdout remains clean for MCP JSON-RPC protocol
	log.SetOutput(os.Stderr)
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Locate .env and token.json
	envFile := findFile(".env")
	tokenPath = findFile("token.json")

	if err := godotenv.Load(envFile); err != nil {
		log.Printf("[MCP] Note: .env not loaded or not found: %v. Reading from environment variables.", err)
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatalf("[MCP] GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set in your environment or .env file.")
	}

	// OAuth2 config
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

	// Create MCP Server
	server := mcp.NewServer(&mcp.Implementation{
		Name:    "FitbitPulse-MCP",
		Version: "1.0.0",
	}, nil)

	// Define Input Arguments Struct
	type DaysArg struct {
		Days int `json:"days,omitempty" jsonschema:"Number of days of data to retrieve (1-90, default 30)"`
	}

	// 1. Tool: get_steps
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_steps",
		Description: "Retrieve daily steps count rollup from Fitbit for the specified period (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchSteps(ctx, client, args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 2. Tool: get_sleep_sessions
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_sleep_sessions",
		Description: "Retrieve nightly sleep sessions, total durations, and sleep stages (deep, rem, light, awake) (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchGenericData(ctx, client, "sleep", args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 3. Tool: get_resting_heart_rate
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_resting_heart_rate",
		Description: "Retrieve daily resting heart rate metrics (beats per minute) recorded during sleep (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchGenericData(ctx, client, "daily-resting-heart-rate", args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 4. Tool: get_hrv
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_hrv",
		Description: "Retrieve heart rate variability (HRV) metrics including average milliseconds, RMSSD, and entropy (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchGenericData(ctx, client, "daily-heart-rate-variability", args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 5. Tool: get_breathing_rate
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_breathing_rate",
		Description: "Retrieve nightly average respiration rate (breaths per minute) (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchGenericData(ctx, client, "daily-respiratory-rate", args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 6. Tool: get_wrist_temperature
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_wrist_temperature",
		Description: "Retrieve nightly skin temperature variations compared against a 30-day baseline (max 90 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		data, err := fetchGenericData(ctx, client, "daily-sleep-temperature-derivations", args.Days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 7. Tool: get_blood_oxygen
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_blood_oxygen",
		Description: "Retrieve nightly oxygen saturation levels (SpO2) average and range (lower/upper bound) (max 7 days).",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		// Blood oxygen has a strict 7-day limit on standard queries
		days := args.Days
		if days <= 0 || days > 7 {
			days = 7
		}
		data, err := fetchGenericData(ctx, client, "daily-oxygen-saturation", days)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(data), nil, nil
	})

	// 8. Tool: get_activity_metrics
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_activity_metrics",
		Description: "Retrieve multi-dimensional activity logs including active minutes, active zone minutes, distance (meters), and sedentary periods.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args DaysArg) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		days := args.Days
		if days <= 0 {
			days = 30
		}
		
		results := make(map[string]interface{})
		
		if dist, err := fetchGenericData(ctx, client, "distance", days); err == nil {
			results["distance"] = dist["dataPoints"]
		}
		if actMin, err := fetchGenericData(ctx, client, "active-minutes", days); err == nil {
			results["active_minutes"] = actMin["dataPoints"]
		}
		if actZones, err := fetchGenericData(ctx, client, "active-zone-minutes", days); err == nil {
			results["active_zone_minutes"] = actZones["dataPoints"]
		}
		if sed, err := fetchGenericData(ctx, client, "sedentary-period", days); err == nil {
			results["sedentary_periods"] = sed["dataPoints"]
		}

		return jsonResult(results), nil, nil
	})

	// 9. Tool: get_dashboard_summary
	mcp.AddTool(server, &mcp.Tool{
		Name:        "get_dashboard_summary",
		Description: "Retrieve a quick summary of averages (steps, heart rate, sleep duration, HRV, breathing rate, temp, SpO2, distance, sedentary) over the last 30 days.",
	}, func(ctx context.Context, req *mcp.CallToolRequest, args struct{}) (*mcp.CallToolResult, any, error) {
		client, err := getClient(ctx)
		if err != nil {
			return errorResult(err), nil, nil
		}
		
		summary, err := calculateDashboardSummary(ctx, client)
		if err != nil {
			return errorResult(err), nil, nil
		}
		return jsonResult(summary), nil, nil
	})

	// Run Stdio Transport Server
	log.Printf("[MCP] Starting FitbitPulse MCP Server...")
	if err := server.Run(context.Background(), &mcp.StdioTransport{}); err != nil {
		log.Fatalf("[MCP] Server failure: %v", err)
	}
}

// Helpers for Client & Credentials

func findFile(name string) string {
	if _, err := os.Stat(name); err == nil {
		return name
	}
	parentPath := filepath.Join("..", name)
	if _, err := os.Stat(parentPath); err == nil {
		return parentPath
	}
	return name
}

func getClient(ctx context.Context) (*http.Client, error) {
	tok, err := tokenFromFile(tokenPath)
	if err != nil {
		return nil, fmt.Errorf("authentication required. Go to http://localhost:8080/login first to authorize access and create the token.json file: %w", err)
	}

	// Create reusable token source that intercepts refresh to save it back to token.json
	baseSource := oauthConfig.TokenSource(ctx, tok)
	savingSource := &savingTokenSource{
		source:    baseSource,
		tokenFile: tokenPath,
	}

	return oauth2.NewClient(ctx, savingSource), nil
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

func saveToken(path string, token *oauth2.Token) error {
	f, err := os.OpenFile(path, os.O_RDWR|os.O_CREATE|os.O_TRUNC, 0600)
	if err != nil {
		return err
	}
	defer f.Close()
	return json.NewEncoder(f).Encode(token)
}

// Fetching steps (with chunking)
func fetchSteps(ctx context.Context, client *http.Client, days int) (map[string]interface{}, error) {
	if days <= 0 {
		days = 30
	}
	if days > 90 {
		days = 90
	}

	var allPoints []interface{}
	var lastRespObj map[string]interface{}

	chunkSize := 30
	numChunks := (days + chunkSize - 1) / chunkSize

	for i := numChunks - 1; i >= 0; i-- {
		startOffset := -chunkSize * (i + 1)
		endOffset := -chunkSize * i
		if i == numChunks-1 && days%chunkSize != 0 {
			startOffset = -days
		}

		startT := time.Now().AddDate(0, 0, startOffset)
		endT := time.Now().AddDate(0, 0, endOffset)
		if i == 0 {
			endT = time.Now().AddDate(0, 0, 1)
		}

		apiURL := fmt.Sprintf("%s/users/me/dataTypes/steps/dataPoints:dailyRollUp", apiBaseURL)
		bodyPayload := fmt.Sprintf(`{
			"range": {
				"start": { "date": {"year": %d, "month": %d, "day": %d}, "time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0} },
				"end": { "date": {"year": %d, "month": %d, "day": %d}, "time": {"hours": 0, "minutes": 0, "seconds": 0, "nanos": 0} }
			},
			"windowSizeDays": 1
		}`, startT.Year(), int(startT.Month()), startT.Day(), endT.Year(), int(endT.Month()), endT.Day())

		req, err := http.NewRequestWithContext(ctx, "POST", apiURL, strings.NewReader(bodyPayload))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		var chunk map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&chunk)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

		if resp.StatusCode == 200 {
			if pts, ok := chunk["rollupDataPoints"].([]interface{}); ok {
				allPoints = append(allPoints, pts...)
			}
			lastRespObj = chunk
		} else {
			if lastRespObj == nil {
				lastRespObj = chunk
			}
		}
	}

	if lastRespObj == nil {
		return nil, fmt.Errorf("failed to fetch steps data")
	}

	lastRespObj["rollupDataPoints"] = allPoints
	return lastRespObj, nil
}

// Fetch generic data points
func fetchGenericData(ctx context.Context, client *http.Client, dataType string, days int) (map[string]interface{}, error) {
	if days <= 0 {
		if dataType == "daily-oxygen-saturation" {
			days = 7
		} else {
			days = 30
		}
	}
	if days > 90 {
		days = 90
	}

	startT := time.Now().AddDate(0, 0, -days)
	startTime := startT.Format(time.RFC3339)
	filterName := strings.ReplaceAll(dataType, "-", "_")

	var filter string
	if dataType == "sleep" {
		filter = fmt.Sprintf("sleep.interval.end_time >= \"%s\"", startTime)
	} else if strings.HasPrefix(dataType, "daily-") {
		startDate := startT.Format("2006-01-02")
		filter = fmt.Sprintf("%s.date >= \"%s\"", filterName, startDate)
	} else if dataType == "oxygen-saturation" {
		filter = fmt.Sprintf("oxygen_saturation.sample_time.physical_time >= \"%s\"", startTime)
	} else {
		filter = fmt.Sprintf("%s.interval.start_time >= \"%s\"", filterName, startTime)
	}

	u, err := url.Parse(fmt.Sprintf("%s/users/me/dataTypes/%s/dataPoints", apiBaseURL, dataType))
	if err != nil {
		return nil, err
	}
	q := u.Query()
	q.Set("filter", filter)

	var allPoints []interface{}
	var lastRespObj map[string]interface{}
	pageToken := ""

	for page := 0; page < 20; page++ {
		if pageToken != "" {
			q.Set("pageToken", pageToken)
		} else if page > 0 {
			break
		}
		u.RawQuery = q.Encode()

		req, err := http.NewRequestWithContext(ctx, "GET", u.String(), nil)
		if err != nil {
			return nil, err
		}

		resp, err := client.Do(req)
		if err != nil {
			return nil, err
		}

		var chunk map[string]interface{}
		err = json.NewDecoder(resp.Body).Decode(&chunk)
		resp.Body.Close()
		if err != nil {
			return nil, err
		}

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
			if lastRespObj == nil {
				lastRespObj = chunk
			}
			break
		}
	}

	if lastRespObj == nil {
		return nil, fmt.Errorf("failed to fetch data for type %s", dataType)
	}

	lastRespObj["dataPoints"] = allPoints
	delete(lastRespObj, "nextPageToken")
	return lastRespObj, nil
}

// Summary Calculation

type DashboardSummary struct {
	AvgSteps            float64 `json:"avg_steps_per_day"`
	AvgActiveMinutes    float64 `json:"avg_active_minutes_per_day"`
	AvgDistanceMeters   float64 `json:"avg_distance_meters_per_day"`
	AvgRestingHeartRate float64 `json:"avg_resting_heart_rate_bpm"`
	AvgSleepDurationHrs float64 `json:"avg_sleep_duration_hours"`
	AvgHRV              float64 `json:"avg_hrv_ms"`
	AvgSpO2             float64 `json:"avg_spo2_percentage"`
	AvgSedentaryHrs     float64 `json:"avg_sedentary_hours_per_day"`
}

func calculateDashboardSummary(ctx context.Context, client *http.Client) (*DashboardSummary, error) {
	summary := &DashboardSummary{}

	// 1. Steps
	if stepsJ, err := fetchSteps(ctx, client, 30); err == nil {
		if pts, ok := stepsJ["rollupDataPoints"].([]interface{}); ok {
			var total float64
			var count float64
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if stepsField, ok := m["steps"].(map[string]interface{}); ok {
						val := parseNumeric(stepsField["countSum"])
						if val > 0 {
							total += val
							count++
						}
					}
				}
			}
			if count > 0 {
				summary.AvgSteps = total / count
			}
		}
	}

	// 2. Active Minutes
	if actJ, err := fetchGenericData(ctx, client, "active-minutes", 30); err == nil {
		if pts, ok := actJ["dataPoints"].([]interface{}); ok {
			dailyMap := make(map[string]float64)
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if actField, ok := m["activeMinutes"].(map[string]interface{}); ok {
						if start, ok := actField["interval"].(map[string]interface{}); ok {
							if cTime, ok := start["civilStartTime"].(map[string]interface{}); ok {
								if dateM, ok := cTime["date"].(map[string]interface{}); ok {
									dateStr := fmt.Sprintf("%.0f-%.0f-%.0f", parseNumeric(dateM["year"]), parseNumeric(dateM["month"]), parseNumeric(dateM["day"]))
									if levels, ok := actField["activeMinutesByActivityLevel"].([]interface{}); ok {
										var mins float64
										for _, lvl := range levels {
											if lvlMap, ok := lvl.(map[string]interface{}); ok {
												mins += parseNumeric(lvlMap["activeMinutes"])
											}
										}
										dailyMap[dateStr] += mins
									}
								}
							}
						}
					}
				}
			}
			var total float64
			for _, v := range dailyMap {
				total += v
			}
			if len(dailyMap) > 0 {
				summary.AvgActiveMinutes = total / float64(len(dailyMap))
			}
		}
	}

	// 3. Distance (meters)
	if distJ, err := fetchGenericData(ctx, client, "distance", 30); err == nil {
		if pts, ok := distJ["dataPoints"].([]interface{}); ok {
			dailyMap := make(map[string]float64)
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if dField, ok := m["distance"].(map[string]interface{}); ok {
						if start, ok := dField["interval"].(map[string]interface{}); ok {
							if cTime, ok := start["civilStartTime"].(map[string]interface{}); ok {
								if dateM, ok := cTime["date"].(map[string]interface{}); ok {
									dateStr := fmt.Sprintf("%.0f-%.0f-%.0f", parseNumeric(dateM["year"]), parseNumeric(dateM["month"]), parseNumeric(dateM["day"]))
									// millimeters to meters = divide by 1000
									dailyMap[dateStr] += parseNumeric(dField["millimeters"]) / 1000.0
								}
							}
						}
					}
				}
			}
			var total float64
			for _, v := range dailyMap {
				total += v
			}
			if len(dailyMap) > 0 {
				summary.AvgDistanceMeters = total / float64(len(dailyMap))
			}
		}
	}

	// 4. Resting Heart Rate
	if hrJ, err := fetchGenericData(ctx, client, "daily-resting-heart-rate", 30); err == nil {
		if pts, ok := hrJ["dataPoints"].([]interface{}); ok {
			var total float64
			var count float64
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if rField, ok := m["dailyRestingHeartRate"].(map[string]interface{}); ok {
						val := parseNumeric(rField["beatsPerMinute"])
						if val > 0 {
							total += val
							count++
						}
					}
				}
			}
			if count > 0 {
				summary.AvgRestingHeartRate = total / count
			}
		}
	}

	// 5. Sleep Duration (hours)
	if sleepJ, err := fetchGenericData(ctx, client, "sleep", 30); err == nil {
		if pts, ok := sleepJ["dataPoints"].([]interface{}); ok {
			var total float64
			var count float64
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if sField, ok := m["sleep"].(map[string]interface{}); ok {
						if summ, ok := sField["summary"].(map[string]interface{}); ok {
							val := parseNumeric(summ["minutesAsleep"])
							if val > 0 {
								total += val / 60.0 // convert to hours
								count++
							}
						}
					}
				}
			}
			if count > 0 {
				summary.AvgSleepDurationHrs = total / count
			}
		}
	}

	// 6. HRV
	if hrvJ, err := fetchGenericData(ctx, client, "daily-heart-rate-variability", 30); err == nil {
		if pts, ok := hrvJ["dataPoints"].([]interface{}); ok {
			var total float64
			var count float64
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if hrvField, ok := m["dailyHeartRateVariability"].(map[string]interface{}); ok {
						val := parseNumeric(hrvField["averageHeartRateVariabilityMilliseconds"])
						if val > 0 {
							total += val
							count++
						}
					}
				}
			}
			if count > 0 {
				summary.AvgHRV = total / count
			}
		}
	}

	// 7. SpO2
	if spo2J, err := fetchGenericData(ctx, client, "daily-oxygen-saturation", 7); err == nil {
		if pts, ok := spo2J["dataPoints"].([]interface{}); ok {
			var total float64
			var count float64
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if sField, ok := m["dailyOxygenSaturation"].(map[string]interface{}); ok {
						val := parseNumeric(sField["averagePercentage"])
						if val > 0 {
							total += val
							count++
						}
					}
				}
			}
			if count > 0 {
				summary.AvgSpO2 = total / count
			}
		}
	}

	// 8. Sedentary Duration (hours)
	if sedJ, err := fetchGenericData(ctx, client, "sedentary-period", 30); err == nil {
		if pts, ok := sedJ["dataPoints"].([]interface{}); ok {
			dailyMap := make(map[string]float64)
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if sField, ok := m["sedentaryPeriod"].(map[string]interface{}); ok {
						if interval, ok := sField["interval"].(map[string]interface{}); ok {
							startStr, _ := interval["startTime"].(string)
							endStr, _ := interval["endTime"].(string)
							if startStr != "" && endStr != "" {
								start, errS := time.Parse(time.RFC3339, startStr)
								end, errE := time.Parse(time.RFC3339, endStr)
								if errS == nil && errE == nil {
									dateStr := start.Format("2006-01-02")
									durHrs := end.Sub(start).Hours()
									dailyMap[dateStr] += durHrs
								}
							}
						}
					}
				}
			}
			var total float64
			for _, v := range dailyMap {
				total += v
			}
			if len(dailyMap) > 0 {
				summary.AvgSedentaryHrs = total / float64(len(dailyMap))
			}
		}
	}

	return summary, nil
}

func parseNumeric(val interface{}) float64 {
	if val == nil {
		return 0
	}
	switch v := val.(type) {
	case float64:
		return v
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		return f
	case int:
		return float64(v)
	case int64:
		return float64(v)
	}
	return 0
}

// Result construction helpers

func errorResult(err error) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		IsError: true,
		Content: []mcp.Content{
			&mcp.TextContent{Text: fmt.Sprintf("Error: %v", err)},
		},
	}
}

func jsonResult(data interface{}) *mcp.CallToolResult {
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return errorResult(err)
	}
	return &mcp.CallToolResult{
		Content: []mcp.Content{
			&mcp.TextContent{Text: string(b)},
		},
	}
}

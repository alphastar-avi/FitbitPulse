package main

import (
	"context"
	"encoding/csv"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"

	"math"

	"github.com/joho/godotenv"
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

type DailyData struct {
	Date                string  `json:"date"`
	Steps               int     `json:"steps,omitempty"`
	RestingHR           int     `json:"resting_heart_rate_bpm,omitempty"`
	SleepHours           float64 `json:"sleep_hours,omitempty"`
	DeepHours           float64 `json:"deep_hours,omitempty"`
	REMHours            float64 `json:"rem_hours,omitempty"`
	LightHours          float64 `json:"light_hours,omitempty"`
	AwakeHours          float64 `json:"awake_hours,omitempty"`
	HRV                 float64 `json:"hrv_ms,omitempty"`
	HRVEntropy          float64 `json:"hrv_entropy,omitempty"`
	HRVRMSSD            float64 `json:"hrv_rmssd_ms,omitempty"`
	BreathingRate       float64 `json:"breathing_rate_bpm,omitempty"`
	WristTempNightly    float64 `json:"wrist_temp_nightly_c,omitempty"`
	WristTempBaseline   float64 `json:"wrist_temp_baseline_c,omitempty"`
	WristTempDeviation  float64 `json:"wrist_temp_deviation_c,omitempty"`
	SpO2Avg             float64 `json:"spo2_avg_pct,omitempty"`
	SpO2Low             float64 `json:"spo2_low_pct,omitempty"`
	SpO2High            float64 `json:"spo2_high_pct,omitempty"`
	DistanceMeters      float64 `json:"distance_meters,omitempty"`
	ActiveMinutes       float64 `json:"active_minutes,omitempty"`
	ZoneFatBurnMinutes  float64 `json:"zone_fat_burn_minutes,omitempty"`
	ZoneCardioMinutes   float64 `json:"zone_cardio_minutes,omitempty"`
	ZonePeakMinutes     float64 `json:"zone_peak_minutes,omitempty"`
	SedentaryHours      float64 `json:"sedentary_hours,omitempty"`
}

func main() {
	log.SetFlags(log.LstdFlags | log.Lshortfile)

	// Command line flags
	formatFlag := flag.String("format", "json", "Output format: json or csv")
	outFlag := flag.String("out", "", "Output file path (default is stdout)")
	daysFlag := flag.Int("days", 30, "Number of days of data to retrieve (1-90)")
	allFlag := flag.Bool("all", false, "Retrieve all available history (90 days)")

	flag.Parse()

	// Handle positional argument for days count or 'all' if provided
	args := flag.Args()
	if len(args) > 0 {
		if args[0] == "all" {
			*allFlag = true
		} else if val, err := strconv.Atoi(args[0]); err == nil {
			*daysFlag = val
		}
	}

	if *allFlag {
		*daysFlag = 90
	}

	if *daysFlag <= 0 || *daysFlag > 90 {
		log.Fatalf("Invalid days count: %d. Please specify a value between 1 and 90, or use -all / all.", *daysFlag)
	}

	// Locate and load .env and token.json
	envFile := findFile(".env")
	tokenPath = findFile("token.json")

	if err := godotenv.Load(envFile); err != nil {
		log.Printf("Note: .env not loaded or not found (%s): %v. Reading from environment variables.", envFile, err)
	}

	clientID := os.Getenv("GOOGLE_CLIENT_ID")
	clientSecret := os.Getenv("GOOGLE_CLIENT_SECRET")

	if clientID == "" || clientSecret == "" {
		log.Fatalf("GOOGLE_CLIENT_ID and GOOGLE_CLIENT_SECRET must be set in your environment or .env file.")
	}

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

	ctx := context.Background()
	client, err := getClient(ctx)
	if err != nil {
		log.Fatalf("Client error: %v", err)
	}

	log.Printf("Fetching Fitbit health data for the past %d days...", *daysFlag)

	// Map of date (YYYY-MM-DD format or similar) -> DailyData
	mergedData := make(map[string]*DailyData)
	getOrCreateRecord := func(dateStr string) *DailyData {
		if _, ok := mergedData[dateStr]; !ok {
			mergedData[dateStr] = &DailyData{Date: dateStr}
		}
		return mergedData[dateStr]
	}

	// Helper to format date keys
	formatDate := func(year, month, day interface{}) string {
		y := int(parseNumeric(year))
		m := int(parseNumeric(month))
		d := int(parseNumeric(day))
		return fmt.Sprintf("%04d-%02d-%02d", y, m, d)
	}
	formatIsoDate := func(isoStr string) string {
		if t, err := time.Parse(time.RFC3339, isoStr); err == nil {
			return t.Format("2006-01-02")
		}
		// Fallback to simple split if timezone format differs
		if len(isoStr) >= 10 {
			return isoStr[:10]
		}
		return isoStr
	}

	// 1. Fetch Steps
	if stepsJ, err := fetchSteps(ctx, client, *daysFlag); err == nil {
		if pts, ok := stepsJ["rollupDataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if stepsField, ok := m["steps"].(map[string]interface{}); ok {
						if cTime, ok := m["civilStartTime"].(map[string]interface{}); ok {
							if dateM, ok := cTime["date"].(map[string]interface{}); ok {
								dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
								rec := getOrCreateRecord(dateKey)
								rec.Steps = int(parseNumeric(stepsField["countSum"]))
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch steps: %v", err)
	}

	// 2. Fetch Resting Heart Rate
	if hrJ, err := fetchGenericData(ctx, client, "daily-resting-heart-rate", *daysFlag); err == nil {
		if pts, ok := hrJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if rField, ok := m["dailyRestingHeartRate"].(map[string]interface{}); ok {
						if dateM, ok := rField["date"].(map[string]interface{}); ok {
							dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
							rec := getOrCreateRecord(dateKey)
							rec.RestingHR = int(parseNumeric(rField["beatsPerMinute"]))
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch resting heart rate: %v", err)
	}

	// 3. Fetch Sleep
	if sleepJ, err := fetchGenericData(ctx, client, "sleep", *daysFlag); err == nil {
		if pts, ok := sleepJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if sField, ok := m["sleep"].(map[string]interface{}); ok {
						if interval, ok := sField["interval"].(map[string]interface{}); ok {
							if startTimeStr, ok := interval["startTime"].(string); ok {
								dateKey := formatIsoDate(startTimeStr)
								rec := getOrCreateRecord(dateKey)
								if summ, ok := sField["summary"].(map[string]interface{}); ok {
									rec.SleepHours = parseFloatLimit(parseNumeric(summ["minutesAsleep"])/60.0, 2)
									if stages, ok := summ["stagesSummary"].([]interface{}); ok {
										for _, stg := range stages {
											if stgMap, ok := stg.(map[string]interface{}); ok {
												mins := parseNumeric(stgMap["minutes"])
												hours := parseFloatLimit(mins/60.0, 2)
												switch stgMap["type"] {
												case "DEEP":
													rec.DeepHours = hours
												case "REM":
													rec.REMHours = hours
												case "LIGHT":
													rec.LightHours = hours
												case "AWAKE":
													rec.AwakeHours = hours
												}
											}
										}
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch sleep: %v", err)
	}

	// 4. Fetch HRV
	if hrvJ, err := fetchGenericData(ctx, client, "daily-heart-rate-variability", *daysFlag); err == nil {
		if pts, ok := hrvJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if hrvField, ok := m["dailyHeartRateVariability"].(map[string]interface{}); ok {
						if dateM, ok := hrvField["date"].(map[string]interface{}); ok {
							dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
							rec := getOrCreateRecord(dateKey)
							rec.HRV = parseNumeric(hrvField["averageHeartRateVariabilityMilliseconds"])
							rec.HRVEntropy = parseFloatLimit(parseNumeric(hrvField["entropy"]), 3)
							rec.HRVRMSSD = parseFloatLimit(parseNumeric(hrvField["deepSleepRootMeanSquareOfSuccessiveDifferencesMilliseconds"]), 2)
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch HRV: %v", err)
	}

	// 5. Fetch Breathing Rate
	if respJ, err := fetchGenericData(ctx, client, "daily-respiratory-rate", *daysFlag); err == nil {
		if pts, ok := respJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if respField, ok := m["dailyRespiratoryRate"].(map[string]interface{}); ok {
						if dateM, ok := respField["date"].(map[string]interface{}); ok {
							dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
							rec := getOrCreateRecord(dateKey)
							rec.BreathingRate = parseNumeric(respField["breathsPerMinute"])
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch respiratory rate: %v", err)
	}

	// 6. Fetch Wrist Temperature
	if tempJ, err := fetchGenericData(ctx, client, "daily-sleep-temperature-derivations", *daysFlag); err == nil {
		if pts, ok := tempJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if tField, ok := m["dailySleepTemperatureDerivations"].(map[string]interface{}); ok {
						if dateM, ok := tField["date"].(map[string]interface{}); ok {
							dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
							rec := getOrCreateRecord(dateKey)
							rec.WristTempNightly = parseFloatLimit(parseNumeric(tField["nightlyTemperatureCelsius"]), 2)
							rec.WristTempBaseline = parseFloatLimit(parseNumeric(tField["baselineTemperatureCelsius"]), 2)
							rec.WristTempDeviation = parseFloatLimit(rec.WristTempNightly-rec.WristTempBaseline, 2)
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch sleep temperature derivations: %v", err)
	}

	// 7. Fetch SpO2 (Standard query has strict 7-day limit on oxygen-saturation, but let's query daily-oxygen-saturation)
	spo2Days := *daysFlag
	if spo2Days > 7 {
		spo2Days = 7
	}
	if spo2J, err := fetchGenericData(ctx, client, "daily-oxygen-saturation", spo2Days); err == nil {
		if pts, ok := spo2J["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if sField, ok := m["dailyOxygenSaturation"].(map[string]interface{}); ok {
						if dateM, ok := sField["date"].(map[string]interface{}); ok {
							dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
							rec := getOrCreateRecord(dateKey)
							rec.SpO2Avg = parseFloatLimit(parseNumeric(sField["averagePercentage"]), 2)
							rec.SpO2Low = parseNumeric(sField["lowerBoundPercentage"])
							rec.SpO2High = parseNumeric(sField["upperBoundPercentage"])
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch oxygen saturation: %v", err)
	}

	// 8. Fetch Distance (meters)
	if distJ, err := fetchGenericData(ctx, client, "distance", *daysFlag); err == nil {
		if pts, ok := distJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if dField, ok := m["distance"].(map[string]interface{}); ok {
						if start, ok := dField["interval"].(map[string]interface{}); ok {
							if cTime, ok := start["civilStartTime"].(map[string]interface{}); ok {
								if dateM, ok := cTime["date"].(map[string]interface{}); ok {
									dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
									rec := getOrCreateRecord(dateKey)
									meters := parseNumeric(dField["millimeters"]) / 1000.0
									rec.DistanceMeters += parseFloatLimit(meters, 1)
								}
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch distance: %v", err)
	}

	// 9. Fetch Active Minutes
	if actJ, err := fetchGenericData(ctx, client, "active-minutes", *daysFlag); err == nil {
		if pts, ok := actJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if actField, ok := m["activeMinutes"].(map[string]interface{}); ok {
						if start, ok := actField["interval"].(map[string]interface{}); ok {
							if cTime, ok := start["civilStartTime"].(map[string]interface{}); ok {
								if dateM, ok := cTime["date"].(map[string]interface{}); ok {
									dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
									rec := getOrCreateRecord(dateKey)
									if levels, ok := actField["activeMinutesByActivityLevel"].([]interface{}); ok {
										var mins float64
										for _, lvl := range levels {
											if lvlMap, ok := lvl.(map[string]interface{}); ok {
												mins += parseNumeric(lvlMap["activeMinutes"])
											}
										}
										rec.ActiveMinutes += mins
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch active minutes: %v", err)
	}

	// 10. Fetch Active Zone Minutes
	if actZoneJ, err := fetchGenericData(ctx, client, "active-zone-minutes", *daysFlag); err == nil {
		if pts, ok := actZoneJ["dataPoints"].([]interface{}); ok {
			for _, p := range pts {
				if m, ok := p.(map[string]interface{}); ok {
					if azField, ok := m["activeZoneMinutes"].(map[string]interface{}); ok {
						if start, ok := azField["interval"].(map[string]interface{}); ok {
							if cTime, ok := start["civilStartTime"].(map[string]interface{}); ok {
								if dateM, ok := cTime["date"].(map[string]interface{}); ok {
									dateKey := formatDate(dateM["year"], dateM["month"], dateM["day"])
									rec := getOrCreateRecord(dateKey)
									mins := parseNumeric(azField["activeZoneMinutes"])
									switch azField["heartRateZone"] {
									case "FAT_BURN":
										rec.ZoneFatBurnMinutes += mins
									case "CARDIO":
										rec.ZoneCardioMinutes += mins
									case "PEAK":
										rec.ZonePeakMinutes += mins
									}
								}
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch active zone minutes: %v", err)
	}

	// 11. Fetch Sedentary Period
	if sedJ, err := fetchGenericData(ctx, client, "sedentary-period", *daysFlag); err == nil {
		if pts, ok := sedJ["dataPoints"].([]interface{}); ok {
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
									dateKey := start.Format("2006-01-02")
									rec := getOrCreateRecord(dateKey)
									durHrs := end.Sub(start).Hours()
									rec.SedentaryHours += parseFloatLimit(durHrs, 2)
								}
							}
						}
					}
				}
			}
		}
	} else {
		log.Printf("Warning: Failed to fetch sedentary periods: %v", err)
	}

	// Convert map to slice and sort by date descending (newest first)
	var records []*DailyData
	for _, v := range mergedData {
		records = append(records, v)
	}
	sort.Slice(records, func(i, j int) bool {
		return records[i].Date > records[j].Date
	})

	// Format output
	var outputBytes []byte
	var writeErr error

	if strings.ToLower(*formatFlag) == "csv" {
		outputBytes, writeErr = formatCSV(records)
	} else {
		outputBytes, writeErr = json.MarshalIndent(records, "", "  ")
	}

	if writeErr != nil {
		log.Fatalf("Failed to format output: %v", writeErr)
	}

	// Output destination
	if *outFlag != "" {
		err = os.WriteFile(*outFlag, outputBytes, 0644)
		if err != nil {
			log.Fatalf("Failed to write to file %s: %v", *outFlag, err)
		}
		fmt.Printf("Successfully wrote report to %s\n", *outFlag)
	} else {
		fmt.Println(string(outputBytes))
	}
}

// Helpers for CSV Formatting
func formatCSV(records []*DailyData) ([]byte, error) {
	var buf strings.Builder
	w := csv.NewWriter(&buf)

	// Write Headers
	headers := []string{
		"Date", "Steps", "Resting_HR_BPM", "Sleep_Hours", "Deep_Hours", "REM_Hours", "Light_Hours", "Awake_Hours",
		"HRV_ms", "HRV_Entropy", "HRV_RMSSD_ms", "Breathing_Rate_BPM", "Wrist_Temp_Nightly_C", "Wrist_Temp_Baseline_C", "Wrist_Temp_Deviation_C",
		"SpO2_Avg_Pct", "SpO2_Low_Pct", "SpO2_High_Pct", "Distance_Meters", "Active_Minutes",
		"Zone_FatBurn_Mins", "Zone_Cardio_Mins", "Zone_Peak_Mins", "Sedentary_Hours",
	}
	if err := w.Write(headers); err != nil {
		return nil, err
	}

	// Write Rows
	for _, r := range records {
		row := []string{
			r.Date,
			valOrEmpty(r.Steps),
			valOrEmpty(r.RestingHR),
			valOrEmpty(r.SleepHours),
			valOrEmpty(r.DeepHours),
			valOrEmpty(r.REMHours),
			valOrEmpty(r.LightHours),
			valOrEmpty(r.AwakeHours),
			valOrEmpty(r.HRV),
			valOrEmpty(r.HRVEntropy),
			valOrEmpty(r.HRVRMSSD),
			valOrEmpty(r.BreathingRate),
			valOrEmpty(r.WristTempNightly),
			valOrEmpty(r.WristTempBaseline),
			valOrEmpty(r.WristTempDeviation),
			valOrEmpty(r.SpO2Avg),
			valOrEmpty(r.SpO2Low),
			valOrEmpty(r.SpO2High),
			valOrEmpty(r.DistanceMeters),
			valOrEmpty(r.ActiveMinutes),
			valOrEmpty(r.ZoneFatBurnMinutes),
			valOrEmpty(r.ZoneCardioMinutes),
			valOrEmpty(r.ZonePeakMinutes),
			valOrEmpty(r.SedentaryHours),
		}
		if err := w.Write(row); err != nil {
			return nil, err
		}
	}

	w.Flush()
	if err := w.Error(); err != nil {
		return nil, err
	}

	return []byte(buf.String()), nil
}

func valOrEmpty(v interface{}) string {
	switch val := v.(type) {
	case int:
		if val == 0 {
			return ""
		}
		return strconv.Itoa(val)
	case float64:
		if val == 0.0 {
			return ""
		}
		return fmt.Sprintf("%.2f", val)
	}
	return ""
}

// Helpers for client and API requests
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
		return nil, fmt.Errorf("authentication required. Go to http://localhost:8080/login first to authorize access: %w", err)
	}

	// Create reusable token source that updates token.json upon refresh
	baseSource := oauthConfig.TokenSource(ctx, tok)
	savingSource := &savingTokenSource{
		source:    baseSource,
		tokenFile: tokenPath,
	}

	return oauth2.NewClient(ctx, savingSource), nil
}

type savingTokenSource struct {
	source    oauth2.TokenSource
	tokenFile string
}

func (s *savingTokenSource) Token() (*oauth2.Token, error) {
	tok, err := s.source.Token()
	if err != nil {
		return nil, err
	}
	existing, err := tokenFromFile(s.tokenFile)
	if err != nil || existing.AccessToken != tok.AccessToken {
		log.Printf("Token refreshed, saving to %s\n", s.tokenFile)
		if errSave := saveToken(s.tokenFile, tok); errSave != nil {
			log.Printf("Failed to save token: %v\n", errSave)
		}
	}
	return tok, nil
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

func fetchSteps(ctx context.Context, client *http.Client, days int) (map[string]interface{}, error) {
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

func fetchGenericData(ctx context.Context, client *http.Client, dataType string, days int) (map[string]interface{}, error) {
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
			bodyStr, _ := json.Marshal(chunk)
			return nil, fmt.Errorf("HTTP %d: %s", resp.StatusCode, string(bodyStr))
		}
	}

	if lastRespObj == nil {
		return nil, fmt.Errorf("failed to fetch data for type %s", dataType)
	}

	lastRespObj["dataPoints"] = allPoints
	delete(lastRespObj, "nextPageToken")
	return lastRespObj, nil
}

func parseNumeric(val interface{}) float64 {
	if val == nil {
		return 0
	}
	var res float64
	switch v := val.(type) {
	case float64:
		res = v
	case string:
		var f float64
		fmt.Sscanf(v, "%f", &f)
		res = f
	case int:
		res = float64(v)
	case int64:
		res = float64(v)
	}
	if math.IsNaN(res) || math.IsInf(res, 0) {
		return 0
	}
	return res
}

func parseFloatLimit(val float64, decimals int) float64 {
	format := fmt.Sprintf("%%.%df", decimals)
	formatted := fmt.Sprintf(format, val)
	parsed, _ := strconv.ParseFloat(formatted, 64)
	return parsed
}

# FitbitPulse

A Go application to frequently download and display Fitbit Charge 6 health data.

<img width="1348" height="1017" alt="Screenshot 2026-05-10 at 10 46 49 PM" src="https://github.com/user-attachments/assets/eb0b69ae-4af1-4635-b866-efcf459afc74" />

<img width="1348" height="1017" alt="Screenshot 2026-05-10 at 10 47 13 PM" src="https://github.com/user-attachments/assets/ca7f5c73-905f-47a8-a8e9-791334fc517d" />

<img width="1361" height="1013" alt="Screenshot 2026-05-27 at 12 11 35 AM" src="https://github.com/user-attachments/assets/effcd018-d3b8-4774-95e2-145b13c9f6a2" />

<img width="1348" height="1017" alt="Screenshot 2026-05-10 at 10 47 22 PM" src="https://github.com/user-attachments/assets/f665b22d-55ba-430d-93f6-ccce77b7d54d" />

and more!

## Features

* **Steps & Activity Tracking**: Daily steps, active zone minutes, sedentary periods, and distance measurements.
* **Vitals & Recovery**: Resting heart rate, heart rate variability (HRV), wrist skin temperature derivations, and average breathing rate.
* **Sleep Stages Analysis**: Visual breakdown of deep, REM, light, and awake intervals from nightly sleep sessions.
* **Sleep Oxygen Saturation & EOV**: Passive blood oxygen (SpO2) monitoring showing nightly ranges, standard deviation, and dynamic estimated oxygen variation (EOV) timelines mapping transient desaturations.
* **Data Portability**: Full history exports available in CSV and JSON formats.
* **Raw Data Explorer**: Direct JSON payload viewing for all queried Google Health API (Fitbit Charge 6) endpoints.

## Setup

1. Clone the repository.
2. Copy `.env.example` to `.env` and fill in your Google API credentials.
3. Run `go mod tidy` to install dependencies.
4. Run `go run main.go` to start the server.
5. Open `http://localhost:8080/login` in your browser to authenticate.

---

## Model Context Protocol (MCP) Server

I built a Model Context Protocol (MCP) server for this project to expose all of the Fitbit Charge 6 health metrics directly to AI assistants (like Claude Desktop or Cursor). This allows an LLM to query my daily steps, resting heart rate, sleep stages, heart rate variability (HRV), and more.

### Features of the MCP Server
- **Auto Token Refresh**: Reuses the OAuth2 configuration and automatically refreshes and saves the token to `token.json` when it expires.
- **Google API Workarounds**: Automatically handles steps chunking (bypassing Google's 33-day query limit) and handles camelCase schemas for queries.
- **Stdio Transport**: Communicates over standard stdin/stdout for easy local integration.

### Exposed Tools
- `get_steps`: Fetch steps daily rollup (up to 90 days).
- `get_sleep_sessions`: Fetch sleep durations and stage breakdowns (REM, deep, light, awake).
- `get_resting_heart_rate`: Fetch daily resting heart rate metrics (BPM).
- `get_hrv`: Fetch Heart Rate Variability (entropy, RMSSD, average ms).
- `get_breathing_rate`: Fetch nightly average breaths per minute.
- `get_wrist_temperature`: Fetch nightly wrist temperature deviations against baseline.
- `get_blood_oxygen`: Fetch nightly SpO2 average and range (max 7 days).
- `get_activity_metrics`: Combined activity metrics (distance, active minutes, active zone minutes, sedentary periods).
- `get_dashboard_summary`: Aggregate summary of 30-day averages across all metrics.

### Setup and Running the MCP Server

1. **Build the MCP binary**:
   ```bash
   go build -o mcp-server-bin ./mcp-server
   ```

2. **Configure Claude Desktop**:
   Open your Claude Desktop config (usually at `~/Library/Application Support/Claude/claude_desktop_config.json`) and add the server:
   ```json
   {
     "mcpServers": {
       "fitbit-pulse": {
         "command": "/path/to/FitbitPulse/mcp-server-bin",
         "cwd": "/path/to/FitbitPulse"
       }
     }
   }
   ```

3. **Configure Cursor**:
   Go to **Settings > Features > MCP**, click **Add New MCP Server**, choose type `command`, and set the command path:
   `/path/to/FitbitPulse/mcp-server-bin`

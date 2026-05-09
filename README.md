# FitbitPulse

A Go application to frequently download and display Fitbit Charge 6 health data.

## Prerequisites

- Go 1.20+
- Fitbit Developer App credentials (Client ID and Client Secret)

## Setup

1. Clone the repository.
2. Put you're client and secret in the env
3. Run `go mod tidy` to install dependencies.
4. Run `go run main.go` to start the server.
5. Open `http://localhost:8080/login` in your browser to authenticate.

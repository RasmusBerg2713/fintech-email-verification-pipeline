package main

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"os"
	"strings"

	verification "github.com/infrai-examples/fintech-email-verification"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	publicBaseURL := os.Getenv("PUBLIC_BASE_URL")
	if publicBaseURL == "" {
		publicBaseURL = "http://localhost:8080"
	}
	flow := verification.NewFlow(verification.NewClient(apiKey), publicBaseURL)
	mux := http.NewServeMux()
	mux.HandleFunc("POST /signup", func(w http.ResponseWriter, r *http.Request) {
		var input verification.SignupInput
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 32<<10)).Decode(&input); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid signup input"})
			return
		}
		result, err := flow.StartSignup(r.Context(), input)
		if err != nil {
			writeFlowError(w, err)
			return
		}
		writeJSON(w, http.StatusAccepted, result)
	})
	mux.HandleFunc("GET /accounts/{accountID}/email-events", func(w http.ResponseWriter, r *http.Request) {
		events, err := flow.DeliveryEvents(r.Context(), r.PathValue("accountID"))
		if err != nil {
			writeFlowError(w, err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]json.RawMessage{"events": events})
	})
	log.Println("verification service listening on :8080")
	log.Fatal(http.ListenAndServe(":8080", mux))
}

func writeFlowError(w http.ResponseWriter, err error) {
	status := http.StatusBadRequest
	var apiErr *verification.APIError
	if errors.As(err, &apiErr) {
		status = apiErr.HTTPStatus
		if status < 400 || status >= 500 {
			status = http.StatusBadGateway
		}
	}
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(value); err != nil && !strings.Contains(err.Error(), "closed") {
		log.Printf("encode response: %v", err)
	}
}

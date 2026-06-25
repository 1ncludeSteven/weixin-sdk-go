package main

import (
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	weixin "github.com/1ncludeSteven/weixin-sdk-go"
)

type HealthResponse struct {
	Status    string `json:"status"`
	Version   string `json:"version"`
	Timestamp string `json:"timestamp"`
	Uptime    string `json:"uptime"`
}

type AccountsResponse struct {
	Count    int      `json:"count"`
	Accounts []string `json:"accounts"`
}

type ErrorResponse struct {
	Error   string `json:"error"`
	Code    int    `json:"code"`
	Message string `json:"message,omitempty"`
}

type EchoRequest struct {
	Message string `json:"message"`
}

type EchoResponse struct {
	Echo      string `json:"echo"`
	Timestamp string `json:"timestamp"`
}

var startTime = time.Now()
var sdk = weixin.New(nil)

func jsonResponse(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(data)
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, HealthResponse{
		Status:    "healthy",
		Version:   weixin.Version,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
		Uptime:    time.Since(startTime).Round(time.Second).String(),
	})
}

func accountsHandler(w http.ResponseWriter, r *http.Request) {
	accounts := sdk.ListAccounts()
	jsonResponse(w, http.StatusOK, AccountsResponse{
		Count:    len(accounts),
		Accounts: accounts,
	})
}

func versionHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusOK, map[string]string{
		"sdk_version": weixin.Version,
		"server_time": time.Now().UTC().Format(time.RFC3339),
	})
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		jsonResponse(w, http.StatusMethodNotAllowed, ErrorResponse{
			Error: "method_not_allowed",
			Code:  405,
		})
		return
	}

	var req EchoRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{
			Error:   "invalid_request",
			Code:    400,
			Message: "request body must be valid JSON with 'message' field",
		})
		return
	}

	if req.Message == "" {
		jsonResponse(w, http.StatusBadRequest, ErrorResponse{
			Error:   "bad_request",
			Code:    400,
			Message: "'message' field is required",
		})
		return
	}

	jsonResponse(w, http.StatusOK, EchoResponse{
		Echo:      req.Message,
		Timestamp: time.Now().UTC().Format(time.RFC3339),
	})
}

func notFoundHandler(w http.ResponseWriter, r *http.Request) {
	jsonResponse(w, http.StatusNotFound, ErrorResponse{
		Error:   "not_found",
		Code:    404,
		Message: fmt.Sprintf("endpoint %s not found", r.URL.Path),
	})
}

func main() {
	port := os.Getenv("PORT")
	if port == "" {
		port = "8443"
	}
	certFile := os.Getenv("CERT_FILE")
	keyFile := os.Getenv("KEY_FILE")

	mux := http.NewServeMux()
	mux.HandleFunc("/health", healthHandler)
	mux.HandleFunc("/api/accounts", accountsHandler)
	mux.HandleFunc("/api/version", versionHandler)
	mux.HandleFunc("/api/echo", echoHandler)
	mux.HandleFunc("/", notFoundHandler)

	log.Printf("weixin-sdk-go demo server starting on :%s", port)
	log.Printf("Endpoints:")
	log.Printf("  GET  /health       - health check")
	log.Printf("  GET  /api/accounts - list accounts")
	log.Printf("  GET  /api/version  - SDK version")
	log.Printf("  POST /api/echo     - echo message (JSON body: {\"message\": \"...\"})")

	var err error
	if certFile != "" && keyFile != "" {
		log.Printf("TLS enabled: cert=%s key=%s", certFile, keyFile)
		err = http.ListenAndServeTLS(":"+port, certFile, keyFile, mux)
	} else {
		log.Printf("Running without TLS")
		err = http.ListenAndServe(":"+port, mux)
	}
	if err != nil {
		log.Fatalf("server failed: %v", err)
	}
}

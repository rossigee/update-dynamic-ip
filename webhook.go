package main

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type WebhookData struct {
	IPAddress string `json:"ip_address"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func startServer(listen_address string) {
	http.HandleFunc("/", handleWebhook)
	http.HandleFunc("/health", healthCheck)
	http.ListenAndServe(listen_address, nil)
}

func response(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	status := "ok"
	if statusCode != 200 {
		status = "error"
	}
	response := Response{
		Status:  status,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}

func handleWebhook(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		response(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var data []WebhookData
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		response(w, http.StatusBadRequest, "Failed to parse JSON")
		log.Errorf("Failed to parse JSON: %v", err)
		return
	}

	if len(data) < 1 {
		response(w, http.StatusBadRequest, "No entries provided")
		log.Errorf("No entries provided in JSON")
		return
	}
	if len(data) > 1 {
		response(w, http.StatusBadRequest, "Unexpected number of entries provided")
		log.Warnf("Unexpected number of entries provided in JSON: %d", len(data))
	}

	entry := data[0]
	ipAddress := entry.IPAddress

	log.Infof("Received webhook with IP address: %s", ipAddress)
	err := setIPAddress(ipAddress)
	if err != nil {
		response(w, http.StatusInternalServerError, "Failed to set IP address")
		log.Errorf("Failed to set IP address: %v", err)
		return
	}

	response(w, http.StatusOK, "Updated DNS configuration")
	log.Infof("Updated DNS configuration successfully")
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusOK)
}

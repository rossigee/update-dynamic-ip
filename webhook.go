package main

import (
	"encoding/json"
	"net/http"

	log "github.com/sirupsen/logrus"
)

type WebhookData struct {
	ServiceName string `json:"service_name"`
	IPAddress   string `json:"ip_address"`
}

type Response struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

func startServer(listen_address string, client K8sClient) {
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(w, r, client)
	})
	http.HandleFunc("/health", healthCheck)
	http.ListenAndServe(listen_address, nil)
}

func response(w http.ResponseWriter, statusCode int, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)

	status := "ok"
	if statusCode != http.StatusOK {
		status = "error"
	}
	response := Response{
		Status:  status,
		Message: message,
	}

	json.NewEncoder(w).Encode(response)
}

func handleWebhook(w http.ResponseWriter, r *http.Request, client K8sClient) {
	if r.Method != http.MethodPost {
		response(w, http.StatusMethodNotAllowed, "Method not allowed")
		return
	}

	var data WebhookData
	log.Debug(r.Body)
	decoder := json.NewDecoder(r.Body)
	if err := decoder.Decode(&data); err != nil {
		response(w, http.StatusBadRequest, "Failed to parse JSON")
		log.Errorf("Failed to parse JSON: %v", err)
		return
	}

	if data.ServiceName == "" || data.IPAddress == "" {
		response(w, http.StatusBadRequest, "Service name ('service_name') and/or IP address ('ip_address') missing from JSON payload")
		log.Errorf("Service name ('service_name') and/or IP address ('ip_address') missing from JSON payload")
		return
	}
	serviceName := data.ServiceName
	ipAddress := data.IPAddress

	log.Debugf("Received webhook for service '%s' with IP address: %s", serviceName, ipAddress)
	err := setIPAddress(client, data.ServiceName, data.IPAddress)
	if err != nil {
		log.Errorf("Failed to set IP address for service '%s' to '%s': %v", serviceName, ipAddress, err)
		response(w, http.StatusInternalServerError, "Failed to update service")
		return
	}

	response(w, http.StatusOK, "Updated DNS configuration")
	log.Infof("Updated DNS for service '%s' to '%s'.", serviceName, ipAddress)
}

func healthCheck(w http.ResponseWriter, r *http.Request) {
	response(w, http.StatusOK, "healthy")
}

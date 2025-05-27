package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

type MockK8sClient struct {
	Services map[string]*corev1.Service
}

func (m *MockK8sClient) GetService(namespace, name string) (*corev1.Service, error) {
	key := fmt.Sprintf("%s/%s", namespace, name)
	if service, ok := m.Services[key]; ok {
		return service, nil
	}
	return nil, fmt.Errorf("service not found")
}

func (m *MockK8sClient) UpdateService(service *corev1.Service) error {
	key := fmt.Sprintf("%s/%s", service.Namespace, service.Name)
	m.Services[key] = service
	return nil
}

func TestHandleWebhook(t *testing.T) {
	// Set up mock client
	mockClient := &MockK8sClient{
		Services: map[string]*corev1.Service{
			"default/service1": {
				ObjectMeta: metav1.ObjectMeta{
					Name:      "service1",
					Namespace: "default",
				},
				Spec: corev1.ServiceSpec{
					ExternalName: "old-ip",
				},
			},
		},
	}

	// Set namespace for testing
	namespace = flag.String("namespace", "default", "Namespace of Service to update")
	flag.Parse()

	// Mock request data
	data := WebhookData{
		ServiceName: "service1",
		IPAddress:   "12.34.56.78",
	}
	jsonData, err := json.Marshal(data)
	if err != nil {
		t.Fatal(err)
	}

	// Create a request to pass to our handler.
	req, err := http.NewRequest("POST", "/", bytes.NewBuffer(jsonData))
	if err != nil {
		t.Fatal(err)
	}

	// Create a ResponseRecorder to record the response.
	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		handleWebhook(w, r, mockClient)
	})

	// Call the ServeHTTP method on the handler.
	handler.ServeHTTP(rr, req)

	// Check the status code and response body.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}
	expected := map[string]string{"status": "ok", "message": "Updated DNS configuration"}
	var actual map[string]string
	err = json.Unmarshal([]byte(strings.TrimSpace(rr.Body.String())), &actual)
	if err != nil {
		t.Fatalf("Error unmarshaling response: %v", err)
	}
	assert.Equal(t, expected, actual, "handler returned unexpected body")

	// Verify that the service was updated
	updatedService, err := mockClient.GetService("default", "service1")
	if err != nil {
		t.Fatal(err)
	}
	if updatedService.Spec.ExternalName != "12.34.56.78" {
		t.Fatalf("Service was not updated correctly. Expected IP: 12.34.56.78, Got: %s", updatedService.Spec.ExternalName)
	}
}

func TestHealthCheck(t *testing.T) {
	req, err := http.NewRequest("GET", "/health", nil)
	if err != nil {
		t.Fatal(err)
	}

	rr := httptest.NewRecorder()
	handler := http.HandlerFunc(healthCheck)

	// Call the ServeHTTP method on the handler.
	handler.ServeHTTP(rr, req)

	// Check the status code and response body.
	if status := rr.Code; status != http.StatusOK {
		t.Fatalf("handler returned wrong status code: got %v want %v", status, http.StatusOK)
	}

	expected := map[string]string{"status": "ok", "message": "healthy"}
	var actual map[string]string
	err = json.Unmarshal([]byte(strings.TrimSpace(rr.Body.String())), &actual)
	if err != nil {
		t.Fatalf("Error unmarshaling response: %v", err)
	}
	assert.Equal(t, expected, actual, "handler returned unexpected body")
}

package app

import (
	"bytes"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gorilla/websocket"
)

func testServer(t *testing.T) (*Store, *httptest.Server, *Hub) {
	t.Helper()
	store, err := OpenStore(":memory:")
	if err != nil {
		t.Fatal(err)
	}
	hub := NewHub([]string{"*"})
	server := httptest.NewServer(NewAPI(store, hub, []string{"*"}))
	t.Cleanup(func() {
		server.Close()
		if err := store.Close(); err != nil {
			t.Errorf("close store: %v", err)
		}
	})
	return store, server, hub
}

func postJSON(t *testing.T, client *http.Client, url string, body any) *http.Response {
	t.Helper()
	encoded, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	request, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(encoded))
	if err != nil {
		t.Fatal(err)
	}
	request.Header.Set("Content-Type", "application/json")
	response, err := client.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	return response
}

func responseJSON(t *testing.T, response *http.Response) map[string]any {
	t.Helper()
	defer response.Body.Close()
	var body map[string]any
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	timeNS, ok := body["time_ns"].(string)
	if !ok || timeNS == "" {
		t.Fatalf("response must include string time_ns, got %#v", body["time_ns"])
	}
	return body
}

func createTestFence(t *testing.T, server *httptest.Server) string {
	t.Helper()
	response := postJSON(t, server.Client(), server.URL+"/geofences", map[string]any{
		"name": "Test zone", "description": "A test area", "category": "restricted_zone",
		"coordinates": [][]float64{{37, -123}, {38, -123}, {38, -122}, {37, -122}, {37, -123}},
	})
	if response.StatusCode != http.StatusCreated {
		body := responseJSON(t, response)
		t.Fatalf("create geofence status = %d, body = %#v", response.StatusCode, body)
	}
	body := responseJSON(t, response)
	return body["id"].(string)
}

func createTestVehicle(t *testing.T, server *httptest.Server) string {
	t.Helper()
	response := postJSON(t, server.Client(), server.URL+"/vehicles", map[string]any{
		"vehicle_number": "KA-01-AB-1234", "driver_name": "A Driver", "vehicle_type": "van", "phone": "+1234567890",
	})
	if response.StatusCode != http.StatusCreated {
		body := responseJSON(t, response)
		t.Fatalf("create vehicle status = %d, body = %#v", response.StatusCode, body)
	}
	body := responseJSON(t, response)
	return body["id"].(string)
}

func TestAPIValidationAndResponseContracts(t *testing.T) {
	_, server, _ := testServer(t)

	response := postJSON(t, server.Client(), server.URL+"/geofences", map[string]any{
		"name": "Open polygon", "category": "delivery_zone",
		"coordinates": [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 1}},
	})
	if response.StatusCode != http.StatusBadRequest {
		t.Fatalf("open polygon status = %d, want %d", response.StatusCode, http.StatusBadRequest)
	}
	if body := responseJSON(t, response); body["error"] == nil {
		t.Fatal("validation response must include an error")
	}

	fenceID := createTestFence(t, server)
	listRequest, err := http.NewRequest(http.MethodGet, server.URL+"/geofences?category=restricted_zone", nil)
	if err != nil {
		t.Fatal(err)
	}
	listResponse, err := server.Client().Do(listRequest)
	if err != nil {
		t.Fatal(err)
	}
	listBody := responseJSON(t, listResponse)
	if listResponse.StatusCode != http.StatusOK || len(listBody["geofences"].([]any)) != 1 {
		t.Fatalf("filtered geofence list = %#v", listBody)
	}

	vehicleID := createTestVehicle(t, server)
	vehiclesRequest, err := http.NewRequest(http.MethodGet, server.URL+"/vehicles", nil)
	if err != nil {
		t.Fatal(err)
	}
	vehiclesResponse, err := server.Client().Do(vehiclesRequest)
	if err != nil {
		t.Fatal(err)
	}
	vehiclesBody := responseJSON(t, vehiclesResponse)
	if len(vehiclesBody["vehicles"].([]any)) != 1 {
		t.Fatalf("unexpected vehicle list: %#v", vehiclesBody)
	}
	duplicate := postJSON(t, server.Client(), server.URL+"/vehicles", map[string]any{
		"vehicle_number": "ka-01-ab-1234", "driver_name": "Another Driver", "vehicle_type": "car", "phone": "1",
	})
	if duplicate.StatusCode != http.StatusConflict {
		t.Fatalf("duplicate vehicle status = %d, want %d", duplicate.StatusCode, http.StatusConflict)
	}
	_ = responseJSON(t, duplicate)

	alert := postJSON(t, server.Client(), server.URL+"/alerts/configure", map[string]any{
		"geofence_id": fenceID, "vehicle_id": vehicleID, "event_type": "entry",
	})
	if alert.StatusCode != http.StatusCreated {
		t.Fatalf("configure alert status = %d", alert.StatusCode)
	}
	alertBody := responseJSON(t, alert)
	if alertBody["alert_id"] == nil || alertBody["event_type"] != "entry" {
		t.Fatalf("unexpected alert response: %#v", alertBody)
	}
	alertsRequest, err := http.NewRequest(http.MethodGet, server.URL+"/alerts?geofence_id="+fenceID+"&vehicle_id="+vehicleID, nil)
	if err != nil {
		t.Fatal(err)
	}
	alertsResponse, err := server.Client().Do(alertsRequest)
	if err != nil {
		t.Fatal(err)
	}
	alertsBody := responseJSON(t, alertsResponse)
	if len(alertsBody["alerts"].([]any)) != 1 {
		t.Fatalf("unexpected alert list: %#v", alertsBody)
	}

	location := postJSON(t, server.Client(), server.URL+"/vehicles/location", map[string]any{
		"vehicle_id": vehicleID, "latitude": 37.5, "longitude": -122.5, "timestamp": "2025-01-15T10:35:00Z",
	})
	if location.StatusCode != http.StatusOK {
		t.Fatalf("update location status = %d, body = %#v", location.StatusCode, responseJSON(t, location))
	}
	locationBody := responseJSON(t, location)
	if locationBody["location_updated"] != true || len(locationBody["current_geofences"].([]any)) != 1 {
		t.Fatalf("unexpected location response: %#v", locationBody)
	}

	currentRequest, err := http.NewRequest(http.MethodGet, server.URL+"/vehicles/location/"+vehicleID, nil)
	if err != nil {
		t.Fatal(err)
	}
	currentResponse, err := server.Client().Do(currentRequest)
	if err != nil {
		t.Fatal(err)
	}
	currentBody := responseJSON(t, currentResponse)
	if currentResponse.StatusCode != http.StatusOK || currentBody["vehicle_number"] != "KA-01-AB-1234" {
		t.Fatalf("unexpected current location: %#v", currentBody)
	}

	historyRequest, err := http.NewRequest(http.MethodGet, server.URL+"/violations/history?vehicle_id="+vehicleID+"&geofence_id="+fenceID+"&limit=1&start_date=2025-01-01T00:00:00Z&end_date=2025-01-31T23:59:59Z", nil)
	if err != nil {
		t.Fatal(err)
	}
	historyResponse, err := server.Client().Do(historyRequest)
	if err != nil {
		t.Fatal(err)
	}
	historyBody := responseJSON(t, historyResponse)
	if historyBody["total_count"] != float64(1) || len(historyBody["violations"].([]any)) != 1 {
		t.Fatalf("unexpected history response: %#v", historyBody)
	}
}

func TestLocationTransitionsArePersistedOnceAndBroadcastForMatchingRules(t *testing.T) {
	_, server, hub := testServer(t)
	fenceID := createTestFence(t, server)
	vehicleID := createTestVehicle(t, server)
	ruleResponse := postJSON(t, server.Client(), server.URL+"/alerts/configure", map[string]any{
		"geofence_id": fenceID, "event_type": "both",
	})
	if ruleResponse.StatusCode != http.StatusCreated {
		t.Fatalf("configure alert status = %d", ruleResponse.StatusCode)
	}
	_ = responseJSON(t, ruleResponse)

	wsURL := "ws" + strings.TrimPrefix(server.URL, "http") + "/ws/alerts"
	connection, _, err := websocket.DefaultDialer.Dial(wsURL, nil)
	if err != nil {
		t.Fatalf("connect WebSocket: %v", err)
	}
	defer connection.Close()
	waitForWebSocketClient(t, hub)

	update := func(latitude, longitude float64, timestamp string) {
		t.Helper()
		response := postJSON(t, server.Client(), server.URL+"/vehicles/location", map[string]any{
			"vehicle_id": vehicleID, "latitude": latitude, "longitude": longitude, "timestamp": timestamp,
		})
		if response.StatusCode != http.StatusOK {
			t.Fatalf("update location status = %d, response = %#v", response.StatusCode, responseJSON(t, response))
		}
		_ = responseJSON(t, response)
	}
	readEvent := func(expectedType string) {
		t.Helper()
		if err := connection.SetReadDeadline(time.Now().Add(2 * time.Second)); err != nil {
			t.Fatal(err)
		}
		var event EventMessage
		if err := connection.ReadJSON(&event); err != nil {
			t.Fatalf("read WebSocket event: %v", err)
		}
		if event.EventID == "" || event.EventType != expectedType ||
			event.Vehicle.VehicleID != vehicleID || event.Geofence.GeofenceID != fenceID {
			t.Fatalf("unexpected WebSocket message: %#v", event)
		}
	}
	update(37.5, -122.5, "2025-01-15T10:35:00Z")
	readEvent("entry")
	update(37.6, -122.6, "2025-01-15T10:36:00Z")
	update(39, -122.5, "2025-01-15T10:37:00Z")
	readEvent("exit")

	historyRequest, err := http.NewRequest(http.MethodGet, server.URL+"/violations/history?vehicle_id="+vehicleID, nil)
	if err != nil {
		t.Fatal(err)
	}
	historyResponse, err := server.Client().Do(historyRequest)
	if err != nil {
		t.Fatal(err)
	}
	historyBody := responseJSON(t, historyResponse)
	if historyBody["total_count"] != float64(2) {
		t.Fatalf("duplicate transition recorded: history = %#v", historyBody)
	}
}

func waitForWebSocketClient(t *testing.T, hub *Hub) {
	t.Helper()
	deadline := time.Now().Add(time.Second)
	for time.Now().Before(deadline) {
		hub.mu.RLock()
		count := len(hub.clients)
		hub.mu.RUnlock()
		if count > 0 {
			return
		}
		time.Sleep(5 * time.Millisecond)
	}
	t.Fatal("WebSocket client was not registered")
}

func TestRulesMatchVehicleAndEvent(t *testing.T) {
	vehicleID := "veh_one"
	rules := []AlertRule{
		{GeofenceID: "geo_one", VehicleID: &vehicleID, EventType: "entry"},
		{GeofenceID: "geo_two", EventType: "both"},
	}
	tests := []struct {
		name       string
		geofenceID string
		vehicleID  string
		eventType  string
		want       bool
	}{
		{"vehicle and entry match", "geo_one", "veh_one", "entry", true},
		{"different vehicle does not match", "geo_one", "veh_two", "entry", false},
		{"wrong event does not match", "geo_one", "veh_one", "exit", false},
		{"all vehicles and both events", "geo_two", "veh_two", "exit", true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := rulesMatch(rules, test.geofenceID, test.vehicleID, test.eventType); got != test.want {
				t.Fatalf("rulesMatch() = %v, want %v", got, test.want)
			}
		})
	}
}

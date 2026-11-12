package app

import (
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/dewanshpatle/geofencing-alert-system/backend/internal/geofence"
)

const maxRequestBody = 1 << 20

type API struct {
	store         *Store
	hub           *Hub
	allowedOrigins map[string]struct{}
}

func NewAPI(store *Store, hub *Hub, allowedOrigins []string) *API {
	origins := make(map[string]struct{}, len(allowedOrigins))
	for _, origin := range allowedOrigins {
		origin = strings.TrimSpace(origin)
		if origin != "" {
			origins[origin] = struct{}{}
		}
	}
	return &API{store: store, hub: hub, allowedOrigins: origins}
}

func (a *API) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	started := time.Now()
	origin := r.Header.Get("Origin")
	if origin != "" && !a.originAllowed(origin) {
		a.writeError(w, http.StatusForbidden, "origin is not allowed", started)
		return
	}
	if origin != "" {
		w.Header().Set("Access-Control-Allow-Origin", origin)
		w.Header().Add("Vary", "Origin")
	}
	w.Header().Set("Access-Control-Allow-Methods", "GET, POST, OPTIONS")
	w.Header().Set("Access-Control-Allow-Headers", "Content-Type")
	if r.Method == http.MethodOptions {
		a.writeJSON(w, http.StatusOK, map[string]any{"ok": true}, started)
		return
	}

	if r.URL.Path == "/ws/alerts" {
		if r.Method != http.MethodGet {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.hub.ServeHTTP(w, r)
		return
	}
	if r.URL.Path == "/healthz" {
		if r.Method != http.MethodGet {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.writeJSON(w, http.StatusOK, map[string]any{"status": "ok"}, started)
		return
	}

	switch {
	case r.URL.Path == "/geofences":
		if r.Method == http.MethodPost {
			a.createGeofence(w, r, started)
		} else if r.Method == http.MethodGet {
			a.listGeofences(w, r, started)
		} else {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
		}
	case r.URL.Path == "/vehicles":
		if r.Method == http.MethodPost {
			a.createVehicle(w, r, started)
		} else if r.Method == http.MethodGet {
			a.listVehicles(w, r, started)
		} else {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
		}
	case r.URL.Path == "/vehicles/location":
		if r.Method != http.MethodPost {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.updateLocation(w, r, started)
	case strings.HasPrefix(r.URL.Path, "/vehicles/location/"):
		if r.Method != http.MethodGet {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.getLocation(w, r, started)
	case r.URL.Path == "/alerts/configure":
		if r.Method != http.MethodPost {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.configureAlert(w, r, started)
	case r.URL.Path == "/alerts":
		if r.Method != http.MethodGet {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.listAlerts(w, r, started)
	case r.URL.Path == "/violations/history":
		if r.Method != http.MethodGet {
			a.writeError(w, http.StatusMethodNotAllowed, "method not allowed", started)
			return
		}
		a.listHistory(w, r, started)
	default:
		a.writeError(w, http.StatusNotFound, "endpoint not found", started)
	}
}

func (a *API) originAllowed(origin string) bool {
	if _, ok := a.allowedOrigins["*"]; ok {
		return true
	}
	if _, ok := a.allowedOrigins[origin]; ok {
		return true
	}
	return false
}

func (a *API) writeJSON(w http.ResponseWriter, status int, body map[string]any, started time.Time) {
	body["time_ns"] = strconv.FormatInt(time.Since(started).Nanoseconds(), 10)
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	if err := json.NewEncoder(w).Encode(body); err != nil {
		log.Printf("write API response: %v", err)
	}
}

func (a *API) writeError(w http.ResponseWriter, status int, message string, started time.Time) {
	a.writeJSON(w, status, map[string]any{"error": message}, started)
}

func (a *API) readJSON(w http.ResponseWriter, r *http.Request, destination any, started time.Time) bool {
	r.Body = http.MaxBytesReader(w, r.Body, maxRequestBody)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		a.writeError(w, http.StatusBadRequest, "invalid JSON request body", started)
		return false
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		a.writeError(w, http.StatusBadRequest, "request body must contain a single JSON object", started)
		return false
	}
	return true
}

func (a *API) createGeofence(w http.ResponseWriter, r *http.Request, started time.Time) {
	var request struct {
		Name        string      `json:"name"`
		Description string      `json:"description"`
		Coordinates [][]float64 `json:"coordinates"`
		Category    string      `json:"category"`
	}
	if !a.readJSON(w, r, &request, started) {
		return
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" || len(request.Name) > 120 {
		a.writeError(w, http.StatusBadRequest, "name is required and must be at most 120 characters", started)
		return
	}
	if len(request.Description) > 2000 {
		a.writeError(w, http.StatusBadRequest, "description must be at most 2000 characters", started)
		return
	}
	if !ValidCategory(request.Category) {
		a.writeError(w, http.StatusBadRequest, "category must be delivery_zone, restricted_zone, toll_zone, or customer_area", started)
		return
	}
	if err := geofence.Validate(request.Coordinates); err != nil {
		a.writeError(w, http.StatusBadRequest, "coordinates must be a closed polygon with at least three unique vertices and valid latitude/longitude ranges", started)
		return
	}
	fence, err := a.store.CreateGeofence(r.Context(), request.Name, strings.TrimSpace(request.Description), request.Category, request.Coordinates)
	if err != nil {
		log.Printf("create geofence: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not create geofence", started)
		return
	}
	a.writeJSON(w, http.StatusCreated, map[string]any{"id": fence.ID, "name": fence.Name, "status": "active"}, started)
}

func (a *API) listGeofences(w http.ResponseWriter, r *http.Request, started time.Time) {
	category := r.URL.Query().Get("category")
	if category != "" && !ValidCategory(category) {
		a.writeError(w, http.StatusBadRequest, "unsupported geofence category", started)
		return
	}
	fences, err := a.store.ListGeofences(r.Context(), category)
	if err != nil {
		log.Printf("list geofences: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not retrieve geofences", started)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"geofences": fences}, started)
}

func (a *API) createVehicle(w http.ResponseWriter, r *http.Request, started time.Time) {
	var request struct {
		VehicleNumber string `json:"vehicle_number"`
		DriverName    string `json:"driver_name"`
		VehicleType   string `json:"vehicle_type"`
		Phone         string `json:"phone"`
	}
	if !a.readJSON(w, r, &request, started) {
		return
	}
	request.VehicleNumber = strings.TrimSpace(request.VehicleNumber)
	request.DriverName = strings.TrimSpace(request.DriverName)
	request.VehicleType = strings.TrimSpace(request.VehicleType)
	request.Phone = strings.TrimSpace(request.Phone)
	if request.VehicleNumber == "" || request.DriverName == "" || request.VehicleType == "" || request.Phone == "" {
		a.writeError(w, http.StatusBadRequest, "vehicle_number, driver_name, vehicle_type, and phone are required", started)
		return
	}
	if len(request.VehicleNumber) > 40 || len(request.DriverName) > 120 || len(request.VehicleType) > 40 || len(request.Phone) > 40 {
		a.writeError(w, http.StatusBadRequest, "vehicle fields exceed their maximum length", started)
		return
	}
	vehicle, err := a.store.CreateVehicle(r.Context(), request.VehicleNumber, request.DriverName, request.VehicleType, request.Phone)
	if errors.Is(err, ErrConflict) {
		a.writeError(w, http.StatusConflict, "a vehicle with this vehicle_number already exists", started)
		return
	}
	if err != nil {
		log.Printf("create vehicle: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not register vehicle", started)
		return
	}
	a.writeJSON(w, http.StatusCreated, map[string]any{"id": vehicle.ID, "vehicle_number": vehicle.VehicleNumber, "status": vehicle.Status}, started)
}

func (a *API) listVehicles(w http.ResponseWriter, r *http.Request, started time.Time) {
	vehicles, err := a.store.ListVehicles(r.Context())
	if err != nil {
		log.Printf("list vehicles: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not retrieve vehicles", started)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"vehicles": vehicles}, started)
}

func (a *API) updateLocation(w http.ResponseWriter, r *http.Request, started time.Time) {
	var request struct {
		VehicleID string  `json:"vehicle_id"`
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
		Timestamp string  `json:"timestamp"`
	}
	if !a.readJSON(w, r, &request, started) {
		return
	}
	request.VehicleID = strings.TrimSpace(request.VehicleID)
	if request.VehicleID == "" {
		a.writeError(w, http.StatusBadRequest, "vehicle_id is required", started)
		return
	}
	if request.Latitude < -90 || request.Latitude > 90 || request.Longitude < -180 || request.Longitude > 180 {
		a.writeError(w, http.StatusBadRequest, "latitude must be between -90 and 90 and longitude between -180 and 180", started)
		return
	}
	at, err := time.Parse(time.RFC3339Nano, request.Timestamp)
	if err != nil {
		a.writeError(w, http.StatusBadRequest, "timestamp must be an ISO 8601 timestamp", started)
		return
	}
	result, err := a.store.UpdateLocation(r.Context(), request.VehicleID, request.Latitude, request.Longitude, at)
	if errors.Is(err, ErrNotFound) {
		a.writeError(w, http.StatusNotFound, "vehicle not found", started)
		return
	}
	if err != nil {
		log.Printf("update vehicle location: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not update vehicle location", started)
		return
	}
	for _, event := range result.Alerts {
		payload, err := json.Marshal(event)
		if err != nil {
			log.Printf("encode real-time alert: %v", err)
			continue
		}
		a.hub.Broadcast(payload)
	}
	a.writeJSON(w, http.StatusOK, map[string]any{
		"vehicle_id": result.VehicleID, "location_updated": result.LocationUpdated, "current_geofences": result.CurrentGeofences,
	}, started)
}

func (a *API) getLocation(w http.ResponseWriter, r *http.Request, started time.Time) {
	vehicleID := strings.Trim(strings.TrimPrefix(r.URL.Path, "/vehicles/location/"), "/")
	if vehicleID == "" || strings.Contains(vehicleID, "/") {
		a.writeError(w, http.StatusNotFound, "vehicle not found", started)
		return
	}
	vehicle, location, geofences, err := a.store.GetCurrentLocation(r.Context(), vehicleID)
	if errors.Is(err, ErrNotFound) {
		a.writeError(w, http.StatusNotFound, "vehicle not found", started)
		return
	}
	if err != nil {
		log.Printf("get vehicle location: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not retrieve vehicle location", started)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{
		"vehicle_id": vehicle.ID, "vehicle_number": vehicle.VehicleNumber, "current_location": location, "current_geofences": geofences,
	}, started)
}

func (a *API) configureAlert(w http.ResponseWriter, r *http.Request, started time.Time) {
	var request struct {
		GeofenceID string  `json:"geofence_id"`
		VehicleID  *string `json:"vehicle_id"`
		EventType  string  `json:"event_type"`
	}
	if !a.readJSON(w, r, &request, started) {
		return
	}
	request.GeofenceID = strings.TrimSpace(request.GeofenceID)
	if request.VehicleID != nil {
		trimmed := strings.TrimSpace(*request.VehicleID)
		request.VehicleID = &trimmed
		if trimmed == "" {
			a.writeError(w, http.StatusBadRequest, "vehicle_id must not be empty when provided", started)
			return
		}
	}
	if request.GeofenceID == "" {
		a.writeError(w, http.StatusBadRequest, "geofence_id is required", started)
		return
	}
	if request.EventType != "entry" && request.EventType != "exit" && request.EventType != "both" {
		a.writeError(w, http.StatusBadRequest, "event_type must be entry, exit, or both", started)
		return
	}
	alert, err := a.store.ConfigureAlert(r.Context(), request.GeofenceID, request.VehicleID, request.EventType)
	if errors.Is(err, ErrNotFound) {
		a.writeError(w, http.StatusNotFound, "geofence or vehicle not found", started)
		return
	}
	if errors.Is(err, ErrDuplicateRule) {
		a.writeError(w, http.StatusConflict, "an equivalent alert rule already exists", started)
		return
	}
	if err != nil {
		log.Printf("configure alert: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not configure alert", started)
		return
	}
	a.writeJSON(w, http.StatusCreated, map[string]any{
		"alert_id": alert.AlertID, "geofence_id": alert.GeofenceID, "vehicle_id": alert.VehicleID,
		"event_type": alert.EventType, "status": alert.Status,
	}, started)
}

func (a *API) listAlerts(w http.ResponseWriter, r *http.Request, started time.Time) {
	alerts, err := a.store.ListAlerts(r.Context(), r.URL.Query().Get("geofence_id"), r.URL.Query().Get("vehicle_id"))
	if err != nil {
		log.Printf("list alerts: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not retrieve alerts", started)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"alerts": alerts}, started)
}

func (a *API) listHistory(w http.ResponseWriter, r *http.Request, started time.Time) {
	query := r.URL.Query()
	filter := HistoryFilter{VehicleID: query.Get("vehicle_id"), GeofenceID: query.Get("geofence_id"), Limit: 50}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 500 {
			a.writeError(w, http.StatusBadRequest, "limit must be an integer between 1 and 500", started)
			return
		}
		filter.Limit = limit
	}
	if raw := query.Get("offset"); raw != "" {
		offset, err := strconv.Atoi(raw)
		if err != nil || offset < 0 {
			a.writeError(w, http.StatusBadRequest, "offset must be a non-negative integer", started)
			return
		}
		filter.Offset = offset
	}
	if raw := query.Get("start_date"); raw != "" {
		start, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "start_date must be an ISO 8601 timestamp", started)
			return
		}
		filter.StartDate = &start
	}
	if raw := query.Get("end_date"); raw != "" {
		end, err := time.Parse(time.RFC3339Nano, raw)
		if err != nil {
			a.writeError(w, http.StatusBadRequest, "end_date must be an ISO 8601 timestamp", started)
			return
		}
		filter.EndDate = &end
	}
	if filter.StartDate != nil && filter.EndDate != nil && filter.StartDate.After(*filter.EndDate) {
		a.writeError(w, http.StatusBadRequest, "start_date must not be after end_date", started)
		return
	}
	result, err := a.store.ListHistory(r.Context(), filter)
	if err != nil {
		log.Printf("list violation history: %v", err)
		a.writeError(w, http.StatusInternalServerError, "could not retrieve violation history", started)
		return
	}
	a.writeJSON(w, http.StatusOK, map[string]any{"violations": result.Violations, "total_count": result.TotalCount}, started)
}

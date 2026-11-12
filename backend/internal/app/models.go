package app

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrNotFound     = errors.New("resource not found")
	ErrConflict     = errors.New("resource already exists")
	ErrDuplicateRule = errors.New("an equivalent alert rule already exists")
)

var categories = map[string]struct{}{
	"delivery_zone":  {},
	"restricted_zone": {},
	"toll_zone":       {},
	"customer_area":   {},
}

func ValidCategory(category string) bool {
	_, ok := categories[category]
	return ok
}

type Geofence struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description"`
	Coordinates [][]float64 `json:"coordinates"`
	Category    string      `json:"category"`
	CreatedAt   string      `json:"created_at"`
}

type Vehicle struct {
	ID           string `json:"id"`
	VehicleNumber string `json:"vehicle_number"`
	DriverName   string `json:"driver_name"`
	VehicleType  string `json:"vehicle_type"`
	Phone        string `json:"phone"`
	Status       string `json:"status"`
	CreatedAt    string `json:"created_at"`
}

type AlertRule struct {
	AlertID       string  `json:"alert_id"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	VehicleID     *string `json:"vehicle_id"`
	VehicleNumber *string `json:"vehicle_number"`
	EventType     string  `json:"event_type"`
	Status        string  `json:"status"`
	CreatedAt     string  `json:"created_at"`
}

type CurrentGeofence struct {
	GeofenceID   string `json:"geofence_id"`
	GeofenceName string `json:"geofence_name"`
	Status       string `json:"status,omitempty"`
	Category     string `json:"category,omitempty"`
}

type Location struct {
	Latitude  float64 `json:"latitude"`
	Longitude float64 `json:"longitude"`
	Timestamp string  `json:"timestamp"`
}

type LocationResult struct {
	VehicleID        string            `json:"vehicle_id"`
	LocationUpdated  bool              `json:"location_updated"`
	CurrentGeofences []CurrentGeofence `json:"current_geofences"`
	Alerts           []EventMessage    `json:"-"`
}

type Violation struct {
	ID            string  `json:"id"`
	VehicleID     string  `json:"vehicle_id"`
	VehicleNumber string  `json:"vehicle_number"`
	GeofenceID    string  `json:"geofence_id"`
	GeofenceName  string  `json:"geofence_name"`
	EventType     string  `json:"event_type"`
	Latitude      float64 `json:"latitude"`
	Longitude     float64 `json:"longitude"`
	Timestamp     string  `json:"timestamp"`
}

type EventMessage struct {
	EventID   string    `json:"event_id"`
	EventType string    `json:"event_type"`
	Timestamp time.Time `json:"timestamp"`
	Vehicle   struct {
		VehicleID   string `json:"vehicle_id"`
		VehicleNumber string `json:"vehicle_number"`
		DriverName  string `json:"driver_name"`
	} `json:"vehicle"`
	Geofence struct {
		GeofenceID   string `json:"geofence_id"`
		GeofenceName string `json:"geofence_name"`
		Category     string `json:"category"`
	} `json:"geofence"`
	Location struct {
		Latitude  float64 `json:"latitude"`
		Longitude float64 `json:"longitude"`
	} `json:"location"`
}

type HistoryFilter struct {
	VehicleID  string
	GeofenceID string
	StartDate  *time.Time
	EndDate    *time.Time
	Limit      int
	Offset     int
}

type HistoryResult struct {
	Violations []Violation `json:"violations"`
	TotalCount int         `json:"total_count"`
}

func id(prefix string) (string, error) {
	random := make([]byte, 12)
	if _, err := rand.Read(random); err != nil {
		return "", err
	}
	return prefix + hex.EncodeToString(random), nil
}

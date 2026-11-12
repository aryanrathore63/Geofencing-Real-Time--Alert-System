package app

import (
	"context"
	"database/sql"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/dewanshpatle/geofencing-alert-system/backend/internal/geofence"
	_ "modernc.org/sqlite"
)

//go:embed migrations/001_init.sql
var migrations embed.FS

type Store struct {
	db *sql.DB
}

func OpenStore(path string) (*Store, error) {
	if path != ":memory:" {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			return nil, fmt.Errorf("create database directory: %w", err)
		}
	}
	dsn := path
	if path == ":memory:" {
		dsn = "file:geofence?mode=memory&cache=shared&_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	} else {
		dsn = "file:" + filepath.ToSlash(path) + "?_pragma=foreign_keys(1)&_pragma=busy_timeout(5000)"
	}
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}
	db.SetMaxOpenConns(1)
	db.SetMaxIdleConns(1)
	store := &Store{db: db}
	if err := store.Migrate(context.Background()); err != nil {
		_ = db.Close()
		return nil, err
	}
	if path != ":memory:" {
		if _, err := db.Exec("PRAGMA journal_mode=WAL"); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("enable SQLite WAL: %w", err)
		}
	}
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) Migrate(ctx context.Context) error {
	schema, err := migrations.ReadFile("migrations/001_init.sql")
	if err != nil {
		return fmt.Errorf("read database migration: %w", err)
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin database migration: %w", err)
	}
	defer tx.Rollback()
	for _, statement := range strings.Split(string(schema), ";") {
		if strings.TrimSpace(statement) == "" {
			continue
		}
		if _, err := tx.ExecContext(ctx, statement); err != nil {
			return fmt.Errorf("apply database migration: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit database migration: %w", err)
	}
	return nil
}

func (s *Store) CreateGeofence(ctx context.Context, name, description, category string, coordinates [][]float64) (Geofence, error) {
	newID, err := id("geo_")
	if err != nil {
		return Geofence{}, err
	}
	encoded, err := json.Marshal(coordinates)
	if err != nil {
		return Geofence{}, err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO geofences (id, name, description, coordinates, category, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		newID, name, description, string(encoded), category, createdAt,
	)
	if err != nil {
		return Geofence{}, err
	}
	return Geofence{ID: newID, Name: name, Description: description, Coordinates: coordinates, Category: category, CreatedAt: createdAt}, nil
}

func (s *Store) ListGeofences(ctx context.Context, category string) ([]Geofence, error) {
	query := `SELECT id, name, description, coordinates, category, created_at FROM geofences`
	args := []any{}
	if category != "" {
		query += ` WHERE category = ?`
		args = append(args, category)
	}
	query += ` ORDER BY created_at DESC, id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	geofences := make([]Geofence, 0)
	for rows.Next() {
		fence, err := scanGeofence(rows)
		if err != nil {
			return nil, err
		}
		geofences = append(geofences, fence)
	}
	return geofences, rows.Err()
}

func scanGeofence(row interface{ Scan(...any) error }) (Geofence, error) {
	var fence Geofence
	var coordinates string
	if err := row.Scan(&fence.ID, &fence.Name, &fence.Description, &coordinates, &fence.Category, &fence.CreatedAt); err != nil {
		return Geofence{}, err
	}
	if err := json.Unmarshal([]byte(coordinates), &fence.Coordinates); err != nil {
		return Geofence{}, fmt.Errorf("decode stored geofence coordinates: %w", err)
	}
	return fence, nil
}

func (s *Store) CreateVehicle(ctx context.Context, number, driver, vehicleType, phone string) (Vehicle, error) {
	var existing string
	err := s.db.QueryRowContext(ctx, `SELECT id FROM vehicles WHERE vehicle_number = ?`, number).Scan(&existing)
	if err == nil {
		return Vehicle{}, ErrConflict
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return Vehicle{}, err
	}
	newID, err := id("veh_")
	if err != nil {
		return Vehicle{}, err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO vehicles (id, vehicle_number, driver_name, vehicle_type, phone, status, created_at) VALUES (?, ?, ?, ?, ?, 'active', ?)`,
		newID, number, driver, vehicleType, phone, createdAt,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return Vehicle{}, ErrConflict
		}
		return Vehicle{}, err
	}
	return Vehicle{ID: newID, VehicleNumber: number, DriverName: driver, VehicleType: vehicleType, Phone: phone, Status: "active", CreatedAt: createdAt}, nil
}

func isUniqueConstraint(err error) bool {
	return err != nil && strings.Contains(strings.ToLower(err.Error()), "unique constraint")
}

func (s *Store) ListVehicles(ctx context.Context) ([]Vehicle, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id, vehicle_number, driver_name, vehicle_type, phone, status, created_at FROM vehicles ORDER BY created_at DESC, id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	vehicles := make([]Vehicle, 0)
	for rows.Next() {
		var vehicle Vehicle
		if err := rows.Scan(&vehicle.ID, &vehicle.VehicleNumber, &vehicle.DriverName, &vehicle.VehicleType, &vehicle.Phone, &vehicle.Status, &vehicle.CreatedAt); err != nil {
			return nil, err
		}
		vehicles = append(vehicles, vehicle)
	}
	return vehicles, rows.Err()
}

func (s *Store) ConfigureAlert(ctx context.Context, geofenceID string, vehicleID *string, eventType string) (AlertRule, error) {
	var fenceName string
	if err := s.db.QueryRowContext(ctx, `SELECT name FROM geofences WHERE id = ?`, geofenceID).Scan(&fenceName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return AlertRule{}, ErrNotFound
		}
		return AlertRule{}, err
	}
	var vehicleNumber *string
	if vehicleID != nil {
		var number string
		if err := s.db.QueryRowContext(ctx, `SELECT vehicle_number FROM vehicles WHERE id = ?`, *vehicleID).Scan(&number); err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return AlertRule{}, ErrNotFound
			}
			return AlertRule{}, err
		}
		vehicleNumber = &number
	}
	var existing string
	err := s.db.QueryRowContext(ctx,
		`SELECT alert_id FROM alert_rules WHERE geofence_id = ? AND IFNULL(vehicle_id, '') = IFNULL(?, '') AND event_type = ?`,
		geofenceID, vehicleID, eventType,
	).Scan(&existing)
	if err == nil {
		return AlertRule{}, ErrDuplicateRule
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return AlertRule{}, err
	}
	newID, err := id("alert_")
	if err != nil {
		return AlertRule{}, err
	}
	createdAt := time.Now().UTC().Format(time.RFC3339Nano)
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO alert_rules (alert_id, geofence_id, vehicle_id, event_type, status, created_at) VALUES (?, ?, ?, ?, 'active', ?)`,
		newID, geofenceID, vehicleID, eventType, createdAt,
	)
	if err != nil {
		if isUniqueConstraint(err) {
			return AlertRule{}, ErrDuplicateRule
		}
		return AlertRule{}, err
	}
	return AlertRule{
		AlertID: newID, GeofenceID: geofenceID, GeofenceName: fenceName, VehicleID: vehicleID,
		VehicleNumber: vehicleNumber, EventType: eventType, Status: "active", CreatedAt: createdAt,
	}, nil
}

func (s *Store) ListAlerts(ctx context.Context, geofenceID, vehicleID string) ([]AlertRule, error) {
	query := `SELECT a.alert_id, a.geofence_id, g.name, a.vehicle_id, v.vehicle_number, a.event_type, a.status, a.created_at
		FROM alert_rules a JOIN geofences g ON g.id = a.geofence_id
		LEFT JOIN vehicles v ON v.id = a.vehicle_id WHERE 1 = 1`
	args := []any{}
	if geofenceID != "" {
		query += ` AND a.geofence_id = ?`
		args = append(args, geofenceID)
	}
	if vehicleID != "" {
		query += ` AND a.vehicle_id = ?`
		args = append(args, vehicleID)
	}
	query += ` ORDER BY a.created_at DESC, a.alert_id`
	rows, err := s.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	alerts := make([]AlertRule, 0)
	for rows.Next() {
		var alert AlertRule
		var idValue, numberValue sql.NullString
		if err := rows.Scan(&alert.AlertID, &alert.GeofenceID, &alert.GeofenceName, &idValue, &numberValue, &alert.EventType, &alert.Status, &alert.CreatedAt); err != nil {
			return nil, err
		}
		if idValue.Valid {
			alert.VehicleID = &idValue.String
		}
		if numberValue.Valid {
			alert.VehicleNumber = &numberValue.String
		}
		alerts = append(alerts, alert)
	}
	return alerts, rows.Err()
}

func (s *Store) UpdateLocation(ctx context.Context, vehicleID string, latitude, longitude float64, at time.Time) (LocationResult, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return LocationResult{}, err
	}
	defer tx.Rollback()

	var vehicle Vehicle
	if err := tx.QueryRowContext(ctx,
		`SELECT id, vehicle_number, driver_name FROM vehicles WHERE id = ?`, vehicleID,
	).Scan(&vehicle.ID, &vehicle.VehicleNumber, &vehicle.DriverName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return LocationResult{}, ErrNotFound
		}
		return LocationResult{}, err
	}
	geofences, err := listGeofencesTx(ctx, tx)
	if err != nil {
		return LocationResult{}, err
	}
	previous := make(map[string]bool)
	rows, err := tx.QueryContext(ctx, `SELECT geofence_id, inside FROM vehicle_geofence_states WHERE vehicle_id = ?`, vehicleID)
	if err != nil {
		return LocationResult{}, err
	}
	for rows.Next() {
		var fenceID string
		var inside int
		if err := rows.Scan(&fenceID, &inside); err != nil {
			_ = rows.Close()
			return LocationResult{}, err
		}
		previous[fenceID] = inside == 1
	}
	if err := rows.Err(); err != nil {
		_ = rows.Close()
		return LocationResult{}, err
	}
	if err := rows.Close(); err != nil {
		return LocationResult{}, err
	}
	rules, err := listActiveRulesTx(ctx, tx)
	if err != nil {
		return LocationResult{}, err
	}

	timestamp := at.UTC().Format(time.RFC3339Nano)
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO locations (vehicle_id, latitude, longitude, timestamp, recorded_at) VALUES (?, ?, ?, ?, ?)`,
		vehicleID, latitude, longitude, timestamp, time.Now().UTC().Format(time.RFC3339Nano),
	); err != nil {
		return LocationResult{}, err
	}

	result := LocationResult{VehicleID: vehicleID, LocationUpdated: true, CurrentGeofences: make([]CurrentGeofence, 0)}
	for _, fence := range geofences {
		inside := geofence.Contains(fence.Coordinates, latitude, longitude)
		state := 0
		if inside {
			state = 1
			result.CurrentGeofences = append(result.CurrentGeofences, CurrentGeofence{
				GeofenceID: fence.ID, GeofenceName: fence.Name, Status: "inside",
			})
		}
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO vehicle_geofence_states (vehicle_id, geofence_id, inside, updated_at) VALUES (?, ?, ?, ?)
			 ON CONFLICT(vehicle_id, geofence_id) DO UPDATE SET inside = excluded.inside, updated_at = excluded.updated_at`,
			vehicleID, fence.ID, state, timestamp,
		); err != nil {
			return LocationResult{}, err
		}

		eventType := ""
		if previous[fence.ID] != inside {
			if inside {
				eventType = "entry"
			} else {
				eventType = "exit"
			}
		}
		if eventType == "" {
			continue
		}
		if err := s.recordTransition(ctx, tx, vehicle, fence, eventType, latitude, longitude, at); err != nil {
			return LocationResult{}, err
		}
		if rulesMatch(rules, fence.ID, vehicleID, eventType) {
			message, err := newEventMessage(vehicle, fence, eventType, latitude, longitude, at)
			if err != nil {
				return LocationResult{}, err
			}
			result.Alerts = append(result.Alerts, message)
		}
	}
	if err := tx.Commit(); err != nil {
		return LocationResult{}, err
	}
	return result, nil
}

func listGeofencesTx(ctx context.Context, tx *sql.Tx) ([]Geofence, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, name, description, coordinates, category, created_at FROM geofences ORDER BY id`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	result := make([]Geofence, 0)
	for rows.Next() {
		fence, err := scanGeofence(rows)
		if err != nil {
			return nil, err
		}
		result = append(result, fence)
	}
	return result, rows.Err()
}

func listActiveRulesTx(ctx context.Context, tx *sql.Tx) ([]AlertRule, error) {
	rows, err := tx.QueryContext(ctx, `SELECT geofence_id, vehicle_id, event_type FROM alert_rules WHERE status = 'active'`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	rules := make([]AlertRule, 0)
	for rows.Next() {
		var rule AlertRule
		var vehicleID sql.NullString
		if err := rows.Scan(&rule.GeofenceID, &vehicleID, &rule.EventType); err != nil {
			return nil, err
		}
		if vehicleID.Valid {
			rule.VehicleID = &vehicleID.String
		}
		rules = append(rules, rule)
	}
	return rules, rows.Err()
}

func rulesMatch(rules []AlertRule, geofenceID, vehicleID, eventType string) bool {
	for _, rule := range rules {
		if rule.GeofenceID != geofenceID || (rule.VehicleID != nil && *rule.VehicleID != vehicleID) {
			continue
		}
		if rule.EventType == "both" || rule.EventType == eventType {
			return true
		}
	}
	return false
}

func (s *Store) recordTransition(ctx context.Context, tx *sql.Tx, vehicle Vehicle, fence Geofence, eventType string, latitude, longitude float64, at time.Time) error {
	violationID, err := id("viol_")
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx,
		`INSERT INTO violations (id, vehicle_id, vehicle_number, driver_name, geofence_id, geofence_name, category, event_type, latitude, longitude, timestamp, timestamp_unix_nano)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		violationID, vehicle.ID, vehicle.VehicleNumber, vehicle.DriverName, fence.ID, fence.Name, fence.Category,
		eventType, latitude, longitude, at.UTC().Format(time.RFC3339Nano), at.UnixNano(),
	)
	return err
}

func newEventMessage(vehicle Vehicle, fence Geofence, eventType string, latitude, longitude float64, at time.Time) (EventMessage, error) {
	eventID, err := id("evt_")
	if err != nil {
		return EventMessage{}, err
	}
	message := EventMessage{EventID: eventID, EventType: eventType, Timestamp: at.UTC()}
	message.Vehicle.VehicleID = vehicle.ID
	message.Vehicle.VehicleNumber = vehicle.VehicleNumber
	message.Vehicle.DriverName = vehicle.DriverName
	message.Geofence.GeofenceID = fence.ID
	message.Geofence.GeofenceName = fence.Name
	message.Geofence.Category = fence.Category
	message.Location.Latitude = latitude
	message.Location.Longitude = longitude
	return message, nil
}

func (s *Store) GetCurrentLocation(ctx context.Context, vehicleID string) (Vehicle, *Location, []CurrentGeofence, error) {
	var vehicle Vehicle
	if err := s.db.QueryRowContext(ctx,
		`SELECT id, vehicle_number, driver_name FROM vehicles WHERE id = ?`, vehicleID,
	).Scan(&vehicle.ID, &vehicle.VehicleNumber, &vehicle.DriverName); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Vehicle{}, nil, nil, ErrNotFound
		}
		return Vehicle{}, nil, nil, err
	}
	var current Location
	err := s.db.QueryRowContext(ctx,
		`SELECT latitude, longitude, timestamp FROM locations WHERE vehicle_id = ? ORDER BY id DESC LIMIT 1`, vehicleID,
	).Scan(&current.Latitude, &current.Longitude, &current.Timestamp)
	if errors.Is(err, sql.ErrNoRows) {
		return vehicle, nil, make([]CurrentGeofence, 0), nil
	}
	if err != nil {
		return Vehicle{}, nil, nil, err
	}
	fences, err := s.ListGeofences(ctx, "")
	if err != nil {
		return Vehicle{}, nil, nil, err
	}
	currentGeofences := make([]CurrentGeofence, 0)
	for _, fence := range fences {
		if geofence.Contains(fence.Coordinates, current.Latitude, current.Longitude) {
			currentGeofences = append(currentGeofences, CurrentGeofence{
				GeofenceID: fence.ID, GeofenceName: fence.Name, Category: fence.Category,
			})
		}
	}
	return vehicle, &current, currentGeofences, nil
}

func (s *Store) ListHistory(ctx context.Context, filter HistoryFilter) (HistoryResult, error) {
	where := ` WHERE 1 = 1`
	args := []any{}
	if filter.VehicleID != "" {
		where += ` AND vehicle_id = ?`
		args = append(args, filter.VehicleID)
	}
	if filter.GeofenceID != "" {
		where += ` AND geofence_id = ?`
		args = append(args, filter.GeofenceID)
	}
	if filter.StartDate != nil {
		where += ` AND timestamp_unix_nano >= ?`
		args = append(args, filter.StartDate.UnixNano())
	}
	if filter.EndDate != nil {
		where += ` AND timestamp_unix_nano <= ?`
		args = append(args, filter.EndDate.UnixNano())
	}
	var total int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM violations`+where, args...).Scan(&total); err != nil {
		return HistoryResult{}, err
	}
	queryArgs := append(append([]any(nil), args...), filter.Limit, filter.Offset)
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, vehicle_id, vehicle_number, geofence_id, geofence_name, event_type, latitude, longitude, timestamp
		 FROM violations`+where+` ORDER BY timestamp_unix_nano DESC, id DESC LIMIT ? OFFSET ?`,
		queryArgs...,
	)
	if err != nil {
		return HistoryResult{}, err
	}
	defer rows.Close()
	violations := make([]Violation, 0)
	for rows.Next() {
		var item Violation
		if err := rows.Scan(&item.ID, &item.VehicleID, &item.VehicleNumber, &item.GeofenceID, &item.GeofenceName,
			&item.EventType, &item.Latitude, &item.Longitude, &item.Timestamp); err != nil {
			return HistoryResult{}, err
		}
		violations = append(violations, item)
	}
	if err := rows.Err(); err != nil {
		return HistoryResult{}, err
	}
	return HistoryResult{Violations: violations, TotalCount: total}, nil
}

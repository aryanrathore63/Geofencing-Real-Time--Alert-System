CREATE TABLE IF NOT EXISTS geofences (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT NOT NULL DEFAULT '',
    coordinates TEXT NOT NULL,
    category TEXT NOT NULL CHECK (category IN ('delivery_zone', 'restricted_zone', 'toll_zone', 'customer_area')),
    created_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS geofences_category_idx ON geofences(category);

CREATE TABLE IF NOT EXISTS vehicles (
    id TEXT PRIMARY KEY,
    vehicle_number TEXT NOT NULL UNIQUE COLLATE NOCASE,
    driver_name TEXT NOT NULL,
    vehicle_type TEXT NOT NULL,
    phone TEXT NOT NULL,
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS locations (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    timestamp TEXT NOT NULL,
    recorded_at TEXT NOT NULL
);

CREATE INDEX IF NOT EXISTS locations_vehicle_latest_idx ON locations(vehicle_id, id DESC);

CREATE TABLE IF NOT EXISTS vehicle_geofence_states (
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    inside INTEGER NOT NULL CHECK (inside IN (0, 1)),
    updated_at TEXT NOT NULL,
    PRIMARY KEY (vehicle_id, geofence_id)
);

CREATE TABLE IF NOT EXISTS alert_rules (
    alert_id TEXT PRIMARY KEY,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    vehicle_id TEXT REFERENCES vehicles(id) ON DELETE CASCADE,
    event_type TEXT NOT NULL CHECK (event_type IN ('entry', 'exit', 'both')),
    status TEXT NOT NULL DEFAULT 'active',
    created_at TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS alert_rules_unique_idx
    ON alert_rules(geofence_id, IFNULL(vehicle_id, ''), event_type);

CREATE TABLE IF NOT EXISTS violations (
    id TEXT PRIMARY KEY,
    vehicle_id TEXT NOT NULL REFERENCES vehicles(id) ON DELETE CASCADE,
    vehicle_number TEXT NOT NULL,
    driver_name TEXT NOT NULL,
    geofence_id TEXT NOT NULL REFERENCES geofences(id) ON DELETE CASCADE,
    geofence_name TEXT NOT NULL,
    category TEXT NOT NULL,
    event_type TEXT NOT NULL CHECK (event_type IN ('entry', 'exit')),
    latitude REAL NOT NULL,
    longitude REAL NOT NULL,
    timestamp TEXT NOT NULL,
    timestamp_unix_nano INTEGER NOT NULL
);

CREATE INDEX IF NOT EXISTS violations_timestamp_idx ON violations(timestamp_unix_nano DESC);
CREATE INDEX IF NOT EXISTS violations_vehicle_idx ON violations(vehicle_id, timestamp_unix_nano DESC);
CREATE INDEX IF NOT EXISTS violations_geofence_idx ON violations(geofence_id, timestamp_unix_nano DESC);

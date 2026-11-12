export type GeofenceCategory =
  | "delivery_zone"
  | "restricted_zone"
  | "toll_zone"
  | "customer_area";

export type EventType = "entry" | "exit" | "both";

export interface Geofence {
  id: string;
  name: string;
  description: string;
  coordinates: [number, number][];
  category: GeofenceCategory;
  created_at: string;
}

export interface Vehicle {
  id: string;
  vehicle_number: string;
  driver_name: string;
  vehicle_type: string;
  phone: string;
  status: string;
  created_at: string;
}

export interface AlertRule {
  alert_id: string;
  geofence_id: string;
  geofence_name: string;
  vehicle_id: string | null;
  vehicle_number: string | null;
  event_type: EventType;
  status: string;
  created_at: string;
}

export interface CurrentGeofence {
  geofence_id: string;
  geofence_name: string;
  status?: "inside";
  category?: GeofenceCategory;
}

export interface Violation {
  id: string;
  vehicle_id: string;
  vehicle_number: string;
  geofence_id: string;
  geofence_name: string;
  event_type: "entry" | "exit";
  latitude: number;
  longitude: number;
  timestamp: string;
}

export interface LiveEvent {
  event_id: string;
  event_type: "entry" | "exit";
  timestamp: string;
  vehicle: {
    vehicle_id: string;
    vehicle_number: string;
    driver_name: string;
  };
  geofence: {
    geofence_id: string;
    geofence_name: string;
    category: GeofenceCategory;
  };
  location: {
    latitude: number;
    longitude: number;
  };
}

export interface LocationSnapshot {
  vehicle_id: string;
  vehicle_number: string;
  current_location: {
    latitude: number;
    longitude: number;
    timestamp: string;
  } | null;
  current_geofences: CurrentGeofence[];
}

export interface HistoryResponse {
  violations: Violation[];
  total_count: number;
}

export interface APIError {
  error?: string;
  time_ns?: string;
}

import type {
  APIError,
  AlertRule,
  Geofence,
  HistoryResponse,
  LiveEvent,
  LocationSnapshot,
  Vehicle,
} from "./types";

const apiBase = (import.meta.env.VITE_API_BASE_URL ?? "").replace(/\/+$/, "");

export class RequestError extends Error {
  readonly status: number;

  constructor(message: string, status: number) {
    super(message);
    this.name = "RequestError";
    this.status = status;
  }
}

async function request<T>(path: string, init?: RequestInit): Promise<T> {
  let response: Response;
  try {
    response = await fetch(`${apiBase}${path}`, {
      ...init,
      headers: {
        ...(init?.body ? { "Content-Type": "application/json" } : {}),
        ...init?.headers,
      },
    });
  } catch {
    throw new RequestError(
      "The API could not be reached. Check the backend URL and try again.",
      0,
    );
  }

  let body: T & APIError;
  try {
    body = (await response.json()) as T & APIError;
  } catch {
    throw new RequestError("The API returned an unreadable response.", response.status);
  }
  if (!response.ok) {
    throw new RequestError(
      body.error ?? `Request failed with status ${response.status}.`,
      response.status,
    );
  }
  return body;
}

function post<T>(path: string, payload: unknown): Promise<T> {
  return request<T>(path, { method: "POST", body: JSON.stringify(payload) });
}

export const api = {
  geofences: (category = "") =>
    request<{ geofences: Geofence[] }>(
      `/geofences${category ? `?category=${encodeURIComponent(category)}` : ""}`,
    ),
  createGeofence: (payload: {
    name: string;
    description: string;
    coordinates: [number, number][];
    category: Geofence["category"];
  }) => post<{ id: string; name: string; status: string }>("/geofences", payload),
  vehicles: () => request<{ vehicles: Vehicle[] }>("/vehicles"),
  createVehicle: (payload: {
    vehicle_number: string;
    driver_name: string;
    vehicle_type: string;
    phone: string;
  }) => post<{ id: string; vehicle_number: string; status: string }>("/vehicles", payload),
  updateLocation: (payload: {
    vehicle_id: string;
    latitude: number;
    longitude: number;
    timestamp: string;
  }) =>
    post<{
      vehicle_id: string;
      location_updated: boolean;
      current_geofences: { geofence_id: string; geofence_name: string; status: string }[];
    }>("/vehicles/location", payload),
  location: (vehicleId: string) =>
    request<LocationSnapshot>(`/vehicles/location/${encodeURIComponent(vehicleId)}`),
  alerts: (geofenceId = "", vehicleId = "") => {
    const query = new URLSearchParams();
    if (geofenceId) query.set("geofence_id", geofenceId);
    if (vehicleId) query.set("vehicle_id", vehicleId);
    return request<{ alerts: AlertRule[] }>(
      `/alerts${query.size ? `?${query.toString()}` : ""}`,
    );
  },
  configureAlert: (payload: {
    geofence_id: string;
    vehicle_id?: string;
    event_type: "entry" | "exit" | "both";
  }) =>
    post<{
      alert_id: string;
      geofence_id: string;
      vehicle_id?: string | null;
      event_type: string;
      status: string;
    }>("/alerts/configure", payload),
  history: (filters: {
    vehicle_id?: string;
    geofence_id?: string;
    start_date?: string;
    end_date?: string;
    limit?: number;
    offset?: number;
  }) => {
    const query = new URLSearchParams();
    for (const [key, value] of Object.entries(filters)) {
      if (value !== undefined && value !== "") query.set(key, String(value));
    }
    return request<HistoryResponse>(`/violations/history?${query.toString()}`);
  },
};

export function getWebSocketURL(): string {
  const url = new URL(apiBase || window.location.origin, window.location.origin);
  url.protocol = url.protocol === "https:" ? "wss:" : "ws:";
  url.pathname = `${url.pathname.replace(/\/+$/, "")}/ws/alerts`;
  url.search = "";
  url.hash = "";
  return url.toString();
}

export function connectAlerts(
  onEvent: (event: LiveEvent) => void,
  onState: (connected: boolean) => void,
): () => void {
  if (typeof WebSocket === "undefined") {
    onState(false);
    return () => undefined;
  }
  let socket: WebSocket | null = null;
  let timer: number | undefined;
  let stopped = false;
  let retry = 0;

  const connect = () => {
    if (stopped) return;
    socket = new WebSocket(getWebSocketURL());
    socket.onopen = () => {
      retry = 0;
      onState(true);
    };
    socket.onmessage = (message) => {
      try {
        onEvent(JSON.parse(String(message.data)) as LiveEvent);
      } catch {
        onState(false);
      }
    };
    socket.onclose = () => {
      onState(false);
      if (!stopped) {
        const delay = Math.min(1000 * 2 ** retry, 15000);
        retry += 1;
        timer = window.setTimeout(connect, delay);
      }
    };
    socket.onerror = () => socket?.close();
  };

  connect();
  return () => {
    stopped = true;
    if (timer !== undefined) window.clearTimeout(timer);
    socket?.close();
  };
}

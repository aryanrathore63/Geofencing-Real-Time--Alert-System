import { useCallback, useEffect, useMemo, useState } from "react";
import type { FormEvent, ReactNode } from "react";
import {
  Activity,
  ArrowDownRight,
  ArrowUpRight,
  BellRing,
  CarFront,
  ChevronLeft,
  ChevronRight,
  Clock3,
  History,
  LayoutDashboard,
  MapPinned,
  Plus,
  Radio,
  RefreshCw,
  ShieldCheck,
  Truck,
  Wifi,
  WifiOff,
} from "lucide-react";
import { api, connectAlerts, RequestError } from "./api";
import { MapPanel } from "./components/MapPanel";
import type {
  AlertRule,
  EventType,
  Geofence,
  GeofenceCategory,
  LiveEvent,
  LocationSnapshot,
  Vehicle,
  Violation,
} from "./types";

type Section = "overview" | "geofences" | "fleet" | "rules" | "history";

const navigation: { id: Section; label: string; icon: typeof LayoutDashboard }[] = [
  { id: "overview", label: "Overview", icon: LayoutDashboard },
  { id: "geofences", label: "Geofences", icon: MapPinned },
  { id: "fleet", label: "Fleet", icon: Truck },
  { id: "rules", label: "Alert rules", icon: BellRing },
  { id: "history", label: "History", icon: History },
];

const categoryLabels: Record<GeofenceCategory, string> = {
  delivery_zone: "Delivery zone",
  restricted_zone: "Restricted zone",
  toll_zone: "Toll zone",
  customer_area: "Customer area",
};

const emptyHistory = { violations: [] as Violation[], total_count: 0 };

function messageFrom(error: unknown): string {
  if (error instanceof RequestError) return error.message;
  return error instanceof Error ? error.message : "Something went wrong. Please try again.";
}

function formatDate(value: string): string {
  const date = new Date(value);
  return Number.isNaN(date.getTime())
    ? value
    : new Intl.DateTimeFormat(undefined, { dateStyle: "medium", timeStyle: "short" }).format(date);
}

function categoryName(category: string): string {
  return categoryLabels[category as GeofenceCategory] ?? category.replaceAll("_", " ");
}

function parseCoordinates(raw: string): [number, number][] {
  const value: unknown = JSON.parse(raw);
  if (
    !Array.isArray(value) ||
    !value.every(
      (point: unknown) =>
        Array.isArray(point) &&
        point.length === 2 &&
        point.every((coordinate: unknown) => typeof coordinate === "number"),
    )
  ) {
    throw new Error("Enter a JSON array of [latitude, longitude] pairs or add points on the map.");
  }
  const points = value.map((point: [number, number]) => [point[0], point[1]] as [number, number]);
  if (
    points.length >= 3 &&
    (points[0][0] !== points.at(-1)?.[0] || points[0][1] !== points.at(-1)?.[1])
  ) {
    points.push([...points[0]]);
  }
  return points;
}

function App() {
  const [section, setSection] = useState<Section>("overview");
  const [geofences, setGeofences] = useState<Geofence[]>([]);
  const [vehicles, setVehicles] = useState<Vehicle[]>([]);
  const [alerts, setAlerts] = useState<AlertRule[]>([]);
  const [history, setHistory] = useState(emptyHistory);
  const [snapshots, setSnapshots] = useState<LocationSnapshot[]>([]);
  const [events, setEvents] = useState<LiveEvent[]>([]);
  const [loading, setLoading] = useState(true);
  const [historyLoading, setHistoryLoading] = useState(false);
  const [apiError, setAPIError] = useState("");
  const [actionError, setActionError] = useState("");
  const [actionMessage, setActionMessage] = useState("");
  const [webSocketConnected, setWebSocketConnected] = useState(false);
  const [saving, setSaving] = useState("");

  const loadData = useCallback(async (quiet = false) => {
    if (!quiet) setLoading(true);
    setAPIError("");
    try {
      const [geofenceResponse, vehicleResponse, alertResponse, historyResponse] = await Promise.all([
        api.geofences(),
        api.vehicles(),
        api.alerts(),
        api.history({ limit: 50, offset: 0 }),
      ]);
      const current = await Promise.all(
        vehicleResponse.vehicles.map((vehicle) => api.location(vehicle.id)),
      );
      setGeofences(geofenceResponse.geofences);
      setVehicles(vehicleResponse.vehicles);
      setAlerts(alertResponse.alerts);
      setHistory(historyResponse);
      setSnapshots(current);
    } catch (error) {
      setAPIError(messageFrom(error));
    } finally {
      if (!quiet) setLoading(false);
    }
  }, []);

  useEffect(() => {
    void loadData();
    return connectAlerts(
      (event) => {
        setEvents((current) => [event, ...current].slice(0, 30));
        setSnapshots((current) => {
          const previous = current.find(
            (snapshot) => snapshot.vehicle_id === event.vehicle.vehicle_id,
          );
          const zones = previous?.current_geofences ?? [];
          const currentGeofences =
            event.event_type === "entry"
              ? [
                  ...zones.filter((zone) => zone.geofence_id !== event.geofence.geofence_id),
                  {
                    geofence_id: event.geofence.geofence_id,
                    geofence_name: event.geofence.geofence_name,
                    category: event.geofence.category,
                  },
                ]
              : zones.filter((zone) => zone.geofence_id !== event.geofence.geofence_id);
          const otherVehicles = current.filter(
            (snapshot) => snapshot.vehicle_id !== event.vehicle.vehicle_id,
          );
          return [
            ...otherVehicles,
            {
              vehicle_id: event.vehicle.vehicle_id,
              vehicle_number: event.vehicle.vehicle_number,
              current_location: {
                latitude: event.location.latitude,
                longitude: event.location.longitude,
                timestamp: event.timestamp,
              },
              current_geofences: currentGeofences,
            },
          ];
        });
        void loadData(true);
      },
      setWebSocketConnected,
    );
  }, [loadData]);

  const locations = useMemo(
    () =>
      Object.fromEntries(
        snapshots.flatMap((snapshot) =>
          snapshot.current_location
            ? [[snapshot.vehicle_id, snapshot.current_location] as const]
            : [],
        ),
      ),
    [snapshots],
  );
  const liveTitle = navigation.find((item) => item.id === section)?.label ?? "Overview";

  const runAction = async <T,>(
    key: string,
    action: () => Promise<T>,
    successText: (result: T) => string,
  ) => {
    setSaving(key);
    setActionError("");
    setActionMessage("");
    try {
      const result = await action();
      setActionMessage(successText(result));
      await loadData(true);
    } catch (error) {
      setActionError(messageFrom(error));
      throw error;
    } finally {
      setSaving("");
    }
  };

  return (
    <div className="app-shell">
      <aside className="sidebar">
        <a className="brand" href="#overview" onClick={() => setSection("overview")}>
          <span className="brand-mark" aria-hidden="true">
            <Radio size={20} strokeWidth={2.2} />
          </span>
          <span>
            <strong>Waypoint</strong>
            <small>FLEET OPERATIONS</small>
          </span>
        </a>
        <div className="workspace-label">WORKSPACE</div>
        <nav className="side-nav" aria-label="Main navigation">
          {navigation.map(({ id, label, icon: Icon }) => (
            <button
              className={`nav-link${section === id ? " nav-link--active" : ""}`}
              key={id}
              type="button"
              onClick={() => {
                setSection(id);
                setActionError("");
                setActionMessage("");
              }}
              aria-current={section === id ? "page" : undefined}
            >
              <Icon size={17} aria-hidden="true" />
              <span>{label}</span>
              {id === "history" && events.length > 0 && (
                <span className="nav-badge">{events.length}</span>
              )}
            </button>
          ))}
        </nav>
        <div className="sidebar-bottom">
          <div className="connection-card">
            <span className={`connection-dot${webSocketConnected ? " is-live" : ""}`} />
            <div>
              <strong>{webSocketConnected ? "Live connection" : "Reconnecting"}</strong>
              <small>Alert stream</small>
            </div>
            {webSocketConnected ? (
              <Wifi size={16} aria-hidden="true" />
            ) : (
              <WifiOff size={16} aria-hidden="true" />
            )}
          </div>
          <span className="sidebar-footnote">GEOSPATIAL MONITORING · V1.0</span>
        </div>
      </aside>

      <main className="main-area">
        <header className="topbar">
          <div className="breadcrumbs">
            <span>Operations</span>
            <span className="crumb-slash">/</span>
            <strong>{liveTitle}</strong>
          </div>
          <div className="topbar-right">
            <span className="api-status">
              <span className={`status-dot${apiError ? " status-dot--error" : ""}`} />
              {apiError ? "API unavailable" : loading ? "Connecting to API" : "API connected"}
            </span>
            <button
              className="icon-button"
              type="button"
              onClick={() => void loadData()}
              aria-label="Refresh dashboard data"
              title="Refresh"
            >
              <RefreshCw size={17} aria-hidden="true" />
            </button>
          </div>
        </header>

        <div className="page-content">
          <section className="page-heading">
            <div>
              <div className="eyebrow">{new Intl.DateTimeFormat(undefined, { weekday: "long", month: "long", day: "numeric" }).format(new Date()).toUpperCase()} · OPERATIONS CENTER</div>
              <h1>{section === "overview" ? "Fleet overview" : liveTitle}</h1>
              <p>
                {section === "overview"
                  ? "Monitor vehicle movement and geofence activity in real time."
                  : sectionDescriptions[section]}
              </p>
            </div>
            {section === "overview" && (
              <button className="button button--primary" type="button" onClick={() => setSection("fleet")}>
                <Plus size={17} aria-hidden="true" />
                Update location
              </button>
            )}
          </section>

          {apiError && (
            <div className="error-banner" role="alert">
              <span>{apiError}</span>
              <button className="button button--small button--quiet" type="button" onClick={() => void loadData()}>
                Try again
              </button>
            </div>
          )}
          {(actionError || actionMessage) && (
            <div className={actionError ? "form-alert form-alert--error" : "form-alert form-alert--success"} role={actionError ? "alert" : "status"}>
              {actionError || actionMessage}
            </div>
          )}

          {loading && !geofences.length && !vehicles.length ? (
            <div className="loading-state" role="status">
              <span className="spinner" />
              Loading operations data…
            </div>
          ) : (
            <>
              {section === "overview" && (
                <Overview
                  geofences={geofences}
                  vehicles={vehicles}
                  alerts={alerts}
                  history={history.violations}
                  snapshots={snapshots}
                  events={events}
                  locations={locations}
                  onNavigate={setSection}
                />
              )}
              {section === "geofences" && (
                <GeofencesPage
                  geofences={geofences}
                  vehicles={vehicles}
                  locations={locations}
                  saving={saving === "geofence"}
                  onCreate={(payload) =>
                    runAction(
                      "geofence",
                      () => api.createGeofence(payload),
                      (result) => `“${result.name}” is active and ready for monitoring.`,
                    )
                  }
                />
              )}
              {section === "fleet" && (
                <FleetPage
                  geofences={geofences}
                  vehicles={vehicles}
                  snapshots={snapshots}
                  locations={locations}
                  saving={saving}
                  onRegister={(payload) =>
                    runAction(
                      "vehicle",
                      () => api.createVehicle(payload),
                      (result) => `${result.vehicle_number} has been registered.`,
                    )
                  }
                  onLocation={(payload) =>
                    runAction(
                      "location",
                      () => api.updateLocation(payload),
                      (result) =>
                        result.current_geofences.length
                          ? `Location saved inside ${result.current_geofences.map((fence) => fence.geofence_name).join(", ")}.`
                          : "Location saved outside all monitored geofences.",
                    )
                  }
                />
              )}
              {section === "rules" && (
                <RulesPage
                  geofences={geofences}
                  vehicles={vehicles}
                  alerts={alerts}
                  saving={saving === "rule"}
                  onCreate={(payload) =>
                    runAction(
                      "rule",
                      () => api.configureAlert(payload),
                      () => "Alert rule is active.",
                    )
                  }
                />
              )}
              {section === "history" && (
                <HistoryPage
                  geofences={geofences}
                  vehicles={vehicles}
                  history={history}
                  loading={historyLoading}
                  onSearch={async (filters) => {
                    setHistoryLoading(true);
                    setActionError("");
                    try {
                      setHistory(await api.history(filters));
                    } catch (error) {
                      setActionError(messageFrom(error));
                    } finally {
                      setHistoryLoading(false);
                    }
                  }}
                />
              )}
            </>
          )}
        </div>
      </main>
    </div>
  );
}

const sectionDescriptions: Record<Section, string> = {
  overview: "Monitor vehicle movement and geofence activity in real time.",
  geofences: "Create boundaries and keep every monitored zone in view.",
  fleet: "Register vehicles and publish their latest coordinates.",
  rules: "Choose which vehicle and zone transitions should raise an alert.",
  history: "Filter recorded entry and exit events across your fleet.",
};

function Panel({
  title,
  subtitle,
  action,
  children,
  className = "",
}: {
  title: string;
  subtitle?: string;
  action?: ReactNode;
  children: ReactNode;
  className?: string;
}) {
  return (
    <section className={`panel ${className}`}>
      <div className="panel-heading">
        <div>
          <h2>{title}</h2>
          {subtitle && <p>{subtitle}</p>}
        </div>
        {action}
      </div>
      {children}
    </section>
  );
}

function SectionHeader({
  title,
  subtitle,
  action,
}: {
  title: string;
  subtitle: string;
  action?: ReactNode;
}) {
  return (
    <div className="section-header">
      <div>
        <h2>{title}</h2>
        <p>{subtitle}</p>
      </div>
      {action}
    </div>
  );
}

function Metric({
  label,
  value,
  note,
  icon: Icon,
  tone,
}: {
  label: string;
  value: number | string;
  note: string;
  icon: typeof Activity;
  tone: "green" | "blue" | "amber" | "red";
}) {
  return (
    <article className="metric-card">
      <div className="metric-top">
        <span>{label}</span>
        <span className={`metric-icon metric-icon--${tone}`}>
          <Icon size={17} aria-hidden="true" />
        </span>
      </div>
      <strong className="metric-value">{value}</strong>
      <span className="metric-note">{note}</span>
    </article>
  );
}

function EmptyState({ title, text, action }: { title: string; text: string; action?: ReactNode }) {
  return (
    <div className="empty-state">
      <span className="empty-icon">
        <Activity size={19} aria-hidden="true" />
      </span>
      <strong>{title}</strong>
      <p>{text}</p>
      {action}
    </div>
  );
}

function CategoryBadge({ category }: { category: GeofenceCategory }) {
  return <span className={`category-badge category-badge--${category}`}>{categoryName(category)}</span>;
}

function EventRow({ event, compact = false }: { event: LiveEvent | Violation; compact?: boolean }) {
  const isLive = "event_id" in event;
  const eventType = event.event_type;
  const vehicleNumber = isLive ? event.vehicle.vehicle_number : event.vehicle_number;
  const geofenceName = isLive ? event.geofence.geofence_name : event.geofence_name;
  const timestamp = event.timestamp;
  return (
    <article className={`event-row${compact ? " event-row--compact" : ""}`}>
      <span className={`event-icon event-icon--${eventType}`}>
        {eventType === "entry" ? (
          <ArrowDownRight size={16} aria-hidden="true" />
        ) : (
          <ArrowUpRight size={16} aria-hidden="true" />
        )}
      </span>
      <div className="event-copy">
        <strong>{vehicleNumber}</strong>
        <span>
          {eventType === "entry" ? "Entered" : "Exited"} <b>{geofenceName}</b>
        </span>
      </div>
      <time dateTime={timestamp}>{formatDate(timestamp)}</time>
    </article>
  );
}

function Overview({
  geofences,
  vehicles,
  alerts,
  history,
  snapshots,
  events,
  locations,
  onNavigate,
}: {
  geofences: Geofence[];
  vehicles: Vehicle[];
  alerts: AlertRule[];
  history: Violation[];
  snapshots: LocationSnapshot[];
  events: LiveEvent[];
  locations: Record<string, { latitude: number; longitude: number }>;
  onNavigate: (section: Section) => void;
}) {
  const activeLocations = Object.keys(locations).length;
  const activity = events.length ? events : history.slice(0, 8);
  return (
    <>
      <div className="metrics-grid">
        <Metric label="Monitored zones" value={geofences.length} note="Across all categories" icon={MapPinned} tone="blue" />
        <Metric label="Registered fleet" value={vehicles.length} note={`${activeLocations} with a reported location`} icon={CarFront} tone="green" />
        <Metric label="Active rules" value={alerts.length} note="Listening for transitions" icon={ShieldCheck} tone="amber" />
        <Metric label="Recent transitions" value={history.length} note="Events on this page" icon={Activity} tone="red" />
      </div>
      <div className="overview-grid">
        <Panel
          title="Live operations map"
          subtitle="Geofence boundaries and last reported vehicle positions"
          action={
            <button className="text-button" type="button" onClick={() => onNavigate("geofences")}>
              Manage zones <ChevronRight size={15} aria-hidden="true" />
            </button>
          }
          className="map-panel-card"
        >
          <MapPanel geofences={geofences} vehicles={vehicles} locations={locations} />
          <div className="map-legend" aria-label="Map legend">
            <span><i className="legend-dot legend-dot--green" />Delivery</span>
            <span><i className="legend-dot legend-dot--red" />Restricted</span>
            <span><i className="legend-dot legend-dot--amber" />Toll</span>
            <span><i className="legend-vehicle" />Vehicle</span>
          </div>
        </Panel>
        <Panel
          title="Recent activity"
          subtitle="Live transitions and latest recorded events"
          action={
            <button className="text-button" type="button" onClick={() => onNavigate("history")}>
              View history <ChevronRight size={15} aria-hidden="true" />
            </button>
          }
        >
          {activity.length ? (
            <div className="event-list">
              {activity.slice(0, 7).map((event) => (
                <EventRow key={"event_id" in event ? event.event_id : event.id} event={event} compact />
              ))}
            </div>
          ) : (
            <EmptyState
              title="No activity yet"
              text="A vehicle entry or exit will appear here as soon as it happens."
              action={
                <button className="button button--secondary button--small" type="button" onClick={() => onNavigate("fleet")}>
                  <Plus size={15} aria-hidden="true" /> Add a location
                </button>
              }
            />
          )}
        </Panel>
      </div>
      <Panel
        title="Fleet snapshot"
        subtitle="Vehicles with their current zone status"
        action={
          <button className="text-button" type="button" onClick={() => onNavigate("fleet")}>
            View fleet <ChevronRight size={15} aria-hidden="true" />
          </button>
        }
      >
        {vehicles.length ? (
          <FleetTable vehicles={vehicles.slice(0, 5)} snapshots={snapshots} geofences={geofences} />
        ) : (
          <EmptyState title="No vehicles registered" text="Add a vehicle to begin tracking fleet movement." />
        )}
      </Panel>
    </>
  );
}

function GeofencesPage({
  geofences,
  vehicles,
  locations,
  saving,
  onCreate,
}: {
  geofences: Geofence[];
  vehicles: Vehicle[];
  locations: Record<string, { latitude: number; longitude: number }>;
  saving: boolean;
  onCreate: (payload: {
    name: string;
    description: string;
    coordinates: [number, number][];
    category: GeofenceCategory;
  }) => Promise<void>;
}) {
  const [category, setCategory] = useState("all");
  const [name, setName] = useState("");
  const [description, setDescription] = useState("");
  const [selectedCategory, setSelectedCategory] = useState<GeofenceCategory>("delivery_zone");
  const [coordinatesText, setCoordinatesText] = useState("[]");
  const [draftPoints, setDraftPoints] = useState<[number, number][]>([]);
  const [formError, setFormError] = useState("");
  const filtered = category === "all" ? geofences : geofences.filter((fence) => fence.category === category);

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");
    try {
        await onCreate({
        name: name.trim(),
        description: description.trim(),
        category: selectedCategory,
        coordinates: parseCoordinates(coordinatesText),
      });
      setName("");
      setDescription("");
      setCoordinatesText("[]");
      setDraftPoints([]);
    } catch (error) {
      setFormError(messageFrom(error));
    }
  };

  const addPoint = (point: [number, number]) => {
    const next = [...draftPoints, point];
    setDraftPoints(next);
    setCoordinatesText(JSON.stringify(next, null, 2));
  };

  return (
    <>
      <SectionHeader title="Geofence management" subtitle={`${geofences.length} zones configured`} />
      <div className="management-grid">
        <Panel title="Create a geofence" subtitle="Click the map to add boundary points or enter coordinates.">
          <form className="form-grid" onSubmit={(event) => void submit(event)}>
            <label className="field field--wide">
              <span>Zone name</span>
              <input value={name} onChange={(event) => setName(event.target.value)} required maxLength={120} placeholder="e.g. Downtown delivery zone" />
            </label>
            <label className="field field--wide">
              <span>Description <small>Optional</small></span>
              <input value={description} onChange={(event) => setDescription(event.target.value)} maxLength={2000} placeholder="What is this area used for?" />
            </label>
            <label className="field field--wide">
              <span>Category</span>
              <select value={selectedCategory} onChange={(event) => setSelectedCategory(event.target.value as GeofenceCategory)}>
                {Object.entries(categoryLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
              </select>
            </label>
            <label className="field field--wide">
              <span>Coordinates <small>[latitude, longitude]</small></span>
              <textarea value={coordinatesText} onChange={(event) => setCoordinatesText(event.target.value)} required rows={5} spellCheck={false} aria-describedby="coordinates-help" />
              <small id="coordinates-help" className="field-hint">Add at least three boundary points. The closing point is added when you save.</small>
            </label>
            {formError && <p className="inline-error" role="alert">{formError}</p>}
            <button className="button button--primary field--wide" disabled={saving} type="submit">
              {saving ? <span className="spinner spinner--small" /> : <Plus size={16} aria-hidden="true" />}
              {saving ? "Creating zone…" : "Create geofence"}
            </button>
          </form>
        </Panel>
        <Panel title="Draw on map" subtitle="Select points in order to sketch a polygon.">
          <MapPanel
            geofences={geofences}
            vehicles={vehicles}
            locations={locations}
            onPoint={addPoint}
            draftPoints={draftPoints}
            selectionHint="Click to add boundary points"
          />
          <div className="map-tools">
            <span>{draftPoints.length} boundary point{draftPoints.length === 1 ? "" : "s"} selected</span>
            <button
              className="text-button"
              type="button"
              disabled={!draftPoints.length}
              onClick={() => {
                const next = draftPoints.slice(0, -1);
                setDraftPoints(next);
                setCoordinatesText(JSON.stringify(next, null, 2));
              }}
            >
              Undo last point
            </button>
            <button
              className="text-button text-button--muted"
              type="button"
              disabled={!draftPoints.length}
              onClick={() => {
                setDraftPoints([]);
                setCoordinatesText("[]");
              }}
            >
              Clear
            </button>
          </div>
        </Panel>
      </div>
      <Panel
        title="Configured zones"
        subtitle="Filter by zone type and review the current boundaries."
        action={
          <label className="compact-filter">
            <span>Category</span>
            <select aria-label="Filter geofences by category" value={category} onChange={(event) => setCategory(event.target.value)}>
              <option value="all">All categories</option>
              {Object.entries(categoryLabels).map(([value, label]) => <option key={value} value={value}>{label}</option>)}
            </select>
          </label>
        }
      >
        {filtered.length ? (
          <div className="table-scroll">
            <table>
              <thead><tr><th>Zone</th><th>Category</th><th>Boundary points</th><th>Created</th></tr></thead>
              <tbody>
                {filtered.map((fence) => (
                  <tr key={fence.id}>
                    <td><strong>{fence.name}</strong><small>{fence.description || "No description"}</small></td>
                    <td><CategoryBadge category={fence.category} /></td>
                    <td>{fence.coordinates.length} points</td>
                    <td>{formatDate(fence.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title={category === "all" ? "No geofences yet" : "No zones in this category"} text="Create a zone above to start monitoring vehicle movement." />
        )}
      </Panel>
    </>
  );
}

function FleetPage({
  geofences,
  vehicles,
  snapshots,
  locations,
  saving,
  onRegister,
  onLocation,
}: {
  geofences: Geofence[];
  vehicles: Vehicle[];
  snapshots: LocationSnapshot[];
  locations: Record<string, { latitude: number; longitude: number }>;
  saving: string;
  onRegister: (payload: {
    vehicle_number: string;
    driver_name: string;
    vehicle_type: string;
    phone: string;
  }) => Promise<void>;
  onLocation: (payload: {
    vehicle_id: string;
    latitude: number;
    longitude: number;
    timestamp: string;
  }) => Promise<void>;
}) {
  const [vehicleNumber, setVehicleNumber] = useState("");
  const [driverName, setDriverName] = useState("");
  const [vehicleType, setVehicleType] = useState("van");
  const [phone, setPhone] = useState("");
  const [selectedVehicle, setSelectedVehicle] = useState("");
  const [latitude, setLatitude] = useState("");
  const [longitude, setLongitude] = useState("");
  const [selectingPoint, setSelectingPoint] = useState(false);
  const [formError, setFormError] = useState("");

  const register = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");
    try {
      await onRegister({
        vehicle_number: vehicleNumber.trim(),
        driver_name: driverName.trim(),
        vehicle_type: vehicleType,
        phone: phone.trim(),
      });
    } catch (error) {
      setFormError(messageFrom(error));
      return;
    }
    setVehicleNumber("");
    setDriverName("");
    setPhone("");
  };
  const update = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");
    if (!selectedVehicle) {
      setFormError("Select a vehicle before updating its location.");
      return;
    }
    const lat = Number(latitude);
    const lon = Number(longitude);
    if (!Number.isFinite(lat) || lat < -90 || lat > 90 || !Number.isFinite(lon) || lon < -180 || lon > 180) {
      setFormError("Enter a latitude from −90 to 90 and longitude from −180 to 180.");
      return;
    }
    try {
      await onLocation({
        vehicle_id: selectedVehicle,
        latitude: lat,
        longitude: lon,
        timestamp: new Date().toISOString(),
      });
    } catch (error) {
      setFormError(messageFrom(error));
    }
  };

  return (
    <>
      <SectionHeader title="Fleet management" subtitle={`${vehicles.length} registered vehicles`} />
      <div className="management-grid">
        <Panel title="Register a vehicle" subtitle="Add driver and vehicle details to your fleet.">
          <form className="form-grid" onSubmit={(event) => void register(event)}>
            <label className="field field--wide"><span>Vehicle number</span><input value={vehicleNumber} onChange={(event) => setVehicleNumber(event.target.value)} required maxLength={40} placeholder="KA-01-AB-1234" /></label>
            <label className="field field--wide"><span>Driver name</span><input value={driverName} onChange={(event) => setDriverName(event.target.value)} required maxLength={120} placeholder="Full name" /></label>
            <label className="field"><span>Vehicle type</span><select value={vehicleType} onChange={(event) => setVehicleType(event.target.value)}><option value="truck">Truck</option><option value="car">Car</option><option value="van">Van</option><option value="motorcycle">Motorcycle</option><option value="other">Other</option></select></label>
            <label className="field"><span>Driver phone</span><input type="tel" value={phone} onChange={(event) => setPhone(event.target.value)} required maxLength={40} placeholder="+1 555 0100" /></label>
            {formError && <p className="inline-error" role="alert">{formError}</p>}
            <button className="button button--primary field--wide" disabled={saving === "vehicle"} type="submit">
              {saving === "vehicle" ? <span className="spinner spinner--small" /> : <Plus size={16} aria-hidden="true" />}
              {saving === "vehicle" ? "Registering…" : "Register vehicle"}
            </button>
          </form>
        </Panel>
        <Panel title="Update vehicle location" subtitle="Enter coordinates or pick a point from the map.">
          <form className="form-grid" onSubmit={(event) => void update(event)}>
            <label className="field field--wide"><span>Vehicle</span><select value={selectedVehicle} onChange={(event) => setSelectedVehicle(event.target.value)} required><option value="">Select a vehicle</option>{vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number} · {vehicle.driver_name}</option>)}</select></label>
            <label className="field"><span>Latitude</span><input type="number" step="any" min="-90" max="90" value={latitude} onChange={(event) => setLatitude(event.target.value)} required placeholder="37.7749" /></label>
            <label className="field"><span>Longitude</span><input type="number" step="any" min="-180" max="180" value={longitude} onChange={(event) => setLongitude(event.target.value)} required placeholder="-122.4194" /></label>
            <button className="button button--secondary field--wide" type="button" onClick={() => setSelectingPoint((current) => !current)} disabled={!vehicles.length}>
              <MapPinned size={16} aria-hidden="true" />
              {selectingPoint ? "Stop selecting on map" : "Pick coordinates on map"}
            </button>
            {selectingPoint && (
              <div className="field--wide">
                <MapPanel
                  geofences={geofences}
                  vehicles={vehicles}
                  locations={locations}
                  onPoint={(point) => {
                    setLatitude(point[0].toFixed(6));
                    setLongitude(point[1].toFixed(6));
                  }}
                  selectionHint="Click the map to choose a location"
                />
              </div>
            )}
            {formError && <p className="inline-error field--wide" role="alert">{formError}</p>}
            <button className="button button--primary field--wide" disabled={saving === "location" || !vehicles.length} type="submit">
              {saving === "location" ? <span className="spinner spinner--small" /> : <MapPinned size={16} aria-hidden="true" />}
              {saving === "location" ? "Saving location…" : "Save location"}
            </button>
          </form>
        </Panel>
      </div>
      <Panel title="Vehicles" subtitle="Latest known position and geofence state.">
        {vehicles.length ? (
          <FleetTable vehicles={vehicles} snapshots={snapshots} geofences={geofences} />
        ) : (
          <EmptyState title="Your fleet is empty" text="Register the first vehicle to start receiving location updates." />
        )}
      </Panel>
    </>
  );
}

function FleetTable({
  vehicles,
  snapshots,
  geofences,
}: {
  vehicles: Vehicle[];
  snapshots: LocationSnapshot[];
  geofences: Geofence[];
}) {
  const snapshotMap = new Map(snapshots.map((snapshot) => [snapshot.vehicle_id, snapshot]));
  return (
    <div className="table-scroll">
      <table>
        <thead><tr><th>Vehicle</th><th>Driver</th><th>Current zone</th><th>Last reported</th><th>Status</th></tr></thead>
        <tbody>
          {vehicles.map((vehicle) => {
            const snapshot = snapshotMap.get(vehicle.id);
            const currentGeofenceIDs = new Set(snapshot?.current_geofences.map((fence) => fence.geofence_id) ?? []);
            const zoneNames = geofences.filter((fence) => currentGeofenceIDs.has(fence.id)).map((fence) => fence.name);
            return (
              <tr key={vehicle.id}>
                <td><strong>{vehicle.vehicle_number}</strong><small>{vehicle.vehicle_type} · {vehicle.phone}</small></td>
                <td>{vehicle.driver_name}</td>
                <td>{zoneNames.length ? zoneNames.join(", ") : <span className="muted-copy">Outside monitored zones</span>}</td>
                <td>{snapshot?.current_location ? formatDate(snapshot.current_location.timestamp) : <span className="muted-copy">No location yet</span>}</td>
                <td><span className="status-pill"><span className="status-dot" />{vehicle.status}</span></td>
              </tr>
            );
          })}
        </tbody>
      </table>
    </div>
  );
}

function RulesPage({
  geofences,
  vehicles,
  alerts,
  saving,
  onCreate,
}: {
  geofences: Geofence[];
  vehicles: Vehicle[];
  alerts: AlertRule[];
  saving: boolean;
  onCreate: (payload: {
    geofence_id: string;
    vehicle_id?: string;
    event_type: EventType;
  }) => Promise<void>;
}) {
  const [geofenceID, setGeofenceID] = useState("");
  const [vehicleID, setVehicleID] = useState("");
  const [eventType, setEventType] = useState<EventType>("both");
  const [formError, setFormError] = useState("");

  const submit = async (event: FormEvent<HTMLFormElement>) => {
    event.preventDefault();
    setFormError("");
    if (!geofenceID) {
      setFormError("Choose a geofence before creating an alert rule.");
      return;
    }
    try {
      await onCreate({ geofence_id: geofenceID, ...(vehicleID ? { vehicle_id: vehicleID } : {}), event_type: eventType });
    } catch (error) {
      setFormError(messageFrom(error));
    }
  };

  return (
    <>
      <SectionHeader title="Alert rules" subtitle={`${alerts.length} active monitoring rules`} />
      <div className="management-grid management-grid--narrow">
        <Panel title="Configure a rule" subtitle="Rules match a specific vehicle or the entire fleet.">
          <form className="form-grid" onSubmit={(event) => void submit(event)}>
            <label className="field field--wide"><span>Geofence</span><select value={geofenceID} onChange={(event) => setGeofenceID(event.target.value)} required><option value="">Choose a zone</option>{geofences.map((fence) => <option key={fence.id} value={fence.id}>{fence.name}</option>)}</select></label>
            <label className="field field--wide"><span>Vehicle <small>Optional · all vehicles</small></span><select value={vehicleID} onChange={(event) => setVehicleID(event.target.value)}><option value="">All vehicles</option>{vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}</select></label>
            <label className="field field--wide"><span>Event</span><select value={eventType} onChange={(event) => setEventType(event.target.value as EventType)}><option value="both">Entry and exit</option><option value="entry">Entry only</option><option value="exit">Exit only</option></select></label>
            {formError && <p className="inline-error" role="alert">{formError}</p>}
            <button className="button button--primary field--wide" type="submit" disabled={saving || !geofences.length}>
              {saving ? <span className="spinner spinner--small" /> : <Plus size={16} aria-hidden="true" />}
              {saving ? "Saving rule…" : "Create alert rule"}
            </button>
            {!geofences.length && <p className="field-hint field--wide">Create a geofence before you configure an alert rule.</p>}
          </form>
        </Panel>
        <div className="rule-note">
          <span className="rule-note-icon"><ShieldCheck size={18} aria-hidden="true" /></span>
          <strong>Transitions, not repeated pings</strong>
          <p>Alerts fire only when a vehicle changes from outside to inside a zone, or inside to outside. Staying in the same state does not create duplicates.</p>
          <div className="rule-note-stat"><span>Configured</span><strong>{alerts.length} rules</strong></div>
        </div>
      </div>
      <Panel title="Configured rules" subtitle="Use the filters to review active zone and vehicle rules.">
        {alerts.length ? (
          <div className="table-scroll">
            <table>
              <thead><tr><th>Geofence</th><th>Vehicle scope</th><th>Trigger</th><th>Status</th><th>Created</th></tr></thead>
              <tbody>
                {alerts.map((alert) => (
                  <tr key={alert.alert_id}>
                    <td><strong>{alert.geofence_name}</strong><small>{alert.geofence_id}</small></td>
                    <td>{alert.vehicle_number ?? "All vehicles"}</td>
                    <td><span className="trigger-label">{alert.event_type === "both" ? "Entry + exit" : alert.event_type}</span></td>
                    <td><span className="status-pill"><span className="status-dot" />{alert.status}</span></td>
                    <td>{formatDate(alert.created_at)}</td>
                  </tr>
                ))}
              </tbody>
            </table>
          </div>
        ) : (
          <EmptyState title="No alert rules configured" text="Create a rule to receive live alerts for selected vehicle and zone transitions." />
        )}
      </Panel>
    </>
  );
}

function HistoryPage({
  geofences,
  vehicles,
  history,
  loading,
  onSearch,
}: {
  geofences: Geofence[];
  vehicles: Vehicle[];
  history: { violations: Violation[]; total_count: number };
  loading: boolean;
  onSearch: (filters: {
    vehicle_id?: string;
    geofence_id?: string;
    start_date?: string;
    end_date?: string;
    limit?: number;
    offset?: number;
  }) => Promise<void>;
}) {
  const [vehicleID, setVehicleID] = useState("");
  const [geofenceID, setGeofenceID] = useState("");
  const [startDate, setStartDate] = useState("");
  const [endDate, setEndDate] = useState("");
  const [offset, setOffset] = useState(0);
  const pageSize = 25;

  const search = (nextOffset = 0) => {
    setOffset(nextOffset);
    void onSearch({
      ...(vehicleID ? { vehicle_id: vehicleID } : {}),
      ...(geofenceID ? { geofence_id: geofenceID } : {}),
      ...(startDate ? { start_date: new Date(startDate).toISOString() } : {}),
      ...(endDate ? { end_date: new Date(endDate).toISOString() } : {}),
      limit: pageSize,
      offset: nextOffset,
    });
  };

  return (
    <>
      <SectionHeader title="Violation history" subtitle={`${history.total_count} recorded transitions`} />
      <Panel title="Filter events" subtitle="Narrow the history by vehicle, geofence, and date.">
        <form className="filter-grid" onSubmit={(event) => { event.preventDefault(); search(0); }}>
          <label className="field"><span>Vehicle</span><select value={vehicleID} onChange={(event) => setVehicleID(event.target.value)}><option value="">All vehicles</option>{vehicles.map((vehicle) => <option key={vehicle.id} value={vehicle.id}>{vehicle.vehicle_number}</option>)}</select></label>
          <label className="field"><span>Geofence</span><select value={geofenceID} onChange={(event) => setGeofenceID(event.target.value)}><option value="">All geofences</option>{geofences.map((fence) => <option key={fence.id} value={fence.id}>{fence.name}</option>)}</select></label>
          <label className="field"><span>From</span><input type="datetime-local" value={startDate} onChange={(event) => setStartDate(event.target.value)} /></label>
          <label className="field"><span>To</span><input type="datetime-local" value={endDate} onChange={(event) => setEndDate(event.target.value)} /></label>
          <button className="button button--primary filter-submit" type="submit" disabled={loading}>
            {loading ? <span className="spinner spinner--small" /> : <Clock3 size={16} aria-hidden="true" />}
            Apply filters
          </button>
        </form>
      </Panel>
      <Panel title="Recorded events" subtitle="Entry and exit transitions are stored for audit and review.">
        {loading ? (
          <div className="loading-state loading-state--compact" role="status"><span className="spinner" />Loading history…</div>
        ) : history.violations.length ? (
          <>
            <div className="table-scroll">
              <table>
                <thead><tr><th>Transition</th><th>Vehicle</th><th>Geofence</th><th>Location</th><th>Timestamp</th></tr></thead>
                <tbody>
                  {history.violations.map((violation) => (
                    <tr key={violation.id}>
                      <td><span className={`transition-pill transition-pill--${violation.event_type}`}>{violation.event_type === "entry" ? <ArrowDownRight size={14} aria-hidden="true" /> : <ArrowUpRight size={14} aria-hidden="true" />}{violation.event_type}</span></td>
                      <td><strong>{violation.vehicle_number}</strong><small>{violation.vehicle_id}</small></td>
                      <td>{violation.geofence_name}</td>
                      <td className="coordinate-cell">{violation.latitude.toFixed(5)}, {violation.longitude.toFixed(5)}</td>
                      <td>{formatDate(violation.timestamp)}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </div>
            <div className="pagination">
              <span>Showing {offset + 1}–{Math.min(offset + history.violations.length, history.total_count)} of {history.total_count}</span>
              <div>
                <button type="button" className="icon-button" disabled={offset === 0 || loading} onClick={() => search(Math.max(0, offset - pageSize))} aria-label="Previous history page"><ChevronLeft size={17} aria-hidden="true" /></button>
                <button type="button" className="icon-button" disabled={offset + pageSize >= history.total_count || loading} onClick={() => search(offset + pageSize)} aria-label="Next history page"><ChevronRight size={17} aria-hidden="true" /></button>
              </div>
            </div>
          </>
        ) : (
          <EmptyState title="No transitions found" text="Try changing the filters or wait for a vehicle to cross a geofence boundary." />
        )}
      </Panel>
    </>
  );
}

export default App;

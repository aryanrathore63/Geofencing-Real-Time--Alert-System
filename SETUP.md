# Setup and operations

## Prerequisites

- Go 1.23 or later
- Node.js 20 or later and npm
- Docker Desktop with the Compose plugin for the container workflow

The map uses OpenStreetMap tiles and does not require a paid API key.

## Configuration

Copy `.env.example` to `.env` when using Compose. It contains placeholders and no credentials.

| Variable | Default | Description |
| --- | --- | --- |
| `API_PORT` | `8080` | Host port for the Go API in Compose |
| `FRONTEND_PORT` | `3000` | Host port for the frontend in Compose |
| `ALLOWED_ORIGINS` | localhost ports | Comma-separated browser origins permitted by API CORS and WebSocket |
| `PORT` | `8080` | API container/listener port |
| `DATABASE_PATH` | `./data/geofencing.db` locally | SQLite database path; use a persistent volume in deployment |
| `VITE_API_BASE_URL` | empty | Frontend API base URL; leave empty for the local Vite proxy or Compose Nginx proxy |

For a separately deployed frontend, set `VITE_API_BASE_URL` to the backend's public HTTPS base URL at build time. Add the exact frontend origin to the backend's `ALLOWED_ORIGINS`. WebSocket connections use `wss://` when that URL is HTTPS.

## Run locally

Start the backend from PowerShell:

```powershell
cd backend
$env:PORT = "8080"
$env:DATABASE_PATH = ".\data\geofencing.db"
$env:ALLOWED_ORIGINS = "http://localhost:5173,http://127.0.0.1:5173"
go run .\cmd\server
```

Start the React development server in a second terminal:

```powershell
cd frontend
npm ci
npm run dev
```

Open <http://localhost:5173>. Vite proxies the required API and `/ws/alerts` to `localhost:8080`.

## Docker Compose

From the project root:

```powershell
Copy-Item .env.example .env
docker compose up --build
```

- Dashboard: <http://localhost:3000>
- API health: <http://localhost:8080/healthz>
- Stop the services: `docker compose down`
- Stop and remove the database volume: `docker compose down --volumes` (this permanently deletes local application data)

The backend stores SQLite in the named `geofencing-data` volume mounted at `/app/data`; SQLite is the database service, so there is no separate database container. Keep that volume in place between runs to preserve vehicles, zones, locations, rules, and violations.

## API examples

Every JSON response includes `time_ns` as a string containing request-handler execution time in nanoseconds. Errors use `{"error":"…","time_ns":"…"}`.

Create a closed polygon (coordinates are `[latitude, longitude]`):

```powershell
curl.exe -X POST http://localhost:8080/geofences `
  -H "Content-Type: application/json" `
  -d '{"name":"Downtown Delivery Zone","description":"Delivery coverage","coordinates":[[37.7749,-122.4194],[37.7849,-122.4194],[37.7849,-122.4094],[37.7749,-122.4094],[37.7749,-122.4194]],"category":"delivery_zone"}'
```

List and filter geofences:

```powershell
curl.exe "http://localhost:8080/geofences?category=delivery_zone"
```

Register a vehicle:

```powershell
curl.exe -X POST http://localhost:8080/vehicles `
  -H "Content-Type: application/json" `
  -d '{"vehicle_number":"KA-01-AB-1234","driver_name":"John Doe","vehicle_type":"truck","phone":"+1234567890"}'
```

Update a vehicle location (use the returned `geo_…` and `veh_…` IDs):

```powershell
curl.exe -X POST http://localhost:8080/vehicles/location `
  -H "Content-Type: application/json" `
  -d '{"vehicle_id":"veh_…","latitude":37.78,"longitude":-122.414,"timestamp":"2025-01-15T10:35:00Z"}'
```

Get the latest location, configure an entry/exit/both alert, and list rules:

```powershell
curl.exe "http://localhost:8080/vehicles/location/veh_…"
curl.exe -X POST http://localhost:8080/alerts/configure `
  -H "Content-Type: application/json" `
  -d '{"geofence_id":"geo_…","vehicle_id":"veh_…","event_type":"both"}'
curl.exe "http://localhost:8080/alerts?geofence_id=geo_…&vehicle_id=veh_…"
```

Filter violation history by vehicle, geofence, time range, and page:

```powershell
curl.exe "http://localhost:8080/violations/history?vehicle_id=veh_…&geofence_id=geo_…&start_date=2025-01-01T00:00:00Z&end_date=2025-02-01T00:00:00Z&limit=25&offset=0"
```

The live alert WebSocket is `ws://localhost:8080/ws/alerts` (use `wss://` behind HTTPS). Alerts are broadcast only when a configured rule matches a geofence transition; each transition is recorded in violation history, and unchanged inside/outside states do not create duplicates.

## Frontend usage

- **Overview:** map, fleet/zone/rule metrics, and recent transitions.
- **Geofences:** create a named category zone by clicking map points in order or entering coordinate pairs; filter the zone list by category.
- **Fleet:** register vehicles, select one, and update its position by coordinates or map click. Current zone status appears in the fleet table.
- **Alert rules:** configure entry, exit, or both for a specific vehicle or the whole fleet.
- **History:** filter by vehicle, geofence, and date range; navigate the response using limit/offset pagination.
- **Live alerts:** the dashboard reconnects to `/ws/alerts` and displays incoming transitions without a page refresh.

## Tests and builds

```powershell
cd backend
go test ./...
go test -race ./...
```

```powershell
cd frontend
npm ci
npm test
npm run build
```

The backend tests cover API responses and validation, coordinate range/closure checks, polygon inclusion, alert-rule matching, transition persistence, duplicate suppression, and WebSocket delivery. Frontend tests exercise geofence creation and vehicle registration. Docker Compose can be smoke-checked with `docker compose config` and `docker compose up --build`.

## Deployment notes

Deployment is not performed by this project setup. Do not publish an image or deploy until you have selected accounts and approved the release.

### Build and push the backend image

From the project root, replace the placeholders with your Docker Hub namespace and release tag:

```powershell
docker login
docker buildx build --platform linux/amd64 -t <dockerhub-user>/geofencing-api:<tag> --push .\backend
```

### Deploy the backend

Choose a container host, deploy `docker.io/<dockerhub-user>/geofencing-api:<tag>`, expose container port `8080`, and configure:

- `PORT=8080`
- `DATABASE_PATH=/app/data/geofencing.db`
- `ALLOWED_ORIGINS=https://<your-frontend-domain>`
- A persistent disk mounted at `/app/data`

Use the platform-generated HTTPS URL as the API base URL and health-check `/healthz`. SQLite is appropriate for a single API instance with persistent local storage; use PostgreSQL/PostGIS and a coordinated event bus before scaling to multiple instances. Add authentication, rate limiting, TLS enforcement, backups, and monitoring before production use.

### Deploy the frontend to Vercel

1. Push the reviewed source to a GitHub repository you control.
2. Import it in Vercel and set the project root to `frontend`.
3. Use `npm run build` and `dist` as the build command/output directory (Vercel usually detects Vite automatically).
4. Set `VITE_API_BASE_URL` to the deployed backend HTTPS base URL, then deploy.
5. Add the resulting frontend origin to the backend `ALLOWED_ORIGINS`; redeploy the backend if needed.
6. Confirm the dashboard loads, API requests succeed, and the WebSocket connects over `wss://`.

The `frontend/vercel.json` SPA fallback is included. This guide does not claim that a deployment or public URL exists.

## Demo / video checklist (30–45 minutes)

- [ ] **App demo (10–12 min):** create a geofence on the map, register a vehicle, update its position, demonstrate entry and exit, show a live alert, filter history, and run Docker Compose locally.
- [ ] **Implementation walkthrough (12–18 min):** explain project structure, Go API and validation, polygon logic, state transitions, WebSocket client lifecycle, SQLite schema, and React integration.
- [ ] **Gaps and roadmap (10–15 min):** discuss edge cases, scale limits, authentication, rate limiting, production database/queue, deployment and backup strategy.
- [ ] Record the walkthrough using a screen recorder. If you choose to share it, upload an unlisted video yourself and add the real link only after it exists.
- [ ] Complete the assessment form received by email. This repository does not submit the form or invent a video URL.

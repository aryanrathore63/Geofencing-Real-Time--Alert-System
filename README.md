# Geofencing Control

A full-stack fleet operations application for defining geofences, tracking vehicle locations, configuring entry/exit alerts, and reviewing transition history.

## Project

- `backend/` — Go REST API, SQLite persistence, point-in-polygon checks, and WebSocket alert delivery.
- `frontend/` — responsive React dashboard with an OpenStreetMap view.
- `docker-compose.yml` — local API and frontend stack with a persistent SQLite volume.
- `SETUP.md` — setup, API examples, tests, Docker instructions, and deployment notes.

## Quick start

```powershell
docker compose up --build
```

Open the dashboard at <http://localhost:3000>. The API listens at <http://localhost:8080>.

The SQLite database is persisted in the `geofencing-data` Docker volume. See [SETUP.md](./SETUP.md) for local development and deployment guidance.

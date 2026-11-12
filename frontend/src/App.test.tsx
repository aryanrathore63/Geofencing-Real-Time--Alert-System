import { fireEvent, render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { beforeEach, describe, expect, it, vi } from "vitest";
import App from "./App";

vi.mock("./components/MapPanel", () => ({
  MapPanel: () => <div data-testid="map-panel">Map preview</div>,
}));

class MockWebSocket {
  onopen: ((event: Event) => void) | null = null;
  onmessage: ((event: MessageEvent) => void) | null = null;
  onclose: ((event: CloseEvent) => void) | null = null;
  onerror: ((event: Event) => void) | null = null;
  close() {
    this.onclose?.(new CloseEvent("close"));
  }
}

describe("operations dashboard", () => {
  let createdFence = false;
  let createdVehicle = false;

  beforeEach(() => {
    createdFence = false;
    createdVehicle = false;
    vi.stubGlobal("WebSocket", MockWebSocket);
    vi.stubGlobal(
      "fetch",
      vi.fn(async (input: RequestInfo | URL, init?: RequestInit) => {
        const url = String(input);
        const method = init?.method ?? "GET";
        let body: Record<string, unknown> = { time_ns: "1" };
        if (url.endsWith("/geofences") && method === "GET") {
          body.geofences = createdFence
            ? [
                {
                  id: "geo_1",
                  name: "Test boundary",
                  description: "",
                  coordinates: [[37, -123], [38, -123], [38, -122], [37, -123]],
                  category: "delivery_zone",
                  created_at: "2025-01-15T10:30:00Z",
                },
              ]
            : [];
        } else if (url.endsWith("/geofences") && method === "POST") {
          createdFence = true;
          body = { id: "geo_1", name: "Test boundary", status: "active", time_ns: "1" };
        } else if (url.endsWith("/vehicles") && method === "GET") {
          body.vehicles = createdVehicle
            ? [
                {
                  id: "veh_1",
                  vehicle_number: "KA-01-AB-1234",
                  driver_name: "A Driver",
                  vehicle_type: "van",
                  phone: "+1234567890",
                  status: "active",
                  created_at: "2025-01-15T10:30:00Z",
                },
              ]
            : [];
        } else if (url.endsWith("/vehicles") && method === "POST") {
          createdVehicle = true;
          body = { id: "veh_1", vehicle_number: "KA-01-AB-1234", status: "active", time_ns: "1" };
        } else if (url.endsWith("/alerts")) {
          body.alerts = [];
        } else if (url.includes("/violations/history")) {
          body = { violations: [], total_count: 0, time_ns: "1" };
        } else if (url.endsWith("/vehicles/location/veh_1")) {
          body = {
            vehicle_id: "veh_1",
            vehicle_number: "KA-01-AB-1234",
            current_location: null,
            current_geofences: [],
            time_ns: "1",
          };
        }
        return {
          ok: true,
          status: 200,
          json: async () => body,
        } as Response;
      }),
    );
  });

  it("creates a geofence from the geofence form", async () => {
    const user = userEvent.setup();
    render(<App />);
    await user.click(await screen.findByRole("button", { name: "Geofences" }));
    expect(await screen.findByText("No geofences yet")).toBeInTheDocument();

    await user.type(screen.getByLabelText("Zone name"), "Test boundary");
    fireEvent.change(screen.getByLabelText(/Coordinates/), {
      target: { value: "[[37,-123],[38,-123],[38,-122],[37,-123]]" },
    });
    await user.click(screen.getByRole("button", { name: "Create geofence" }));

    expect(await screen.findByRole("status")).toHaveTextContent(
      "“Test boundary” is active and ready for monitoring.",
    );
    expect(await screen.findByText("Test boundary")).toBeInTheDocument();
  });

  it("registers a vehicle and shows the new fleet row", async () => {
    const user = userEvent.setup();
    render(<App />);
    const fleetButtons = await screen.findAllByRole("button", { name: "Fleet" });
    await user.click(fleetButtons[0]);
    await user.type(screen.getByLabelText("Vehicle number"), "KA-01-AB-1234");
    await user.type(screen.getByLabelText("Driver name"), "A Driver");
    await user.type(screen.getByLabelText("Driver phone"), "+1234567890");
    await user.click(screen.getByRole("button", { name: "Register vehicle" }));

    expect(await screen.findByText("KA-01-AB-1234 has been registered.")).toBeInTheDocument();
    await waitFor(() => expect(screen.getAllByText("KA-01-AB-1234").length).toBeGreaterThan(0));
  });
});

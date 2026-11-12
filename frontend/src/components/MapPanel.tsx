import { useMemo } from "react";
import {
  CircleMarker,
  MapContainer,
  Polyline,
  Polygon,
  Popup,
  TileLayer,
  useMapEvents,
} from "react-leaflet";
import type { Geofence, Vehicle } from "../types";

const categoryColors: Record<string, string> = {
  delivery_zone: "#35c78a",
  restricted_zone: "#ff7166",
  toll_zone: "#f4b65f",
  customer_area: "#70a9ff",
};

interface MapPanelProps {
  geofences: Geofence[];
  vehicles: Vehicle[];
  locations: Record<string, { latitude: number; longitude: number }>;
  onPoint?: (point: [number, number]) => void;
  draftPoints?: [number, number][];
  selectionHint?: string;
}

function MapClickHandler({ onPoint }: { onPoint?: MapPanelProps["onPoint"] }) {
  useMapEvents({
    click(event) {
      onPoint?.([event.latlng.lat, event.latlng.lng]);
    },
  });
  return null;
}

export function MapPanel({
  geofences,
  vehicles,
  locations,
  onPoint,
  draftPoints = [],
  selectionHint,
}: MapPanelProps) {
  const center = useMemo<[number, number]>(() => {
    const firstVehicle = Object.values(locations)[0];
    if (firstVehicle) return [firstVehicle.latitude, firstVehicle.longitude];
    const firstFence = geofences[0]?.coordinates[0];
    return firstFence ?? [37.7749, -122.4194];
  }, [geofences, locations]);

  return (
    <div className={`map-wrap${onPoint ? " map-wrap--selectable" : ""}`}>
      {selectionHint && <div className="map-hint">{selectionHint}</div>}
      <MapContainer center={center} zoom={12} scrollWheelZoom className="map-canvas">
        <TileLayer
          attribution='&copy; <a href="https://www.openstreetmap.org/copyright">OpenStreetMap</a> contributors'
          url="https://{s}.tile.openstreetmap.org/{z}/{x}/{y}.png"
        />
        <MapClickHandler onPoint={onPoint} />
        {draftPoints.length > 1 && (
          <Polyline
            positions={draftPoints}
            pathOptions={{ color: "#b69cff", weight: 3, dashArray: "7 7" }}
          />
        )}
        {draftPoints.map((point, index) => (
          <CircleMarker
            key={`draft-${index}`}
            center={point}
            radius={5}
            pathOptions={{ color: "#101b2c", fillColor: "#b69cff", fillOpacity: 1, weight: 2 }}
          />
        ))}
        {geofences.map((fence) => (
          <Polygon
            key={fence.id}
            positions={fence.coordinates}
            pathOptions={{
              color: categoryColors[fence.category] ?? "#70a9ff",
              fillColor: categoryColors[fence.category] ?? "#70a9ff",
              fillOpacity: 0.13,
              weight: 2,
            }}
          >
            <Popup>
              <strong>{fence.name}</strong>
              <br />
              {fence.category.replaceAll("_", " ")}
            </Popup>
          </Polygon>
        ))}
        {vehicles.map((vehicle) => {
          const point = locations[vehicle.id];
          if (!point) return null;
          return (
            <CircleMarker
              key={vehicle.id}
              center={[point.latitude, point.longitude]}
              radius={8}
              pathOptions={{ color: "#101b2c", fillColor: "#50d6a1", fillOpacity: 1, weight: 3 }}
            >
              <Popup>
                <strong>{vehicle.vehicle_number}</strong>
                <br />
                {vehicle.driver_name}
              </Popup>
            </CircleMarker>
          );
        })}
      </MapContainer>
    </div>
  );
}

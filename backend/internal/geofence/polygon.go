package geofence

import (
	"errors"
	"math"
)

var ErrInvalidPolygon = errors.New("coordinates must form a valid closed polygon")

// Validate checks the [latitude, longitude] pairs accepted by the API.
func Validate(points [][]float64) error {
	if len(points) < 4 {
		return ErrInvalidPolygon
	}
	unique := make(map[[2]float64]struct{}, len(points)-1)
	for _, point := range points {
		if len(point) != 2 || math.IsNaN(point[0]) || math.IsInf(point[0], 0) ||
			math.IsNaN(point[1]) || math.IsInf(point[1], 0) ||
			point[0] < -90 || point[0] > 90 || point[1] < -180 || point[1] > 180 {
			return ErrInvalidPolygon
		}
		unique[[2]float64{point[0], point[1]}] = struct{}{}
	}
	if points[0][0] != points[len(points)-1][0] || points[0][1] != points[len(points)-1][1] {
		return ErrInvalidPolygon
	}
	if len(unique) < 3 {
		return ErrInvalidPolygon
	}
	if math.Abs(area(points)) < 1e-12 {
		return ErrInvalidPolygon
	}
	if selfIntersects(points) {
		return ErrInvalidPolygon
	}
	return nil
}

// Contains treats points on a polygon edge as inside.
func Contains(polygon [][]float64, latitude, longitude float64) bool {
	if len(polygon) < 4 {
		return false
	}
	inside := false
	for i, j := 0, len(polygon)-1; i < len(polygon); j, i = i, i+1 {
		lat1, lon1 := polygon[j][0], polygon[j][1]
		lat2, lon2 := polygon[i][0], polygon[i][1]
		if onSegment(longitude, latitude, lon1, lat1, lon2, lat2) {
			return true
		}
		if (lat1 > latitude) != (lat2 > latitude) &&
			longitude < (lon2-lon1)*(latitude-lat1)/(lat2-lat1)+lon1 {
			inside = !inside
		}
	}
	return inside
}

func area(points [][]float64) float64 {
	var sum float64
	for i := 0; i < len(points)-1; i++ {
		sum += points[i][1]*points[i+1][0] - points[i+1][1]*points[i][0]
	}
	return sum / 2
}

func onSegment(x, y, x1, y1, x2, y2 float64) bool {
	cross := (x-x1)*(y2-y1) - (y-y1)*(x2-x1)
	if math.Abs(cross) > 1e-10 {
		return false
	}
	return x >= math.Min(x1, x2)-1e-10 && x <= math.Max(x1, x2)+1e-10 &&
		y >= math.Min(y1, y2)-1e-10 && y <= math.Max(y1, y2)+1e-10
}

func selfIntersects(points [][]float64) bool {
	edges := len(points) - 1
	for i := 0; i < edges; i++ {
		for j := i + 1; j < edges; j++ {
			if j == i+1 || (i == 0 && j == edges-1) {
				continue
			}
			if segmentsIntersect(
				points[i][1], points[i][0], points[i+1][1], points[i+1][0],
				points[j][1], points[j][0], points[j+1][1], points[j+1][0],
			) {
				return true
			}
		}
	}
	return false
}

func segmentsIntersect(ax, ay, bx, by, cx, cy, dx, dy float64) bool {
	o1 := orientation(ax, ay, bx, by, cx, cy)
	o2 := orientation(ax, ay, bx, by, dx, dy)
	o3 := orientation(cx, cy, dx, dy, ax, ay)
	o4 := orientation(cx, cy, dx, dy, bx, by)
	if o1 != o2 && o3 != o4 {
		return true
	}
	return (o1 == 0 && onSegment(cx, cy, ax, ay, bx, by)) ||
		(o2 == 0 && onSegment(dx, dy, ax, ay, bx, by)) ||
		(o3 == 0 && onSegment(ax, ay, cx, cy, dx, dy)) ||
		(o4 == 0 && onSegment(bx, by, cx, cy, dx, dy))
}

func orientation(ax, ay, bx, by, cx, cy float64) int {
	cross := (by-ay)*(cx-bx) - (bx-ax)*(cy-by)
	if math.Abs(cross) <= 1e-10 {
		return 0
	}
	if cross > 0 {
		return 1
	}
	return 2
}

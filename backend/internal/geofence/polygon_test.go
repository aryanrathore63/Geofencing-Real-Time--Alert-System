package geofence

import "testing"

var square = [][]float64{
	{37, -123},
	{38, -123},
	{38, -122},
	{37, -122},
	{37, -123},
}

func TestValidatePolygon(t *testing.T) {
	tests := []struct {
		name   string
		points [][]float64
		valid  bool
	}{
		{"closed square", square, true},
		{"closed triangle", [][]float64{{0, 0}, {1, 0}, {1, 1}, {0, 0}}, true},
		{"open ring", square[:4], false},
		{"too few unique points", [][]float64{{0, 0}, {1, 1}, {0, 0}, {0, 0}}, false},
		{"latitude out of range", [][]float64{{0, 0}, {91, 0}, {1, 1}, {0, 0}}, false},
		{"longitude out of range", [][]float64{{0, 0}, {1, 181}, {1, 1}, {0, 0}}, false},
		{"zero area", [][]float64{{0, 0}, {1, 1}, {2, 2}, {0, 0}}, false},
		{"self intersection", [][]float64{{0, 0}, {2, 2}, {0, 2}, {2, 0}, {0, 0}}, false},
		{"malformed pair", [][]float64{{0, 0}, {1}, {2, 2}, {0, 0}}, false},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got := Validate(test.points) == nil
			if got != test.valid {
				t.Fatalf("Validate() valid = %v, want %v", got, test.valid)
			}
		})
	}
}

func TestContainsPointInPolygon(t *testing.T) {
	tests := []struct {
		name string
		lat  float64
		lon  float64
		want bool
	}{
		{"inside", 37.5, -122.5, true},
		{"outside", 36.9, -122.5, false},
		{"on edge", 37.5, -123, true},
		{"on vertex", 37, -123, true},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if got := Contains(square, test.lat, test.lon); got != test.want {
				t.Fatalf("Contains() = %v, want %v", got, test.want)
			}
		})
	}
}

package main

import "testing"

func Test_getAspectRatio(t *testing.T) {
	tests := []struct {
		name string // description of this test case
		// Named input parameters for target function.
		w    float64
		h    float64
		want string
	}{
		{
			name: "normal landscape",
			w:    1280,
			h:    720,
			want: "16:9",
		},
		{
			name: "normal portrait",
			w:    720,
			h:    1280,
			want: "9:16",
		},
		{
			name: "square",
			w:    100,
			h:    100,
			want: "other",
		},
		{
			name: "off by one landscape",
			w:    1280,
			h:    721,
			want: "16:9",
		},
		{
			name: "off by one portrait",
			w:    721,
			h:    1280,
			want: "9:16",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := getAspectRatio(tt.w, tt.h)
			if got != tt.want {
				t.Errorf("getAspectRatio() = %v, want %v", got, tt.want)
			}
		})
	}
}

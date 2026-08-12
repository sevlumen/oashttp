package httpsem

import "testing"

func TestStatusAllowsBody(t *testing.T) {
	tests := []struct {
		status int
		want   bool
	}{
		{100, false},
		{101, false},
		{199, false},
		{200, true},
		{201, true},
		{204, false},
		{205, false},
		{304, false},
		{400, true},
		{500, true},
	}
	for _, tc := range tests {
		if got := StatusAllowsBody(tc.status); got != tc.want {
			t.Fatalf("StatusAllowsBody(%d)=%v want=%v", tc.status, got, tc.want)
		}
	}
}

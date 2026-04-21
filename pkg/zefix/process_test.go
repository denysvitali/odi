package zefix

import "testing"

func TestIsDisabledDSN(t *testing.T) {
	tests := []struct {
		name string
		dsn  string
		want bool
	}{
		{name: "empty", dsn: "", want: true},
		{name: "literal disabled", dsn: "user=disabled database=disabled", want: true},
		{name: "disabled with connection fields", dsn: "host=127.0.0.1 user=disabled database=disabled", want: true},
		{name: "postgres url", dsn: "postgresql://zefix:secret@postgres:5432/zefix?sslmode=disable", want: false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IsDisabledDSN(tt.dsn); got != tt.want {
				t.Fatalf("IsDisabledDSN(%q) = %v, want %v", tt.dsn, got, tt.want)
			}
		})
	}
}

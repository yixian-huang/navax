package maintenance

import (
	"testing"
	"time"
)

func TestMaintenanceWindow(t *testing.T) {
	day := "09:00-10:30"
	overnight := "23:00-01:00"
	for _, test := range []struct {
		at     string
		window *string
		want   bool
	}{
		{"09:30", &day, true}, {"10:30", &day, false}, {"23:30", &overnight, true},
		{"00:30", &overnight, true}, {"12:00", &overnight, false}, {"12:00", nil, true},
	} {
		at, err := time.Parse("15:04", test.at)
		if err != nil {
			t.Fatal(err)
		}
		if got := insideMaintenanceWindow(at, test.window); got != test.want {
			t.Fatalf("insideMaintenanceWindow(%s) = %v, want %v", test.at, got, test.want)
		}
	}
	if _, _, err := parseMaintenanceWindow("09:00-09:00"); err == nil {
		t.Fatal("zero maintenance window was accepted")
	}
}

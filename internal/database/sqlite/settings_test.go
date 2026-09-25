package sqlite

import (
	"context"
	"testing"
)

func TestTimezoneConfigDefaultStopsAtAdminOverride(t *testing.T) {
	ctx := context.Background()
	s := New(openMigrated(t))
	if err := s.SetUnmodifiedTimezoneDefault(ctx, "Asia/Shanghai"); err != nil {
		t.Fatal(err)
	}
	values, err := s.GetAllSettings(ctx)
	if err != nil || values["timezone"] != "Asia/Shanghai" {
		t.Fatalf("config default = %q, err = %v", values["timezone"], err)
	}
	if err := s.SetSettings(ctx, map[string]string{"timezone": "Europe/Berlin"}, "admin", 123); err != nil {
		t.Fatal(err)
	}
	if err := s.SetUnmodifiedTimezoneDefault(ctx, "America/New_York"); err != nil {
		t.Fatal(err)
	}
	values, err = s.GetAllSettings(ctx)
	if err != nil || values["timezone"] != "Europe/Berlin" {
		t.Fatalf("admin override = %q, err = %v", values["timezone"], err)
	}
}

package config_test

import (
	"testing"

	"github.com/wirvii/orcon/internal/platform/config"
)

func TestRoleValid(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name string
		role config.Role
		want bool
	}{
		{name: "standalone", role: config.RoleStandalone, want: true},
		{name: "api", role: config.RoleAPI, want: true},
		{name: "scheduler", role: config.RoleScheduler, want: true},
		{name: "worker", role: config.RoleWorker, want: true},
		{name: "empty", role: "", want: false},
		{name: "unknown", role: "invalid", want: false},
		{name: "case sensitive", role: "Standalone", want: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			if got := tt.role.Valid(); got != tt.want {
				t.Fatalf("Role(%q).Valid() = %v, want %v", tt.role, got, tt.want)
			}
		})
	}
}

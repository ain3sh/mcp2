package profile

import (
	"testing"

	"mcp2/internal/config"
)

func TestIsAllowed(t *testing.T) {
	tests := []struct {
		name     string
		filter   config.ComponentFilter
		toolName string
		want     bool
	}{
		{
			name: "empty allow, empty deny -> allow all",
			filter: config.ComponentFilter{
				Allow: nil,
				Deny:  nil,
			},
			toolName: "any",
			want:     true,
		},
		{
			name: "allow specific",
			filter: config.ComponentFilter{
				Allow: []string{"tool_a"},
				Deny:  nil,
			},
			toolName: "tool_a",
			want:     true,
		},
		{
			name: "allow specific, deny implicit",
			filter: config.ComponentFilter{
				Allow: []string{"tool_a"},
				Deny:  nil,
			},
			toolName: "tool_b",
			want:     false,
		},
		{
			name: "allow glob",
			filter: config.ComponentFilter{
				Allow: []string{"tool_*"},
				Deny:  nil,
			},
			toolName: "tool_a",
			want:     true,
		},
		{
			name: "deny takes precedence",
			filter: config.ComponentFilter{
				Allow: []string{"*"},
				Deny:  []string{"tool_a"},
			},
			toolName: "tool_a",
			want:     false,
		},
		{
			name: "deny takes precedence over empty allow",
			filter: config.ComponentFilter{
				Allow: nil,
				Deny:  []string{"tool_a"},
			},
			toolName: "tool_a",
			want:     false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isAllowed(tt.filter, tt.toolName); got != tt.want {
				t.Errorf("isAllowed() = %v, want %v", got, tt.want)
			}
		})
	}
}

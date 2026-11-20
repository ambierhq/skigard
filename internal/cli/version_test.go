package cli

import (
	"runtime/debug"
	"testing"
)

func TestResolveVersion(t *testing.T) {
	vcs := func(rev, mod string) []debug.BuildSetting {
		return []debug.BuildSetting{{Key: "vcs.revision", Value: rev}, {Key: "vcs.modified", Value: mod}}
	}
	tests := []struct {
		name    string
		ldflags string
		info    *debug.BuildInfo
		want    string
	}{
		{"ldflags win", "1.2.3", &debug.BuildInfo{Main: debug.Module{Version: "v9.9.9"}}, "1.2.3"},
		{"no info", "dev", nil, "dev"},
		{"module version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1"}}, "0.1.1"},
		{"prerelease", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.2.0-rc.1"}}, "0.2.0-rc.1"},
		{"devel no vcs", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}}, "dev"},
		{"empty", "", &debug.BuildInfo{}, "dev"},
		{"vcs clean", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs("abcdef1234567890", "false")}, "dev+abcdef1"},
		{"vcs dirty", "dev", &debug.BuildInfo{Main: debug.Module{Version: "(devel)"}, Settings: vcs("abcdef1234567890", "true")}, "dev+abcdef1-dirty"},
		{"pseudo version", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.2-0.20250101120000-abcdef123456"}}, "dev"},
		{"dirty module", "dev", &debug.BuildInfo{Main: debug.Module{Version: "v0.1.1+dirty"}, Settings: vcs("abcdef1234567890", "true")}, "dev+abcdef1-dirty"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := resolveVersion(tt.ldflags, tt.info); got != tt.want {
				t.Errorf("got %q, want %q", got, tt.want)
			}
		})
	}
}

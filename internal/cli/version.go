package cli

import (
	"runtime/debug"
	"strings"
)

// resolveVersion picks the version to report. A value injected with ldflags
// always wins; otherwise the module version of a `go install pkg@vX.Y.Z` build,
// then the VCS revision of a local build, then "dev".
func resolveVersion(ldflags string, info *debug.BuildInfo) string {
	if ldflags != "" && ldflags != "dev" {
		return ldflags
	}
	if info == nil {
		return "dev"
	}
	if v := info.Main.Version; v != "" && v != "(devel)" && !strings.Contains(v, "+dirty") && !isPseudoVersion(v) {
		return strings.TrimPrefix(v, "v")
	}
	var rev string
	var dirty bool
	for _, s := range info.Settings {
		switch s.Key {
		case "vcs.revision":
			rev = s.Value
		case "vcs.modified":
			dirty = s.Value == "true"
		}
	}
	if rev == "" {
		return "dev"
	}
	if len(rev) > 7 {
		rev = rev[:7]
	}
	if dirty {
		rev += "-dirty"
	}
	return "dev+" + rev
}

// isPseudoVersion reports whether v looks like vX.Y.Z-yyyymmddhhmmss-abcdef123456.
func isPseudoVersion(v string) bool {
	parts := strings.Split(v, "-")
	if len(parts) < 3 {
		return false
	}
	ts, rev := parts[len(parts)-2], parts[len(parts)-1]
	ts = ts[max(strings.LastIndex(ts, ".")+1, 0):]
	return len(ts) == 14 && len(rev) == 12
}

func init() {
	info, _ := debug.ReadBuildInfo()
	Version = resolveVersion(Version, info)
}

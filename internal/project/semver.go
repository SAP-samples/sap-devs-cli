package project

import (
	"strconv"
	"strings"
)

type semver struct {
	Major, Minor, Patch int
	Valid               bool
}

func parseSemver(s string) semver {
	s = strings.TrimLeft(s, "v")
	parts := strings.SplitN(s, ".", 3)
	if len(parts) < 2 {
		return semver{}
	}
	major, err1 := strconv.Atoi(parts[0])
	minor, err2 := strconv.Atoi(parts[1])
	patch := 0
	if len(parts) == 3 {
		// Strip pre-release suffix (e.g., "0-rc.1")
		p := strings.SplitN(parts[2], "-", 2)[0]
		patch, _ = strconv.Atoi(p)
	}
	if err1 != nil || err2 != nil {
		return semver{}
	}
	return semver{Major: major, Minor: minor, Patch: patch, Valid: true}
}

// CompareVersions returns -1 if a < b, 0 if equal (or unparseable), 1 if a > b.
func CompareVersions(a, b string) int {
	va, vb := parseSemver(a), parseSemver(b)
	if !va.Valid || !vb.Valid {
		return 0
	}
	if va.Major != vb.Major {
		if va.Major < vb.Major {
			return -1
		}
		return 1
	}
	if va.Minor != vb.Minor {
		if va.Minor < vb.Minor {
			return -1
		}
		return 1
	}
	if va.Patch != vb.Patch {
		if va.Patch < vb.Patch {
			return -1
		}
		return 1
	}
	return 0
}

// VersionStaleness returns "error" (>1 major behind), "warning" (>=2 minor behind),
// or "" (up-to-date or unparseable).
func VersionStaleness(current, latest string) string {
	vc, vl := parseSemver(current), parseSemver(latest)
	if !vc.Valid || !vl.Valid {
		return ""
	}
	if vl.Major-vc.Major > 0 {
		return "error"
	}
	if vc.Major == vl.Major && vl.Minor-vc.Minor >= 2 {
		return "warning"
	}
	return ""
}

package core

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

type splitPattern struct {
	re       *regexp.Regexp
	extGroup string
}

var splitPatterns = []splitPattern{
	{regexp.MustCompile(`(?i)^(?P<base>.+)_(?P<num>\d+)\.pkgpart$`), ""},
	{regexp.MustCompile(`(?i)^(?P<base>.+\.pkg)\.(?P<num>\d+)$`), ""},
	{regexp.MustCompile(`(?i)^(?P<base>.+\.pkg)_(?P<num>\d+)$`), ""},
	{regexp.MustCompile(`(?i)^(?P<base>.+)_(?P<num>\d+)(?P<ext>\.pkg)$`), "ext"},
	{regexp.MustCompile(`(?i)^(?P<base>.+)\.part(?P<num>\d+)(?P<ext>\.pkg)$`), "ext"},
}

func namedGroup(re *regexp.Regexp, match []string, name string) string {
	for i, n := range re.SubexpNames() {
		if n == name && i < len(match) {
			return match[i]
		}
	}
	return ""
}

type numberedPart struct {
	num  int
	path string
}

// DetectParts finds all split parts related to filePath and returns them
// in the correct merge order together with a suggested output filename.
func DetectParts(filePath string) ([]string, string) {
	dir := filepath.Dir(filePath)
	name := filepath.Base(filePath)

	for _, sp := range splitPatterns {
		m := sp.re.FindStringSubmatch(name)
		if m == nil {
			continue
		}

		base := namedGroup(sp.re, m, "base")
		ext := ""
		if sp.extGroup != "" {
			ext = namedGroup(sp.re, m, sp.extGroup)
		}

		entries, err := os.ReadDir(dir)
		if err != nil {
			continue
		}

		var parts []numberedPart
		for _, e := range entries {
			if e.IsDir() {
				continue
			}
			em := sp.re.FindStringSubmatch(e.Name())
			if em == nil {
				continue
			}
			if namedGroup(sp.re, em, "base") != base {
				continue
			}
			if sp.extGroup != "" && namedGroup(sp.re, em, sp.extGroup) != ext {
				continue
			}
			num, _ := strconv.Atoi(namedGroup(sp.re, em, "num"))
			parts = append(parts, numberedPart{num, filepath.Join(dir, e.Name())})
		}

		if len(parts) == 0 {
			continue
		}

		sort.Slice(parts, func(i, j int) bool { return parts[i].num < parts[j].num })

		baseCandidate := filepath.Join(dir, base+ext)
		known := make(map[string]bool, len(parts))
		for _, p := range parts {
			known[p.path] = true
		}
		if info, err := os.Stat(baseCandidate); err == nil && !info.IsDir() && !known[baseCandidate] {
			parts = append([]numberedPart{{-1, baseCandidate}}, parts...)
		}

		ordered := make([]string, len(parts))
		for i, p := range parts {
			ordered[i] = p.path
		}

		outputName := base + ext
		if !strings.HasSuffix(strings.ToLower(outputName), ".pkg") {
			outputName += ".pkg"
		}
		return ordered, outputName
	}

	return []string{filePath}, filepath.Base(filePath)
}

// SuggestOutputPath returns a non-colliding output path for the merged file.
func SuggestOutputPath(parts []string, detectedName string) string {
	if len(parts) == 0 {
		return ""
	}
	dir := filepath.Dir(parts[0])
	candidate := filepath.Join(dir, detectedName)

	sources := make(map[string]bool, len(parts))
	for _, p := range parts {
		abs, _ := filepath.Abs(p)
		sources[abs] = true
	}
	abs, _ := filepath.Abs(candidate)
	if sources[abs] {
		ext := filepath.Ext(detectedName)
		stem := strings.TrimSuffix(detectedName, ext)
		candidate = filepath.Join(dir, stem+"_merged"+ext)
	}
	return candidate
}

package core

import "fmt"

// FormatSize returns a human-readable byte count (e.g. "1.50 KB").
func FormatSize(b int64) string {
	const unit = 1024
	if b < unit {
		return fmt.Sprintf("%d B", b)
	}
	div, exp := int64(unit), 0
	for n := b / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(b)/float64(div), "KMGTPE"[exp])
}

// FormatTime returns a duration as MM:SS or HH:MM:SS.
func FormatTime(seconds float64) string {
	if seconds < 0 || seconds > 360_000 {
		return "--:--"
	}
	s := int(seconds)
	h, m, sec := s/3600, (s%3600)/60, s%60
	if h > 0 {
		return fmt.Sprintf("%02d:%02d:%02d", h, m, sec)
	}
	return fmt.Sprintf("%02d:%02d", m, sec)
}

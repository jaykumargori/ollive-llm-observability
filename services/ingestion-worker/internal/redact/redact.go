package redact

import "regexp"

var patterns = []*regexp.Regexp{
	regexp.MustCompile(`[A-Za-z0-9._%+\-]+@[A-Za-z0-9.\-]+\.[A-Za-z]{2,}`),
	regexp.MustCompile(`(?i)(sk|xoxb|ghp|AIza)[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`(?:\+?\d[\d .\-()]{8,}\d)`),
}

func Text(s string) string {
	for _, re := range patterns {
		s = re.ReplaceAllString(s, "[REDACTED]")
	}
	return s
}

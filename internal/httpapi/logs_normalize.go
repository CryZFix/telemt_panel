package httpapi

import (
	"strings"
	"time"
	"unicode/utf8"
)

// stripLogControls removes terminal instructions, not log text. Newlines and
// tabs remain readable; control strings cannot swallow subsequent log lines.
// Plain messages take the allocation-free path. Every input byte is scanned
// at most a constant number of times, including malformed escape sequences.
func stripLogControls(msg string) string {
	// Journald may deliver invalid UTF-8 as a byte array. Repair it before
	// removal can join separated bytes into a new Unicode control character.
	msg = strings.ToValidUTF8(msg, "\uFFFD")
	first := strings.IndexFunc(msg, logControl)
	if first < 0 {
		return msg
	}
	var out strings.Builder
	out.Grow(len(msg))
	plain := 0
	for i := first; i < len(msg); {
		r, width := utf8.DecodeRuneInString(msg[i:])
		if !logControl(r) {
			i += width
			continue
		}
		out.WriteString(msg[plain:i])
		i += width
		switch r {
		case '\x1b':
			if i < len(msg) {
				switch msg[i] {
				case '[':
					i = skipLogCSI(msg, i+1)
				case ']', 'P', '^', '_', 'X':
					i = skipLogControlString(msg, i+1)
				default:
					for i < len(msg) && msg[i] >= 0x20 && msg[i] <= 0x2f {
						i++
					}
					if i < len(msg) && msg[i] >= 0x30 && msg[i] <= 0x7e {
						i++
					}
				}
			}
		case '\u009b':
			i = skipLogCSI(msg, i)
		case '\u009d', '\u0090', '\u009e', '\u009f', '\u0098':
			i = skipLogControlString(msg, i)
		}
		plain = i
	}
	out.WriteString(msg[plain:])
	return out.String()
}

func logControl(r rune) bool {
	return r < 0x20 && r != '\n' && r != '\r' && r != '\t' || r >= 0x7f && r <= 0x9f
}

func skipLogCSI(msg string, i int) int {
	for i < len(msg) {
		c := msg[i]
		if c >= 0x40 && c <= 0x7e {
			return i + 1
		}
		if c < 0x20 || c > 0x3f {
			return i
		}
		i++
	}
	return i
}

func skipLogControlString(msg string, i int) int {
	for i < len(msg) {
		switch msg[i] {
		case '\n', '\r':
			return i
		case '\a':
			return i + 1
		case '\x1b':
			if i+1 < len(msg) && msg[i+1] == '\\' {
				return i + 2
			}
		}
		if strings.HasPrefix(msg[i:], "\u009c") {
			return i + len("\u009c")
		}
		i++
	}
	return i
}

// telemtLogPrefix recognizes the console/file tracing format used by Telemt.
// A valid timestamp, level and crate target are required: arbitrary payload
// words and other services must not override the log source's priority.
// The original readable prefix stays in Msg; source timestamps take precedence.
func telemtLogPrefix(msg string) (time.Time, string, bool) {
	stamp, rest, ok := strings.Cut(msg, " ")
	if !ok || len(stamp) > 64 {
		return time.Time{}, "", false
	}
	level, rest, ok := strings.Cut(strings.TrimLeft(rest, " \t"), " ")
	if !ok {
		return time.Time{}, "", false
	}
	switch level {
	case "ERROR":
		level = "error"
	case "WARN":
		level = "warn"
	case "INFO":
		level = "info"
	case "DEBUG", "TRACE":
		level = "debug"
	default:
		return time.Time{}, "", false
	}
	target, _, ok := strings.Cut(strings.TrimLeft(rest, " \t"), " ")
	if !ok || !strings.HasSuffix(target, ":") || !(target == "telemt:" || strings.HasPrefix(target, "telemt::")) {
		return time.Time{}, "", false
	}
	for _, r := range target {
		if !(r == ':' || r == '_' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' || r >= '0' && r <= '9') {
			return time.Time{}, "", false
		}
	}
	ts, err := time.Parse(time.RFC3339Nano, stamp)
	return ts, level, err == nil
}

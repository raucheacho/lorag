package memory

import (
	"crypto/sha1"
	"encoding/hex"
	"regexp"
	"strings"
)

// Normalization turns two occurrences of the same error into the same text:
// what changes between machines and runs (paths, line numbers, versions,
// addresses, ids, numbers) is replaced by a placeholder.
var normalizers = []struct {
	re   *regexp.Regexp
	with string
}{
	{regexp.MustCompile(`\bhttps?://\S+`), "<url>"},
	{regexp.MustCompile(`[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}`), "<id>"},
	{regexp.MustCompile(`\b0x[0-9a-f]+\b`), "<addr>"},
	{regexp.MustCompile(`\b[0-9a-f]{12,}\b`), "<hash>"},
	// Windows and Unix paths, with an optional :line:col suffix.
	{regexp.MustCompile(`(?:[a-z]:)?(?:[\\/][\w.@~+\-]+){2,}[\\/]?(?::\d+)*`), "<path>"},
	{regexp.MustCompile(`\b[\w\-]+\.(?:js|jsx|ts|tsx|go|py|rb|rs|java|kt|swift|c|cc|cpp|h|zig|php|cs|mjs|cjs)(?::\d+)+`), "<file>"},
	{regexp.MustCompile(`\bv?\d+(?:\.\d+)+(?:[-+][\w.]+)?\b`), "<ver>"},
	{regexp.MustCompile(`\bline \d+\b`), "line <n>"},
	{regexp.MustCompile(`\b\d+\b`), "<n>"},
	{regexp.MustCompile(`\s+`), " "},
}

// errorLine finds the line that states the error in a log or stack trace:
// the first line mentioning an error, else the first non-empty line.
var errorWord = regexp.MustCompile(`(?i)\b(error|exception|fail(ed|ure)?|panic|fatal|cannot|can't|unable|not found|denied|refused|undefined|invalid)\b|\b[A-Z]{1,5}_?[A-Z0-9_]*\d{2,}\b|\bE[A-Z]{3,}\b`)

var errorPrefix = regexp.MustCompile(`(?i)^\W*(?:(?:npm|yarn|pnpm)\s+)?(?:(?:uncaught|unhandled)\s+)?(?:[\w.$]*(?:error|exception)|\[?(?:err!|err\b|fatal|panic|warn(?:ing)?)\]?)\s*(?:\[[^\]]*\])?\s*:?\s*`)

// ExactNeedles returns the strings to look for verbatim, most specific
// first: the error line, then the same without its "Error:" or
// "npm ERR!" prefix, which logs and documents often omit.
func ExactNeedles(text string) []string {
	line := strings.TrimSpace(errorLine(text))
	out := []string{line}
	if bare := strings.TrimSpace(errorPrefix.ReplaceAllString(line, "")); len(bare) >= 8 && bare != line {
		out = append(out, bare)
	}
	return out
}

// ErrorLine returns the line of text that states the error.
func ErrorLine(text string) string { return errorLine(text) }

func errorLine(text string) string {
	var first string
	for _, l := range strings.Split(text, "\n") {
		l = strings.TrimSpace(l)
		if l == "" || strings.HasPrefix(l, "at ") || strings.HasPrefix(l, "File \"") {
			continue
		}
		if first == "" {
			first = l
		}
		if errorWord.MatchString(l) {
			return l
		}
	}
	return first
}

// Normalize returns the stable form of an error message.
func Normalize(text string) string {
	s := strings.ToLower(errorLine(text))
	for _, n := range normalizers {
		s = n.re.ReplaceAllString(s, n.with)
	}
	return strings.TrimSpace(s)
}

// Signature identifies a problem across machines and runs: 10 hex digits of
// the hash of its normalized error (or of its description when there is no
// error message).
func Signature(errorText, problem string) string {
	key := Normalize(errorText)
	if key == "" {
		key = Normalize(problem)
	}
	sum := sha1.Sum([]byte(key))
	return hex.EncodeToString(sum[:])[:10]
}

// LooksLikeError reports whether a query is an error message rather than a
// question, so it can go to the exact route and the signature first.
func LooksLikeError(text string) bool {
	if strings.Contains(text, "\n") && errorWord.MatchString(text) {
		return true
	}
	t := strings.TrimSpace(text)
	return errorWord.MatchString(t) && (strings.Contains(t, ":") || strings.Contains(t, "'") || strings.Contains(t, "`") || strings.Contains(t, "\""))
}

// Secrets are masked before anything is written: an agent pasting a log
// must not store an API key in its memory.
var secrets = []*regexp.Regexp{
	regexp.MustCompile(`-----BEGIN [A-Z ]*PRIVATE KEY-----[\s\S]*?-----END [A-Z ]*PRIVATE KEY-----`),
	regexp.MustCompile(`\b(sk|pk|rk)[-_](live|test|proj|ant)?[-_]?[A-Za-z0-9_\-]{16,}`),
	regexp.MustCompile(`\b(ghp|gho|ghu|ghs|ghr|github_pat)_[A-Za-z0-9_]{20,}`),
	regexp.MustCompile(`\bxox[abprs]-[A-Za-z0-9\-]{10,}`),
	regexp.MustCompile(`\bAKIA[0-9A-Z]{16}\b`),
	regexp.MustCompile(`\bAIza[0-9A-Za-z_\-]{35}\b`),
	regexp.MustCompile(`\beyJ[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}\.[A-Za-z0-9_\-]{10,}`),
	regexp.MustCompile(`(?i)\b(bearer)\s+[A-Za-z0-9._\-]{16,}`),
	regexp.MustCompile(`(?i)\b[\w.\-]*(password|passwd|pwd|secret|token|api[_-]?key|access[_-]?key)[\w.\-]*(\s*[:=]\s*)["']?[^\s"']{6,}`),
	regexp.MustCompile(`(?i)\b[a-z][a-z0-9+.\-]*://[^\s/:@]+:[^\s/@]+@`),
}

// MaskSecrets replaces credentials with [secret]. Key names are kept so the
// solution still reads ("API_KEY=[secret]").
func MaskSecrets(s string) string {
	for i, re := range secrets {
		s = re.ReplaceAllStringFunc(s, func(m string) string {
			switch i {
			case 7: // bearer <token>
				return m[:len("bearer")] + " [secret]"
			case 8: // key = value
				loc := re.FindStringSubmatchIndex(m)
				return m[:loc[5]] + "[secret]"
			case 9: // scheme://user:pass@
				at := strings.LastIndex(m, ":")
				return m[:at] + ":[secret]@"
			}
			return "[secret]"
		})
	}
	return s
}

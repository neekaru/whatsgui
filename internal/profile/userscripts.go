package profile

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

const (
	// MaxUserscriptBytes bounds the size of a single userscript or @require file.
	MaxUserscriptBytes = 2 << 20
	// MaxResolvedUserscriptBytes bounds the combined source after resolving @require.
	MaxResolvedUserscriptBytes = 4 << 20

	RunAtDocumentStart = "document-start"
	RunAtDocumentEnd   = "document-end"
	RunAtDocumentIdle  = "document-idle"
)

// UserscriptPayload is an enabled userscript pushed from the workspace to a
// profile worker over the private pipe.
type UserscriptPayload struct {
	ID          string   `json:"id"`
	Name        string   `json:"name"`
	Version     string   `json:"version,omitempty"`
	Description string   `json:"description,omitempty"`
	Author      string   `json:"author,omitempty"`
	Namespace   string   `json:"namespace,omitempty"`
	RunAt       string   `json:"runAt,omitempty"`
	Grants      []string `json:"grants,omitempty"`
	Matches     []string `json:"matches,omitempty"`
	Code        string   `json:"code"`
	Hash        string   `json:"hash"`
}

// UserscriptMetadata holds standard ==UserScript== header fields.
type UserscriptMetadata struct {
	Name        string
	Namespace   string
	Description string
	Version     string
	Author      string
	RunAt       string
	Matches     []string
	Requires    []string
	Grants      []string
}

// HashUserscript returns a short deterministic digest of the script source so
// edited scripts re-run cleanly when pushed to an already-loaded page.
func HashUserscript(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:8])
}

// NormalizeRunAt maps @run-at values to one of the supported lifecycle stages.
func NormalizeRunAt(raw string) string {
	switch strings.ToLower(strings.TrimSpace(raw)) {
	case RunAtDocumentStart:
		return RunAtDocumentStart
	case RunAtDocumentEnd:
		return RunAtDocumentEnd
	default:
		return RunAtDocumentIdle
	}
}

// ParseUserscriptMetadata extracts ==UserScript== header metadata from code.
// Missing names fall back to filename without its .user.js/.js suffix.
func ParseUserscriptMetadata(filename, code string) UserscriptMetadata {
	meta := UserscriptMetadata{RunAt: RunAtDocumentIdle}
	inHeader := false
	for rawLine := range strings.SplitSeq(code, "\n") {
		line := strings.TrimSpace(strings.TrimSuffix(rawLine, "\r"))
		if !strings.HasPrefix(line, "//") {
			if inHeader {
				break
			}
			continue
		}
		comment := strings.TrimSpace(strings.TrimPrefix(line, "//"))
		if comment == "==UserScript==" {
			inHeader = true
			continue
		}
		if comment == "==/UserScript==" {
			break
		}
		if !inHeader || !strings.HasPrefix(comment, "@") {
			continue
		}
		rest := comment[1:]
		idx := strings.IndexAny(rest, " \t")
		if idx < 0 {
			continue
		}
		key := strings.ToLower(rest[:idx])
		val := strings.TrimSpace(rest[idx+1:])
		if val == "" {
			continue
		}
		switch key {
		case "name":
			if meta.Name == "" {
				meta.Name = val
			}
		case "namespace":
			if meta.Namespace == "" {
				meta.Namespace = val
			}
		case "description":
			if meta.Description == "" {
				meta.Description = val
			}
		case "version":
			if meta.Version == "" {
				meta.Version = val
			}
		case "author":
			if meta.Author == "" {
				meta.Author = val
			}
		case "run-at":
			meta.RunAt = NormalizeRunAt(val)
		case "match", "include":
			meta.Matches = append(meta.Matches, val)
		case "require":
			meta.Requires = append(meta.Requires, val)
		case "grant":
			meta.Grants = append(meta.Grants, val)
		}
	}
	if meta.Name == "" {
		meta.Name = stripUserscriptSuffix(filename)
	}
	return meta
}

func stripUserscriptSuffix(filename string) string {
	lower := strings.ToLower(filename)
	switch {
	case strings.HasSuffix(lower, ".user.js"):
		return filename[:len(filename)-len(".user.js")]
	case strings.HasSuffix(lower, ".js"):
		return filename[:len(filename)-len(".js")]
	default:
		return filename
	}
}

// MatchesWhatsApp reports whether a userscript's @match/@include rules allow
// running on https://web.whatsapp.com/. Scripts without rules default to true.
func MatchesWhatsApp(matches []string) bool {
	if len(matches) == 0 {
		return true
	}
	for _, raw := range matches {
		if matchWhatsAppPattern(strings.TrimSpace(raw)) {
			return true
		}
	}
	return false
}

func matchWhatsAppPattern(pattern string) bool {
	if pattern == "" {
		return false
	}
	if pattern == "<all_urls>" || pattern == "*" {
		return true
	}
	scheme, rest, hasScheme := strings.Cut(pattern, "://")
	if !hasScheme {
		return wildcardMatch(strings.ToLower(pattern), "https://web.whatsapp.com/")
	}
	scheme = strings.ToLower(scheme)
	if scheme != "*" && scheme != "https" {
		return false
	}
	host, urlPath, hasPath := strings.Cut(rest, "/")
	host = strings.ToLower(host)
	hostMatches := host == "*" || host == "web.whatsapp.com" ||
		(strings.HasPrefix(host, "*.") && strings.HasSuffix("web.whatsapp.com", host[1:])) ||
		wildcardMatch(host, "web.whatsapp.com")
	if !hostMatches {
		return false
	}
	if !hasPath || urlPath == "" || urlPath == "*" {
		return true
	}
	return wildcardMatch("/"+urlPath, "/")
}

func wildcardMatch(pattern, text string) bool {
	pIdx, tIdx := 0, 0
	starIdx, matchIdx := -1, 0
	for tIdx < len(text) {
		if pIdx < len(pattern) && (pattern[pIdx] == text[tIdx]) {
			pIdx++
			tIdx++
		} else if pIdx < len(pattern) && pattern[pIdx] == '*' {
			starIdx = pIdx
			matchIdx = tIdx
			pIdx++
		} else if starIdx != -1 {
			pIdx = starIdx + 1
			matchIdx++
			tIdx = matchIdx
		} else {
			return false
		}
	}
	for pIdx < len(pattern) && pattern[pIdx] == '*' {
		pIdx++
	}
	return pIdx == len(pattern)
}

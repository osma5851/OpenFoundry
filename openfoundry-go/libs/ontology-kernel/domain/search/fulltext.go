// Lexical (token + exact-match) full-text scorer for ontology search.
//
// Mirrors `libs/ontology-kernel/src/domain/search/fulltext.rs`.
// Pure logic, no IO. Used by the hybrid orchestrator in
// [search.go] as the lexical half of the score.

package search

import "strings"

// Tokenize mirrors `pub fn tokenize`. Splits on any character that
// is not alphanumeric, `_`, or `-`, drops empty fragments, and
// lower-cases each surviving token.
func Tokenize(input string) []string {
	out := []string{}
	current := strings.Builder{}
	flush := func() {
		if current.Len() > 0 {
			out = append(out, strings.ToLower(current.String()))
			current.Reset()
		}
	}
	for _, r := range input {
		if isTokenChar(r) {
			current.WriteRune(r)
		} else {
			flush()
		}
	}
	flush()
	return out
}

func isTokenChar(r rune) bool {
	if r == '_' || r == '-' {
		return true
	}
	if r >= '0' && r <= '9' {
		return true
	}
	if r >= 'a' && r <= 'z' {
		return true
	}
	if r >= 'A' && r <= 'Z' {
		return true
	}
	// Honour `is_alphanumeric` for non-ASCII letters/digits.
	return isUnicodeLetter(r) || isUnicodeDigit(r)
}

// isUnicodeLetter / isUnicodeDigit — minimal unicode predicates so we
// match Rust `char::is_alphanumeric` for the common Latin-extended +
// CJK token shapes search hits across.
func isUnicodeLetter(r rune) bool {
	switch {
	case r < 0x80:
		return false
	case r >= 0x00C0 && r <= 0x024F:
		return true
	case r >= 0x0370 && r <= 0x03FF:
		return true
	case r >= 0x0400 && r <= 0x04FF:
		return true
	case r >= 0x4E00 && r <= 0x9FFF:
		return true
	case r >= 0x3040 && r <= 0x30FF:
		return true
	}
	return false
}

func isUnicodeDigit(r rune) bool {
	return r >= 0x0660 && r <= 0x0669 // Arabic-Indic digits, the most common non-ASCII numeric shape callers feed into search.
}

// LexicalScore mirrors `pub fn score` from `fulltext.rs`. Returns
// a [0, ~1.5] lexical score over (title, body) given the query —
// see the Rust comments for the coverage + exact-match weighting.
//
// (Renamed from `Score` because the Go port flattens
// `domain::search::{fulltext, semantic}` into a single package; the
// equivalent semantic.rs entry point is [SemanticScore].)
func LexicalScore(query, title, body string) float32 {
	queryTokens := Tokenize(query)
	if len(queryTokens) == 0 {
		return 0
	}
	titleTokens := Tokenize(title)
	bodyTokens := Tokenize(body)

	querySet := stringSet(queryTokens)
	titleSet := stringSet(titleTokens)
	bodySet := stringSet(bodyTokens)

	titleHits := float32(0)
	bodyHits := float32(0)
	for token := range querySet {
		if titleSet[token] {
			titleHits++
		}
		if bodySet[token] {
			bodyHits++
		}
	}
	denom := float32(len(querySet))
	if denom < 1 {
		denom = 1
	}
	coverage := (titleHits*1.5 + bodyHits) / denom

	loweredQuery := strings.ToLower(strings.TrimSpace(query))
	loweredTitle := strings.ToLower(title)
	loweredBody := strings.ToLower(body)

	exactTitle := float32(0)
	if loweredQuery != "" && strings.Contains(loweredTitle, loweredQuery) {
		exactTitle = 0.35
	}
	exactBody := float32(0)
	if loweredQuery != "" && strings.Contains(loweredBody, loweredQuery) {
		exactBody = 0.15
	}

	scaled := coverage / 2.5
	if scaled > 1.0 {
		scaled = 1.0
	}
	return scaled + exactTitle + exactBody
}

func stringSet(values []string) map[string]bool {
	out := make(map[string]bool, len(values))
	for _, v := range values {
		out[v] = true
	}
	return out
}

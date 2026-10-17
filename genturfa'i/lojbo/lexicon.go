package lojbo

import (
	"strings"
)

type lexicon struct {
	// wordCats maps a written cmavo to the selma'o it belongs to.
	wordCats map[string]map[string]bool
	// cmavo is the set of simple cmavo, used to split compound cmavo.
	cmavo map[string]bool
}

func newLexicon(prods map[string]*production) *lexicon {
	l := &lexicon{
		wordCats: make(map[string]map[string]bool),
		cmavo:    make(map[string]bool),
	}
	memo := make(map[string]map[string]bool)
	failed := make(map[string]bool)

	var finite func(name string) (map[string]bool, bool)
	var finiteExpr func(e *expr) (map[string]bool, bool)
	finiteExpr = func(e *expr) (map[string]bool, bool) {
		if e == nil {
			return map[string]bool{"": true}, true
		}
		switch e.kind {
		case kindEmpty:
			return map[string]bool{"": true}, true
		case kindToken:
			return map[string]bool{e.text: true}, true
		case kindName:
			return finite(e.name)
		case kindAlt:
			out := make(map[string]bool)
			for _, k := range e.kids {
				s, ok := finiteExpr(k)
				if !ok {
					return nil, false
				}
				for w := range s {
					out[w] = true
				}
			}
			return out, true
		case kindSeq:
			cur := map[string]bool{"": true}
			for _, k := range e.kids {
				s, ok := finiteExpr(k)
				if !ok {
					return nil, false
				}
				next := make(map[string]bool)
				for a := range cur {
					for b := range s {
						next[a+b] = true
					}
				}
				cur = next
			}
			return cur, true
		case kindGroup:
			if len(e.kids) == 1 {
				return finiteExpr(e.kids[0])
			}
			return map[string]bool{"": true}, true
		case kindOption:
			out := map[string]bool{"": true}
			if len(e.kids) == 1 {
				s, ok := finiteExpr(e.kids[0])
				if !ok {
					return nil, false
				}
				for w := range s {
					out[w] = true
				}
			}
			return out, true
		case kindRep:
			if len(e.kids) == 1 {
				s, ok := finiteExpr(e.kids[0])
				if !ok {
					return nil, false
				}
				if len(s) == 1 && s[""] {
					return map[string]bool{"": true}, true
				}
			}
			return nil, false
		}
		return nil, false
	}

	finite = func(name string) (map[string]bool, bool) {
		if s, ok := memo[name]; ok {
			return s, true
		}
		if failed[name] {
			return nil, false
		}
		prod, ok := prods[name]
		if !ok || !isLexical(name) {
			failed[name] = true
			return nil, false
		}
		s, ok := finiteExpr(prod.expr)
		if !ok {
			failed[name] = true
			return nil, false
		}
		memo[name] = s
		return s, true
	}

	for name := range prods {
		if !isLexical(name) || name == "cBRIVLA" || name == "cCMEVLA" {
			continue
		}
		if !(len(name) > 1 && name[0] == 'c' && name[1] >= 'A' && name[1] <= 'Z') {
			continue
		}
		words, ok := finite(name)
		if !ok {
			continue
		}
		for w := range words {
			if w == "" || strings.ContainsAny(w, " ") {
				continue
			}
			if l.wordCats[w] == nil {
				l.wordCats[w] = make(map[string]bool)
			}
			l.wordCats[w][name] = true
			l.cmavo[w] = true
		}
	}
	return l
}

var consonantSet = map[rune]bool{
	'b': true, 'c': true, 'd': true, 'f': true, 'g': true,
	'j': true, 'k': true, 'l': true, 'm': true, 'n': true,
	'p': true, 'r': true, 's': true, 't': true, 'v': true,
	'x': true, 'z': true,
}

func isVowel(r rune) bool {
	switch r {
	case 'a', 'e', 'i', 'o', 'u':
		return true
	}
	return false
}

func isConsonant(r rune) bool { return consonantSet[r] }

// isBrivla recognises the shape of a gismu, lujvo or fu'ivla: it ends in a
// vowel and contains a consonant pair among its first five letters.
func isBrivla(letters string) bool {
	rs := []rune(letters)
	if len(rs) == 0 || !isVowel(rs[len(rs)-1]) {
		return false
	}
	var core []rune
	for _, r := range rs {
		if r == '\'' || r == 'y' || r == '.' {
			continue
		}
		core = append(core, r)
		if len(core) == 5 {
			break
		}
	}
	for i := 1; i < len(core); i++ {
		if isConsonant(core[i-1]) && isConsonant(core[i]) {
			return true
		}
	}
	return false
}

// isCmevla recognises a name: it ends in a consonant.
func isCmevla(letters string) bool {
	rs := []rune(letters)
	if len(rs) == 0 {
		return false
	}
	return isConsonant(rs[len(rs)-1])
}

func (l *lexicon) classify(word string) map[string]bool {
	cats := make(map[string]bool)
	if c, ok := l.wordCats[word]; ok {
		for k := range c {
			cats[k] = true
		}
	}
	letters := strings.ReplaceAll(word, ".", "")
	if isBrivla(letters) {
		cats["cBRIVLA"] = true
	}
	if isCmevla(letters) {
		cats["cCMEVLA"] = true
	}
	return cats
}

func (l *lexicon) isWord(word string) bool {
	if _, ok := l.wordCats[word]; ok {
		return true
	}
	letters := strings.ReplaceAll(word, ".", "")
	return isBrivla(letters) || isCmevla(letters)
}

func (l *lexicon) tokenize(input string) []Token {
	fields := strings.Fields(input)
	var out []Token
	wordID := 0
	for _, raw := range fields {
		// A comma is an optional pause, so it does not split a name such as
		// .lu,is.karol.
		raw = strings.ReplaceAll(raw, ",", ".")
		word := strings.Trim(raw, ".«»“”\"()[]<>;:!?")
		if word == "" {
			continue
		}
		// The erasure cmavo and FAhO are handled before parsing, as the
		// reference parser does.
		switch word {
		case "fa'o":
			return out
		case "ba'e", "za'e":
			// BAhE emphasises or marks the next word. It does not affect
			// the syntax, so it is dropped.
			wordID++
			continue
		case "si":
			if len(out) > 0 {
				last := out[len(out)-1].Word
				for len(out) > 0 && out[len(out)-1].Word == last {
					out = out[:len(out)-1]
				}
			}
			wordID++
			continue
		case "sa", "su":
			for len(out) > 0 && !isBoundary(out[len(out)-1].Text) {
				out = out[:len(out)-1]
			}
			wordID++
			continue
		}
		if l.isWord(word) {
			out = append(out, Token{Raw: raw, Text: word, Word: wordID})
			wordID++
			continue
		}
		// A compound cmavo has no consonant pair. Split it into simple cmavo,
		// ignoring the pause dots that may separate its parts.
		joined := strings.ReplaceAll(word, ".", "")
		if parts, ok := l.split(joined); ok && len(parts) > 1 {
			for _, part := range parts {
				out = append(out, Token{Raw: part, Text: part, Word: wordID})
			}
			wordID++
			continue
		}
		out = append(out, Token{Raw: raw, Text: word, Word: wordID})
		wordID++
	}
	return out
}

func isBoundary(word string) bool {
	switch word {
	case "i", "ni'o":
		return true
	}
	return false
}

// split segments a compound cmavo. It prefers the longest first part and
// backtracks when that leads to a dead end.
func (l *lexicon) split(s string) ([]string, bool) {
	memo := make(map[int][]string)
	bad := make(map[int]bool)
	var rec func(int) ([]string, bool)
	rec = func(i int) ([]string, bool) {
		if i == len(s) {
			return nil, true
		}
		if v, ok := memo[i]; ok {
			return v, true
		}
		if bad[i] {
			return nil, false
		}
		for j := len(s); j > i; j-- {
			part := s[i:j]
			if !l.cmavo[part] {
				continue
			}
			if rest, ok := rec(j); ok {
				out := append([]string{part}, rest...)
				memo[i] = out
				return out, true
			}
		}
		bad[i] = true
		return nil, false
	}
	return rec(0)
}

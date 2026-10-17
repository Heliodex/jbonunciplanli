// Package lojbo implements a parser for Lojban text driven by the EBNF
// grammar in "bynyfys pa".
package lojbo

import (
	"fmt"
	"maps"
	"sort"
	"strings"

	ebnf "genturfa_i/bynyfys"
)

// anyWordCost penalises readings that treat a word as a catch-all word
// instead of using its real category.
const anyWordCost = 1

// Token is one Lojban word. Raw is the word as written, Text is the word
// with pause dots removed. A compound cmavo such as "lonu" becomes two
// tokens ("lo", "nu").
type Token struct {
	Raw  string
	Text string
	// Word is the number of the written word this token came from. A
	// compound cmavo such as "lonu" gives two tokens with the same Word, so
	// erasure with si can remove the whole written word.
	Word int
}

// Node is a parse tree node. A node with a name is either a grammar
// production or a lexical category. Anonymous nodes only group children.
type Node struct {
	Name string
	Text string
	Kids []*Node
	From int
	To   int
	// Cost is used to choose between ambiguous parses. Matching a word with
	// the catch-all any_word costs more than matching it with a real
	// category, so the parser prefers a reading that uses every word for
	// what it actually is.
	Cost int
}

// String renders the tree, one production per line, leaving out the empty
// productions that the grammar allows everywhere.
func (n *Node) String() string {
	var b strings.Builder
	n.write(&b, 0)
	return b.String()
}

// hasText reports whether a subtree contains at least one word.
func (n *Node) hasText() bool {
	if n == nil {
		return false
	}
	if n.Text != "" {
		return true
	}
	for _, k := range n.Kids {
		if k.hasText() {
			return true
		}
	}
	return false
}

func (n *Node) write(b *strings.Builder, depth int) {
	if n == nil {
		return
	}
	for n.Name == "" && n.Text == "" && len(n.Kids) == 1 {
		n = n.Kids[0]
	}
	if n.Name != "" || n.Text != "" {
		b.WriteString(strings.Repeat("  ", depth))
		if n.Name != "" {
			b.WriteString(n.Name)
		}
		if n.Text != "" {
			if n.Name != "" {
				b.WriteByte(' ')
			}
			b.WriteString(n.Text)
		}
		b.WriteByte('\n')
		depth++
	}
	for _, k := range n.Kids {
		if !k.hasText() {
			continue
		}
		k.write(b, depth)
	}
}

// Error reports where parsing stopped and what could have continued there.
type Error struct {
	Token    int
	Word     string
	Expected []string
}

func (e *Error) Error() string {
	word := e.Word
	if word == "" {
		word = "fa'o"
	}
	if len(e.Expected) == 0 {
		return fmt.Sprintf("fliba fi lo valsi be li %d (%s)", e.Token+1, word)
	}
	return fmt.Sprintf("fliba fi lo valsi be li %d (%s) .i se kakne lo nu jmina %s",
		e.Token+1, word, strings.Join(e.Expected, " | "))
}

type result map[int]*Node

type exprKind uint8

const (
	kindEmpty exprKind = iota
	kindToken
	kindName
	kindAlt
	kindSeq
	kindGroup
	kindOption
	kindRep
)

type expr struct {
	id   int
	kind exprKind
	text string
	name string
	kids []*expr
}

type production struct {
	name string
	expr *expr
}

type memoKey struct {
	id  int
	pos int
}

type nameKey struct {
	name string
	pos  int
}

// Parser parses tokenized Lojban using a compiled EBNF grammar.
type Parser struct {
	grammar ebnf.Grammar

	prods map[string]*production
	lex   *lexicon

	tokens []Token
	cats   []map[string]bool

	nameMemo map[nameKey]result
	exprMemo map[memoKey]result

	anything map[int]result

	maxPos   int
	expected map[string]bool

	nextExprID int
	inProgress map[nameKey]bool
}

// New compiles an EBNF grammar. The start production is "text".
func New(g ebnf.Grammar) (*Parser, error) {
	p := &Parser{
		grammar:    g,
		prods:      make(map[string]*production),
		inProgress: make(map[nameKey]bool),
	}
	for name := range g {
		p.prods[name] = &production{name: name}
	}
	for name, prod := range g {
		e, err := p.compile(plusify(prod.Expr))
		if err != nil {
			return nil, fmt.Errorf("%s: %w", name, err)
		}
		p.prods[name].expr = e
	}
	if _, ok := p.prods["text"]; !ok {
		return nil, fmt.Errorf("missing production text")
	}
	p.lex = newLexicon(p.prods)
	return p, nil
}

// plusify rewrites the grammar's {...}. In "bynyfys pa" braces come from the
// CLL "..." operator, which means one or more, not the zero or more that the
// EBNF library assumes. X becomes X {X}. Wrapping braces in [] still gives
// zero or more, which is what expressions like [{free}] mean.
func plusify(x ebnf.Expression) ebnf.Expression {
	switch x := x.(type) {
	case nil:
		return nil
	case ebnf.Alternative:
		y := make(ebnf.Alternative, len(x))
		for i, k := range x {
			y[i] = plusify(k)
		}
		return y
	case ebnf.Sequence:
		y := make(ebnf.Sequence, len(x))
		for i, k := range x {
			y[i] = plusify(k)
		}
		return y
	case *ebnf.Group:
		return &ebnf.Group{Lparen: x.Lparen, Body: plusify(x.Body)}
	case *ebnf.Option:
		return &ebnf.Option{Lbrack: x.Lbrack, Body: plusify(x.Body)}
	case *ebnf.Repetition:
		body := plusify(x.Body)
		return ebnf.Sequence{body, &ebnf.Repetition{Lbrace: x.Lbrace, Body: body}}
	case *ebnf.Production:
		return &ebnf.Production{Name: x.Name, Expr: plusify(x.Expr)}
	default:
		return x
	}
}

func (p *Parser) compile(x ebnf.Expression) (*expr, error) {
	p.nextExprID++
	e := &expr{id: p.nextExprID}
	switch x := x.(type) {
	case nil:
		e.kind = kindEmpty
	case ebnf.Alternative:
		e.kind = kindAlt
		for _, k := range x {
			ke, err := p.compile(k)
			if err != nil {
				return nil, err
			}
			e.kids = append(e.kids, ke)
		}
	case ebnf.Sequence:
		e.kind = kindSeq
		for _, k := range x {
			ke, err := p.compile(k)
			if err != nil {
				return nil, err
			}
			e.kids = append(e.kids, ke)
		}
	case *ebnf.Name:
		e.kind = kindName
		e.name = x.String
	case *ebnf.Token:
		e.kind = kindToken
		e.text = x.String
	case *ebnf.Range:
		return nil, fmt.Errorf("character ranges are not supported")
	case *ebnf.Group:
		e.kind = kindGroup
		k, err := p.compile(x.Body)
		if err != nil {
			return nil, err
		}
		e.kids = []*expr{k}
	case *ebnf.Option:
		e.kind = kindOption
		k, err := p.compile(x.Body)
		if err != nil {
			return nil, err
		}
		e.kids = []*expr{k}
	case *ebnf.Repetition:
		e.kind = kindRep
		k, err := p.compile(x.Body)
		if err != nil {
			return nil, err
		}
		e.kids = []*expr{k}
	case *ebnf.Production:
		return p.compile(x.Expr)
	case *ebnf.Bad:
		return nil, fmt.Errorf("%s", x.Error)
	default:
		return nil, fmt.Errorf("unexpected expression %T", x)
	}
	return e, nil
}

// Parse tokenizes and parses text. It returns the tree for the whole input.
func (p *Parser) Parse(input string) (*Node, error) {
	tokens := p.lex.tokenize(input)
	p.tokens = tokens
	p.cats = make([]map[string]bool, len(tokens))
	for i, t := range tokens {
		p.cats[i] = p.lex.classify(t.Text)
	}
	p.nameMemo = make(map[nameKey]result)
	p.exprMemo = make(map[memoKey]result)
	p.inProgress = make(map[nameKey]bool)
	p.anything = make(map[int]result)
	p.maxPos = 0
	p.expected = make(map[string]bool)

	res := p.evalName("text", 0)
	if node, ok := res[len(tokens)]; ok {
		return node, nil
	}

	// Find the furthest point any production reached.
	pos := p.maxPos
	if pos > len(tokens) {
		pos = len(tokens)
	}
	word := ""
	if pos < len(tokens) {
		word = tokens[pos].Text
	}
	var expected []string
	for k := range p.expected {
		expected = append(expected, k)
	}
	sort.Strings(expected)
	if len(expected) > 12 {
		expected = expected[:12]
	}
	return nil, &Error{Token: pos, Word: word, Expected: expected}
}

// Tokenize exposes the tokenizer for tests and tools.
func (p *Parser) Tokenize(input string) []Token {
	toks := p.lex.tokenize(input)
	return toks
}

func (p *Parser) evalName(name string, pos int) result {
	if isLexical(name) {
		return p.evalLexical(name, pos)
	}
	prod, ok := p.prods[name]
	if !ok {
		return result{}
	}
	key := nameKey{name, pos}
	if r, ok := p.nameMemo[key]; ok {
		return r
	}
	// Guard against accidental cyclic nullable productions. The guard is
	// keyed by position as well as name, because a production may call
	// itself at a later position quite legitimately.
	if p.inProgress[key] {
		return result{}
	}
	p.inProgress[key] = true
	inner := p.evalExpr(prod.expr, pos)
	delete(p.inProgress, key)

	r := make(result, len(inner))
	for end, node := range inner {
		r[end] = &Node{Name: name, Kids: []*Node{node}, From: pos, To: end, Cost: nodeCost(node)}
	}
	// A free modifier may follow almost any construct. The reference parser
	// attaches indicators to the preceding word, which evalLexical already
	// does; this handles the free modifiers proper (vocatives, parentheses,
	// subscripts and so on).
	if name != "free" {
		for _, end := range sortedKeys(inner) {
			if !p.canStartFree(end) {
				continue
			}
			base := r[end]
			for fend, fnode := range p.evalName("free", end) {
				if fend == end {
					continue
				}
				cand := &Node{Name: name, Kids: []*Node{base.Kids[0], fnode}, From: pos, To: fend, Cost: base.Cost + nodeCost(fnode)}
				if old, ok := r[fend]; !ok || cand.Cost < old.Cost {
					r[fend] = cand
				}
			}
		}
	}
	p.nameMemo[key] = r
	return r
}

// canStartFree reports whether a free modifier can begin at pos.
func (p *Parser) canStartFree(pos int) bool {
	if pos >= len(p.tokens) {
		return false
	}
	c := p.cats[pos]
	switch {
	case c["cSEI"], c["cSOI"], c["cTO"], c["cXI"], c["cDOI"], c["cCOI"]:
		return true
	case c["cUI"], c["cCAI"], c["cY"], c["cDAhO"], c["cFUhO"]:
		return true
	case c["cPA"], c["cBY"]:
		return pos+1 < len(p.tokens) && p.cats[pos+1]["cMAI"]
	}
	return false
}

func nodeCost(n *Node) int {
	if n == nil {
		return 0
	}
	return n.Cost
}

func (p *Parser) evalLexical(name string, pos int) result {
	if pos >= len(p.tokens) {
		p.note(pos, name)
		return result{}
	}
	if _, ok := p.cats[pos][name]; !ok {
		p.note(pos, name)
		return result{}
	}
	// Indicators attach to the word before them, as in the reference
	// parser, so they may appear after almost any word.
	end := p.indicatorEnd(pos + 1)
	return result{end: {Name: name, Text: p.join(pos, end), From: pos, To: end}}
}

// join renders the words from pos to end as one string.
func (p *Parser) join(pos, end int) string {
	if end <= pos+1 {
		return p.tokens[pos].Text
	}
	var b strings.Builder
	for i := pos; i < end; i++ {
		if i > pos {
			b.WriteByte(' ')
		}
		b.WriteString(p.tokens[i].Text)
	}
	return b.String()
}

// indicatorEnd returns the position after any indicators that follow pos.
func (p *Parser) indicatorEnd(pos int) int {
	for pos < len(p.tokens) {
		c := p.cats[pos]
		switch {
		case c["cUI"] || c["cCAI"]:
			pos++
			if pos < len(p.tokens) && p.cats[pos]["cNAI"] {
				pos++
			}
		case c["cY"] || c["cDAhO"] || c["cFUhE"] || c["cFUhO"]:
			pos++
		default:
			return pos
		}
	}
	return pos
}

func (p *Parser) evalExpr(e *expr, pos int) result {
	if e == nil {
		return result{pos: nil}
	}
	key := memoKey{e.id, pos}
	if r, ok := p.exprMemo[key]; ok {
		return r
	}
	var r result
	switch e.kind {
	case kindEmpty:
		r = result{pos: nil}
	case kindToken:
		// Tokens are only used by lexical productions, which are matched as
		// whole words. A syntax production has no bare token.
		r = result{}
	case kindName:
		switch e.name {
		case "any_word":
			r = p.evalAnyWord(pos)
		case "anything":
			r = p.evalAnything(pos)
		default:
			r = p.evalName(e.name, pos)
		}
	case kindAlt:
		r = make(result)
		for _, k := range e.kids {
			for end, node := range p.evalExpr(k, pos) {
				if old, exists := r[end]; !exists || nodeCost(node) < nodeCost(old) {
					r[end] = node
				}
			}
		}
	case kindSeq:
		r = p.evalSeq(e.kids, pos)
	case kindGroup:
		if len(e.kids) == 1 {
			r = p.evalExpr(e.kids[0], pos)
		} else {
			r = result{pos: nil}
		}
	case kindOption:
		r = make(result)
		if len(e.kids) == 1 {
			maps.Copy(r, p.evalExpr(e.kids[0], pos))
		}
		if old, ok := r[pos]; !ok || nodeCost(old) > 0 {
			r[pos] = nil
		}
	case kindRep:
		r = p.evalRep(e.kids[0], pos)
	default:
		r = result{}
	}
	p.exprMemo[key] = r
	return r
}

func (p *Parser) evalSeq(kids []*expr, pos int) result {
	type path struct {
		end  int
		cost int
		kids []*Node
	}
	cur := []path{{end: pos}}
	for _, k := range kids {
		best := make(map[int]path)
		for _, st := range cur {
			sub := p.evalExpr(k, st.end)
			for _, end := range sortedKeys(sub) {
				nk := make([]*Node, len(st.kids), len(st.kids)+1)
				copy(nk, st.kids)
				nk = append(nk, sub[end])
				c := st.cost + nodeCost(sub[end])
				if old, ok := best[end]; !ok || c < old.cost {
					best[end] = path{end: end, cost: c, kids: nk}
				}
			}
		}
		cur = cur[:0]
		ends := make([]int, 0, len(best))
		for end := range best {
			ends = append(ends, end)
		}
		sort.Ints(ends)
		for _, end := range ends {
			cur = append(cur, best[end])
		}
		if len(cur) == 0 {
			break
		}
	}
	r := make(result, len(cur))
	for _, st := range cur {
		r[st.end] = &Node{Kids: st.kids, From: pos, To: st.end, Cost: st.cost}
	}
	return r
}

func (p *Parser) evalRep(body *expr, pos int) result {
	// Relax until every end position has its cheapest path. Zero-length
	// matches are ignored so that nullable bodies cannot loop forever.
	bestCost := map[int]int{pos: 0}
	bestKids := map[int][]*Node{pos: nil}
	changed := true
	for changed {
		changed = false
		for _, st := range sortedCostKeys(bestCost) {
			sub := p.evalExpr(body, st)
			for _, end := range sortedKeys(sub) {
				if end == st {
					continue
				}
				c := bestCost[st] + nodeCost(sub[end])
				if old, ok := bestCost[end]; !ok || c < old {
					nk := make([]*Node, len(bestKids[st]), len(bestKids[st])+1)
					copy(nk, bestKids[st])
					nk = append(nk, sub[end])
					bestCost[end] = c
					bestKids[end] = nk
					changed = true
				}
			}
		}
	}
	r := make(result, len(bestCost))
	for end, cost := range bestCost {
		r[end] = &Node{Kids: bestKids[end], From: pos, To: end, Cost: cost}
	}
	return r
}

func (p *Parser) evalAnyWord(pos int) result {
	if pos >= len(p.tokens) || len(p.cats[pos]) == 0 {
		p.note(pos, "any_word")
		return result{}
	}
	end := p.indicatorEnd(pos + 1)
	return result{end: {Name: "any_word", Text: p.join(pos, end), From: pos, To: end, Cost: anyWordCost}}
}

func (p *Parser) evalAnything(pos int) result {
	if r, ok := p.anything[pos]; ok {
		return r
	}
	r := make(result, len(p.tokens)+1-pos)
	for end := pos; end <= len(p.tokens); end++ {
		var b strings.Builder
		for i := pos; i < end; i++ {
			if i > pos {
				b.WriteByte(' ')
			}
			b.WriteString(p.tokens[i].Raw)
		}
		r[end] = &Node{Name: "anything", Text: b.String(), From: pos, To: end}
	}
	p.anything[pos] = r
	return r
}

func (p *Parser) note(pos int, name string) {
	if pos > p.maxPos {
		p.maxPos = pos
		p.expected = map[string]bool{name: true}
	} else if pos == p.maxPos {
		p.expected[name] = true
	}
}

func sortedKeys(m result) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func sortedCostKeys(m map[int]int) []int {
	keys := make([]int, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Ints(keys)
	return keys
}

func isLexical(name string) bool {
	switch name {
	case "consonant", "terminalvowel", "vowel", "letter", "anything", "any_word":
		return true
	}
	return len(name) > 1 && name[0] == 'c' && name[1] >= 'A' && name[1] <= 'Z'
}

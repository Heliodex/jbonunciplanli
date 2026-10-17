package lojbo

import (
	"strings"
	"testing"
)

func load(t *testing.T) *Parser {
	t.Helper()
	p, err := Load("../bynyfys pa")
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	return p
}

var validTexts = []string{
	"coi",
	"coi ro do",
	"mi klama le zarci",
	"la .djan. cu klama le zarci",
	".i mi klama le zarci .i do klama le zdani",
	"mi viska ko",
	"mi tavla bau la .lojban.",
	"lo nu mi klama cu cfari",
	"mi djica lo nu do klama",
	"mi na klama",
	"mi na'e klama",
	"mi pu ca'o klama",
	"li ci bi'e pi'i vo su'i mu du li paze",
	"mi prami lo prenu poi klama",
	"zoi gy. some arbitrary text .gy. cu valsi",
	"lu mi klama li'u cu jufra",
	"mi cusku lu mi klama li'u do",
	"ro da poi prenu cu klama",
	"mi klama lo zarci be le zdani",
	"mi cusku lo se du'u mi klama",
	"mi mutce leka tatpi",
	"la .alis. cu zvati la se manci tumla .i finti fa la .lu,is.karol.",
	"le tirxu be me'e zo .teris. cu klama le barda tcadu",
	"ni'o la berti brife jo'u la solri pu troci leka djuno",
	"mi klama coi",
	"mi coi klama",
	"mi klama to mi klama toi",
	"mi klama sei do klama",
	"mi klama .a'u",
	"mi klama fa do",
	"na'e zei .a zei na'e zei by. zei livgyterbilma",
	".i baku la berti brife co'a traji cupra le brife",
	"fa klama",
	"ni'o la berti brife jo'u la solri .i la berti brife jo'u la solri pu troci leka djuno ledu'u makau traji leka vlipa vau fo le'i me lenei .icabo le pa litru noi dasni lo glare kosta cu mo'u klama",
	".i lu'i le remei pu simxu leka tugni fi lenu traji leka vlipa fa le traji be leka clira fa lonu ce'u snada leka gasnu lenu le pa litru co'u dasni le kosta",
	"mi klama le zdani be do gi'e viska lo nu do zvati",
}

var invalidTexts = []string{
	"xyzzy",
	"mi klama le",
	"le zarci cu",
	"mi klama ku",
	"mi klama toi",
	"mi klama sei do",
	"cu mi klama",
}

func TestValid(t *testing.T) {
	p := load(t)
	for _, s := range validTexts {
		if _, err := p.Parse(s); err != nil {
			t.Errorf("Parse(%q) = %v, want success", s, err)
		}
	}
}

func TestInvalid(t *testing.T) {
	p := load(t)
	for _, s := range invalidTexts {
		if _, err := p.Parse(s); err == nil {
			t.Errorf("Parse(%q) = success, want failure", s)
		}
	}
}

func TestTokens(t *testing.T) {
	p := load(t)
	for in, want := range map[string][]string{
		"lonu mi klama":  {"lo", "nu", "mi", "klama"},
		"ledu'u":         {"le", "du'u"},
		"makau":          {"ma", "kau"},
		"by.by.":         {"by", "by"},
		".i lunu":        {"i", "lu", "nu"},
		"mi klama si":    {"mi"},
		"mi lonu si":     {"mi"},
		"mi goi fa'o do": {"mi", "goi"},
	} {
		toks := p.Tokenize(in)
		var got []string
		for _, tok := range toks {
			got = append(got, tok.Text)
		}
		if strings.Join(got, " ") != strings.Join(want, " ") {
			t.Errorf("Tokenize(%q) = %v, want %v", in, got, want)
		}
	}
}

func TestTree(t *testing.T) {
	p := load(t)
	node, err := p.Parse("mi klama")
	if err != nil {
		t.Fatal(err)
	}
	tree := node.String()
	for _, want := range []string{"text", "cKOhA mi", "cBRIVLA klama"} {
		if !strings.Contains(tree, want) {
			t.Errorf("tree does not contain %q:\n%s", want, tree)
		}
	}
	if strings.Contains(tree, "any_word") {
		t.Errorf("tree should use real categories, got:\n%s", tree)
	}
}

func TestParserReuse(t *testing.T) {
	p := load(t)
	for _, s := range []string{"mi klama", "do klama", "mi klama le zarci"} {
		if _, err := p.Parse(s); err != nil {
			t.Fatalf("Parse(%q) after reuse = %v", s, err)
		}
	}
}

func TestErrorPosition(t *testing.T) {
	p := load(t)
	_, err := p.Parse("mi klama le")
	if err == nil {
		t.Fatal("expected error")
	}
	perr, ok := err.(*Error)
	if !ok {
		t.Fatalf("error is %T, want *Error", err)
	}
	if perr.Token != 3 {
		t.Errorf("Token = %d, want 3", perr.Token)
	}
}

package syntax

import (
	"testing"

	"github.com/stretchr/testify/assert"

	"github.com/arr-ai/arrai/rel"
	"github.com/arr-ai/wbnf/parser"
)

func TestParseXMLTrivial(t *testing.T) {
	t.Parallel()
	assertParse(t, rel.NewXML([]rune("a"), []rel.Attr{}), "<a/>")
	assertParse(t, rel.NewXML([]rune("@b-c_1$.D"), []rel.Attr{}), "<@b-c_1$.D/>")
}

func TestParseXMLTrivialWithEndTag(t *testing.T) {
	t.Parallel()
	assertParse(t, rel.NewXML([]rune("a"), []rel.Attr{}), "<a></a>")
}

func TestParseXMLTrivialWithMismatchedEndTags(t *testing.T) {
	t.Parallel()
	value, err := Parse(parser.NewScanner("<a></ab>"), "")
	assert.Error(t, err, "%s", value)
}

func TestParseXMLNested(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), []rel.Attr{},
			rel.NewXML([]rune("b"), []rel.Attr{})),
		"<a><b/></a>")
}

func TestParseXML1Attr(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML(
			[]rune("a"),
			[]rel.Attr{{Name: "x", Value: rel.NewNumber(1)}},
		),
		`<a x=1/>`)
}

func TestParseXML2Attrs(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML(
			[]rune("abc"),
			[]rel.Attr{
				{Name: "x", Value: rel.NewNumber(1)},
				{Name: "yz", Value: rel.NewString([]rune("hello"))},
			}),
		`<abc x=1 yz="hello"/>`)
}

func TestParseXML1Data(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML(
			[]rune("abc"),
			[]rel.Attr{
				{Name: "x", Value: rel.NewNumber(1)},
				{Name: "yz", Value: rel.NewString([]rune("hello"))},
			}),
		`<abc x=1 yz="hello"/>`)
}

func TestParseXMLHtmlEntities(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), nil, rel.NewString([]rune("&"))),
		`<a>&amp;</a>`)
}

func TestParseXMLHtmlEntitiesEuroBug(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), nil, rel.NewString([]rune("€"))),
		`<a>&euro;</a>`)
}

var xmlSpacePreserve = rel.Attr{
	Name:  "{https://www.w3.org/XML/1998/namespace}space",
	Value: rel.NewString([]rune("preserve")),
}

// TODO: More edge-case coverage.
func TestParseTrimSpace(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), nil, rel.NewString([]rune("foo"))),
		`<a>
  foo
</a>`)
}

func TestParseXMLSpaceBadValue(t *testing.T) {
	t.Parallel()
	assertParseError(t, `<a xml:space="wrong"/>`)
}

func TestParseSpacePreserve(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), []rel.Attr{xmlSpacePreserve},
			rel.NewString([]rune("\n  foo\n"))),
		`<a xml:space="preserve">
  foo
</a>`)
}

func xmlnsDefault(ns string) rel.Attr {
	return rel.Attr{Name: "xmlns", Value: rel.NewString([]rune(ns))}
}

func xmlnsAlias(alias string, ns string) rel.Attr {
	return rel.Attr{
		Name:  "{http://www.w3.org/2000/xmlns/}" + alias,
		Value: rel.NewString([]rune(ns)),
	}
}

func TestParseXmlns(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML(
			[]rune("{my-ns}foobar"),
			[]rel.Attr{xmlnsAlias("me", "my-ns")},
		),
		`<me:foobar xmlns:me="my-ns"/>`)
}

func TestParseXmlnsDefault(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML(
			[]rune("{my-ns}foobar"),
			[]rel.Attr{xmlnsDefault("my-ns")},
		),
		`<foobar xmlns="my-ns"/>`)
}

func TestParseXmlnsDefaultInAlias(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("{my-ns}foobar"),
			[]rel.Attr{
				xmlnsDefault("def-ns"),
				xmlnsAlias("me", "my-ns"),
			},
			rel.NewXML([]rune("{def-ns}baz"), nil),
		),
		`<me:foobar xmlns="def-ns" xmlns:me="my-ns"><baz/></me:foobar>`)
}

func TestParseXmlnsAliasInDefault(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("{def-ns}foobar"),
			[]rel.Attr{
				xmlnsDefault("def-ns"),
				xmlnsAlias("me", "my-ns"),
			},
			rel.NewXML([]rune("{my-ns}baz"), nil),
		),
		`<foobar xmlns="def-ns" xmlns:me="my-ns"><me:baz/></foobar>`)
}

func TestParseXmlExprInsideElt(t *testing.T) {
	t.Parallel()
	assertParse(t,
		rel.NewXML([]rune("a"), []rel.Attr{},
			rel.NewNumber(1)),
		`<a>{1}</a>`)
}

// TODO: Fix
// func TestParseXmlDotExprAttr(t *testing.T) {
// 	t.Parallel()
// 	assertParse(t,
// 		rel.NewXML([]rune("a"),
// 			[]rel.Attr{
// 				{Name: "attr", Value: rel.NewNumber(42)},
// 			}),
// 		`<a attr=.foo/>`,
// 	)
// }

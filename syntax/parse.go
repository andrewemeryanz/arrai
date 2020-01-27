package syntax

import (
	"fmt"
	"path"
	"strconv"
	"strings"

	"github.com/arr-ai/arrai/rel"
	"github.com/arr-ai/wbnf/ast"
	"github.com/arr-ai/wbnf/parser"
	"github.com/arr-ai/wbnf/wbnf"
)

type noParseType struct{}

type parseFunc func(v interface{}) (rel.Expr, error)

func (*noParseType) Error() string {
	return "No parse"
}

var noParse = &noParseType{}

var arraiParsers = wbnf.MustCompile(`
expr    -> amp="&"* @ arrow=(nest | unnest | ARROW @)*
         > @:binop=("with" | "without")
         > @:binop="||"
         > @:binop="&&"
         > @:binop=/{!?(?:<>?=?|>=?|=)}
         > @ if=("if" t=expr "else" f=expr)*
         > @:binop=/{[-+|]}
         > @:binop=/{&|[-<][-&][->]}
         > @:binop=/{[\*/%]|-%|//}
         > @:rbinop="**"
		 > unop=/{[-+!]|\*\*?}* @
		 > @ call=("(" arg=expr:",", ")")*
         > @ "count"? touch?
         > dot+ | @ dot*
         > "{" rel=(names tuple=("(" v=@:",", ")"):",",?) "}"
         | "{" set=(elt=@:",",?) "}"
         | "[" array=(item=@:",",?) "]"
         | "{:" embed=(grammar=@ "." rule=IDENT subgrammar=()) ":}"
         | op="\\\\" @
         | fn="\\" IDENT @
         | "//" pkg=( "." ("/" local=IDENT)+
                    | "." std=IDENT?
                    | http="http://"? fqdn=IDENT:"." ("/" path=IDENT)*
                    )
         | "(" tuple=(k=IDENT ":" v=@ | ":" vk=(@ "." k=IDENT)):",",? ")"
         | "(" @ ")"
         | IDENT | STR | NUM;
nest    -> "nest" names IDENT;
unnest  -> "unnest" IDENT;
touch   -> ("->*" ("&"? IDENT | STR))+ "(" expr:"," ","? ")";
dot     -> dot="." ("&"? IDENT | STR | "*");
names   -> "|" IDENT:"," "|";

ARROW  -> /{->|:>|=>|>>|order|where|sum|max|mean|median|min};
IDENT  -> /{ [$@A-Za-z_][0-9$@A-Za-z_]* | ' (?: \\. | [^\\'] )* ' };
STR    -> /{ " (?: \\. | [^\\"] )* " };
NUM    -> /{(?: [0-9]+(?:\.[0-9]*)? | \.[0-9]+ ) (?: [Ee][-+]?[0-9]+ )?};

.wrapRE -> /{\s*()};
`)

type parse struct {
	sourceDir string
}

func (p *parse) parseExpr(b ast.Branch) rel.Expr {
	// log.Printf("b=%v", b)
	name, c := which(b,
		"amp", "arrow", "unop", "binop", "rbinop",
		"if", "call", "touch", "dot",
		"rel", "set", "array", "embed", "op", "fn", "pkg", "tuple",
		"IDENT", "STR", "NUM",
		"expr",
	)
	if c == nil {
		panic(fmt.Errorf("misshapen node AST: %v", b))
	}
	switch name {
	case "amp", "arrow":
		expr := p.parseExpr(b["expr"].(ast.One).Node.(ast.Branch))
		if arrows, has := b["arrow"]; has {
			for _, arrow := range arrows.(ast.Many) {
				part, d := which(arrow.(ast.Branch), "nest", "unnest", "ARROW")
				switch part {
				case "nest":
				case "unnset":
					panic("unfinished")
				case "ARROW":
					f := binops[d.(ast.One).Node.(ast.Leaf).Scanner().String()]
					expr = f(expr, p.parseExpr(arrow.(ast.Branch)["expr"].(ast.One).Node.(ast.Branch)))
				}
			}
		}
		if name == "amp" {
			for range c.(ast.Many) {
				expr = rel.NewFunction("-", expr)
			}
		}
		return expr
	case "unop":
		ops := c.(ast.Many)
		result := p.parseExpr(b.MustOne("expr").(ast.Branch))
		for i := len(ops) - 1; i >= 0; i-- {
			f := unops[ops[i].(ast.Leaf).String()]
			result = f(result)
		}
		return result
	case "binop":
		ops := c.(ast.Many)
		args := b["expr"].(ast.Many)
		result := p.parseExpr(args[0].(ast.Branch))
		for i, arg := range args[1:] {
			f := binops[ops[i].(ast.Leaf).Scanner().String()]
			result = f(result, p.parseExpr(arg.(ast.Branch)))
		}
		return result
	case "rbinop":
		ops := c.(ast.Many)
		args := b["expr"].(ast.Many)
		result := p.parseExpr(args[len(args)-1].(ast.Branch))
		for i := len(args) - 2; i >= 0; i-- {
			f := binops[ops[i].(ast.Leaf).String()]
			result = f(p.parseExpr(args[i].(ast.Branch)), result)
		}
		return result
	case "if":
		result := p.parseExpr(b.MustOne("expr").(ast.Branch))
		for _, ifelse := range c.(ast.Many) {
			t := p.parseExpr(ifelse.MustOne("t").(ast.Branch))
			f := p.parseExpr(ifelse.MustOne("f").(ast.Branch))
			result = rel.NewIfElseExpr(result, t, f)
		}
		return result
	case "call":
		result := p.parseExpr(b.MustOne("expr").(ast.Branch))
		for _, call := range c.(ast.Many) {
			for _, arg := range p.parseExprs(call.MustMany("arg")...) {
				result = rel.NewCallExpr(result, arg)
			}
		}
		return result
	case "touch":
		// touch -> ("->*" ("&"? IDENT | STR))+ "(" expr:"," ","? ")";
		// result := p.parseExpr(b.MustOne("expr").(ast.Branch))

	case "dot":
		result := p.parseExpr(b.MustOne("expr").(ast.Branch))
		if result == nil {
			result = rel.DotIdent
		}
		for _, dot := range c.(ast.Many) {
			ident := dot.(ast.Branch)["IDENT"].(ast.One).Node.(ast.Leaf).Scanner().String()
			result = rel.NewDotExpr(result, ident)
		}
		return result
	case "rel":
		names := parseNames(c.(ast.One).Node.(ast.Branch)["names"].(ast.One).Node.(ast.Branch))
		tuples := c.(ast.One).Node.(ast.Branch)["tuple"].(ast.Many)
		tupleExprs := make([][]rel.Expr, 0, len(tuples))
		for _, tuple := range tuples {
			tupleExprs = append(tupleExprs, p.parseExprs(tuple.(ast.Branch)["v"].(ast.Many)...))
		}
		result, err := rel.NewRelationExpr(names, tupleExprs...)
		if err != nil {
			panic(err)
		}
		return result
	case "set":
		return rel.NewSetExpr(p.parseExprs(c.(ast.One).Node.(ast.Branch)["elt"].(ast.Many)...)...)
	case "fn":
		ident := b.MustOne("IDENT")
		expr := p.parseExpr(b.MustOne("expr").(ast.Branch))
		return rel.NewFunction(ident.Scanner().String(), expr)
	case "pkg":
		pkg := c.(ast.One).Node.(ast.Branch)
		if std, has := pkg["std"]; has {
			pkgName := std.(ast.One).Node.(ast.Leaf).Scanner().String()
			return NewPackageExpr(rel.NewDotExpr(rel.DotIdent, pkgName))
		} else if local := pkg["local"]; local != nil {
			var sb strings.Builder
			for i, part := range local.(ast.Many) {
				if i > 0 {
					sb.WriteRune('/')
				}
				sb.WriteString(strings.Trim(part.Scanner().String(), "'"))
			}
			filepath := sb.String()
			if p.sourceDir == "" {
				panic(fmt.Errorf("local import %q invalid; no local context", filepath))
			}
			return rel.NewCallExpr(
				NewPackageExpr(rel.NewIdentExpr("//./")),
				rel.NewString([]rune(path.Join(p.sourceDir, filepath))))
		} else if fqdn := pkg["fqdn"]; fqdn != nil {
			var sb strings.Builder
			if http := pkg["http"]; http != nil {
				sb.WriteString(http.(ast.One).Node.(ast.Leaf).Scanner().String())
			}
			for i, part := range fqdn.(ast.Many) {
				if i > 0 {
					sb.WriteRune('.')
				}
				sb.WriteString(strings.Trim(part.Scanner().String(), "'"))
			}
			if path := pkg["path"]; path != nil {
				for _, part := range path.(ast.Many) {
					sb.WriteRune('/')
					sb.WriteString(strings.Trim(part.Scanner().String(), "'"))
				}
			}
			return rel.NewCallExpr(NewPackageExpr(rel.NewIdentExpr("//")), rel.NewString([]rune(sb.String())))
		} else {
			return NewPackageExpr(rel.DotIdent)
		}
	case "tuple":
		entries := c.(ast.Many)
		attrs := make([]rel.AttrExpr, 0, len(entries))
		for _, entry := range entries {
			k := entry.MustOne("k").(ast.Leaf).Scanner().String()
			v := p.parseExpr(entry.MustOne("v").(ast.Branch))
			attr, err := rel.NewAttrExpr(k, v)
			if err != nil {
				panic(err)
			}
			attrs = append(attrs, attr)
		}
		return rel.NewTupleExpr(attrs...)
	case "IDENT":
		s := c.(ast.One).Scanner().String()
		switch s {
		case "true":
			return rel.True
		case "false":
			return rel.False
		}
		return rel.NewIdentExpr(s)
	case "STR":
		s := c.Scanner().String()
		return rel.NewString([]rune(parseArraiString(s)))
	case "NUM":
		s := c.Scanner().String()
		n, err := strconv.ParseFloat(s, 64)
		if err != nil {
			panic("Wat?")
		}
		return rel.NewNumber(n)
	case "expr":
		return p.parseExpr(c.(ast.One).Node.(ast.Branch))
	}
	panic(fmt.Errorf("unhandled node: %v", b))
}

func (p *parse) parseExprs(exprs ...ast.Node) []rel.Expr {
	result := make([]rel.Expr, 0, len(exprs))
	for _, expr := range exprs {
		result = append(result, p.parseExpr(expr.(ast.Branch)))
	}
	return result
}

func parseNames(names ast.Branch) []string {
	idents := names["IDENT"].(ast.Many)
	result := make([]string, 0, len(idents))
	for _, ident := range idents {
		result = append(result, ident.(ast.Leaf).Scanner().String())
	}
	return result
}

// MustParseString parses input string and returns the parsed Expr or panics.
func MustParseString(s, sourceDir string) rel.Expr {
	return MustParse(parser.NewScanner(s), sourceDir)
}

// MustParse parses input and returns the parsed Expr or panics.
func MustParse(s *parser.Scanner, sourceDir string) rel.Expr {
	expr, err := Parse(s, sourceDir)
	if err != nil {
		panic(err)
	}
	return expr
}

// ParseString parses input string and returns the parsed Expr or an error.
func ParseString(s, sourceDir string) (rel.Expr, error) {
	return Parse(parser.NewScanner(s), sourceDir)
}

// Parse parses input and returns the parsed Expr or an error.
func Parse(s *parser.Scanner, sourceDir string) (rel.Expr, error) {
	v, err := arraiParsers.Parse(wbnf.Rule("expr"), s)
	if err != nil {
		return nil, err
	}
	if s.String() != "" {
		return nil, fmt.Errorf("input not consumed: %v", s)
	}
	// log.Print("v=", v)
	ast := ast.ParserNodeToNode(arraiParsers.Grammar(), v)
	return (&parse{sourceDir: sourceDir}).parseExpr(ast), nil
}
func which(b ast.Branch, names ...string) (string, ast.Children) {
	if len(names) == 0 {
		panic("wat?")
	}
	for _, name := range names {
		if children, has := b[name]; has {
			return name, children
		}
	}
	return "", nil
}

var unops = map[string]newUnOpFunc{
	"+":  rel.NewPosExpr,
	"-":  rel.NewNegExpr,
	"**": rel.NewPowerSetExpr,
	"!":  rel.NewNotExpr,
	"*":  rel.NewEvalExpr,
	"//": NewPackageExpr,
}

var binops = map[string]newBinOpFunc{
	"->":      rel.NewArrowExpr,
	">>":      rel.NewAngleArrowExpr,
	"=>":      rel.NewDArrowExpr,
	"order":   rel.NewOrderExpr,
	"where":   rel.NewWhereExpr,
	"sum":     rel.NewSumExpr,
	"max":     rel.NewMaxExpr,
	"mean":    rel.NewMeanExpr,
	"median":  rel.NewMedianExpr,
	"min":     rel.NewMinExpr,
	"with":    rel.NewWithExpr,
	"without": rel.NewWithoutExpr,
	"&&": rel.MakeBinValExpr("and", func(a, b rel.Value) rel.Value {
		if !a.Bool() {
			return a
		}
		return b
	}),
	"||": rel.MakeBinValExpr("and", func(a, b rel.Value) rel.Value {
		if a.Bool() {
			return a
		}
		return b
	}),
	"=":   rel.MakeEqExpr("=", func(a, b rel.Value) bool { return a.Equal(b) }),
	"<":   rel.MakeEqExpr("<", func(a, b rel.Value) bool { return a.Less(b) }),
	">":   rel.MakeEqExpr(">", func(a, b rel.Value) bool { return b.Less(a) }),
	"!=":  rel.MakeEqExpr("!=", func(a, b rel.Value) bool { return !a.Equal(b) }),
	"<=":  rel.MakeEqExpr("<=", func(a, b rel.Value) bool { return !b.Less(a) }),
	">=":  rel.MakeEqExpr(">=", func(a, b rel.Value) bool { return !a.Less(b) }),
	"+":   rel.NewAddExpr,
	"-":   rel.NewSubExpr,
	"<&>": rel.NewJoinExpr,
	"*":   rel.NewMulExpr,
	"/":   rel.NewDivExpr,
	"%":   rel.NewModExpr,
	"-%":  rel.NewSubModExpr,
	"//":  rel.NewIdivExpr,
	"**":  rel.NewPowExpr,
}

func parseTouchTail(v interface{}, expr rel.Expr) (rel.Expr, error) {
	panic("not implemented")
	// path := make([]string, 0, 4) // A bit of spare buffer
	// for v.Scan(ARROWST) {
	// 	if !v.Scan(IDENT, Token('&'), STR) {
	// 		return nil, expecting(v, "after '.'", "ident", "string", "'*'")
	// 	}
	// 	if v.Token() == STR {
	// 		path = append(path, ParseArraiString(v.Lexeme()))
	// 	} else if v.Token() == Token('&') {
	// 		if !v.Scan(IDENT) {
	// 			return nil, expecting(v, "after '.&'-prefix", "ident")
	// 		}
	// 		path = append(path, "&"+string(v.Lexeme()))
	// 	} else {
	// 		path = append(path, string(v.Lexeme()))
	// 	}
	// }
	// if len(path) == 0 {
	// 	return expr, nil
	// }
	// leaf := path[len(path)-1]
	// attrExpr, err := parseAttrExpr(v, leaf)
	// tupleExpr := makeTupleExpr(leaf, attrExpr)
	// if err != nil {
	// 	return nil, err
	// }
	// for i := len(path) - 2; i >= 0; i-- {
	// 	tupleFunc := rel.NewFunction(".", tupleExpr)
	// 	dotExpr := rel.NewDotExpr(rel.DotIdent, path[i])
	// 	arrowExpr := rel.NewArrowExpr(dotExpr, tupleFunc)
	// 	tupleExpr = makeTupleExpr(path[i], arrowExpr)
	// 	if err != nil {
	// 		panic(err)
	// 	}
	// }
	// return rel.NewArrowExpr(expr, rel.NewFunction(".", tupleExpr)), nil
}

func parseDotTail(v interface{}, expr rel.Expr) (rel.Expr, error) {
	panic("not implemented")
	// for v.Scan(Token('.')) {
	// 	if !v.Scan(IDENT, Token('&'), STR, Token('*')) {
	// 		return nil, expecting(v, "after '.'", "ident", "string", "'*'")
	// 	}
	// 	if v.Token() == STR {
	// 		expr = rel.NewDotExpr(expr, ParseArraiString(v.Lexeme()))
	// 	} else if v.Token() == Token('&') {
	// 		if !v.Scan(IDENT) {
	// 			return nil, expecting(v, "after '.&'-prefix", "ident")
	// 		}
	// 		expr = rel.NewDotExpr(expr, "&"+string(v.Lexeme()))
	// 	} else {
	// 		expr = rel.NewDotExpr(expr, string(v.Lexeme()))
	// 	}
	// }
	// return expr, nil
}

func parseTuple(ast ast.Branch) rel.Expr {
	panic("not implemented")
	// 	if !v.Scan(Token('(')) {
	// 		return nil, expecting(v, "tuple start", "'('")
	// 	}
	// 	attrs := []rel.AttrExpr{}
	// tupleLoop:
	// 	for {
	// 		var name string
	// 		switch v.Peek() {
	// 		case Token(')'):
	// 			break tupleLoop
	// 		case STR:
	// 			v.Scan()
	// 			name = ParseArraiString(v.Lexeme())
	// 		case IDENT:
	// 			v.Scan()
	// 			name = string(v.Lexeme())
	// 		case Token('&'):
	// 			v.Scan()
	// 			if !v.Scan(IDENT) {
	// 				return nil, expecting(v, "after '&'-prefix", "ident")
	// 			}
	// 			name = "&" + string(v.Lexeme())
	// 		}
	// 		expr, err := parseAttrExpr(v, name)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		attr, err := rel.NewAttrExpr(name, expr)
	// 		if err != nil {
	// 			return nil, err
	// 		}
	// 		attrs = append(attrs, attr)
	// 		if !v.Scan(Token(',')) {
	// 			break
	// 		}
	// 	}
	// 	if !v.Scan(Token(')')) {
	// 		return nil, expecting(v, "after tuple body", "')'")
	// 	}
	// 	return rel.NewTupleExpr(attrs...), nil
}

func parseAttrExpr(v interface{}, name string) (rel.Expr, error) {
	panic("not implemented")
	// if name != "" && !v.Scan(Token(':')) {
	// 	return nil, expecting(v, "after tuple name", "':'")
	// }
	// expr, err := parseExpr(ast)
	// if err != nil {
	// 	return nil, err
	// }
	// if name == "" {
	// 	e := expr
	// 	for b, ok := e.(rel.LHSExpr); ok; b, ok = e.(rel.LHSExpr) {
	// 		e = b.LHS()
	// 	}
	// 	if dot, ok := e.(*rel.DotExpr); ok {
	// 		name = dot.Attr()
	// 	} else {
	// 		return nil, expecting(
	// 			v, "after omitted attr ident", "expr with ident LHS")
	// 	}
	// }
	// if name[:1] == "&" {
	// 	expr = rel.NewFunction("-", expr)
	// }
	// return expr, nil
}

type newBinOpFunc func(a, b rel.Expr) rel.Expr
type newUnOpFunc func(e rel.Expr) rel.Expr

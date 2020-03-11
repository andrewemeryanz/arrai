package rel

import (
	"testing"

	"github.com/arr-ai/wbnf/ast"

	"github.com/arr-ai/wbnf/parser"
	"github.com/arr-ai/wbnf/wbnf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func assertASTNodeToValueToNode(t *testing.T, p parser.Parsers, rule, src string) bool { //nolint:unparam
	v, err := p.Parse(parser.Rule(rule), parser.NewScanner(src))
	assert.NoError(t, err)
	ast1 := ast.FromParserNode(p.Grammar(), v)
	value := ASTBranchToValue(ast1)
	ast2 := ASTBranchFromValue(value)
	return assert.True(t, ast1.ContentEquals(ast2), "expected: %v\nactual:   %v", ast1, ast2)
}

func TestNodeToValueSimple(t *testing.T) {
	assertASTNodeToValueToNode(t, wbnf.Core(), "grammar", `expr -> "+"|"*";`)
}

func TestGrammarToValueExpr(t *testing.T) {
	assertASTNodeToValueToNode(t, wbnf.Core(), "grammar", `x->@:"+" > @:"*" > "1";`)
}

func TestNodeToValueExpr(t *testing.T) {
	grammar := `expr -> @:op="+" > @:op="*" > n=[0-9];`

	exprP, err := wbnf.Compile(grammar, nil)
	require.NoError(t, err)
	assertASTNodeToValueToNode(t, exprP, "expr", `1+2*3`)
}

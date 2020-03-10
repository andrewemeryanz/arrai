package rel

import (
	"bytes"
)

// ArrayExpr...
type ArrayExpr struct {
	elements []Expr
}

// NewArrayExpr returns a new Expr that constructs an Array.
func NewArrayExpr(elements ...Expr) Expr {
	values := make([]Value, 0, len(elements))
	for _, expr := range elements {
		if value, ok := expr.(Value); ok {
			values = append(values, value)
			continue
		}
		return ArrayExpr{elements: elements}
	}
	return NewArray(values...)
}

// Elements returns a Set's elements.
func (e ArrayExpr) Elements() []Expr {
	panic("unfinished")
}

// String returns a string representation of the expression.
func (e ArrayExpr) String() string {
	var b bytes.Buffer
	b.WriteByte('[')
	i := 0
	for _, expr := range e.elements {
		if i > 0 {
			b.WriteString(", ")
		}
		b.WriteString(expr.String())
		i++
	}
	b.WriteByte(']')
	return b.String()
}

// Eval returns the subject
func (e ArrayExpr) Eval(local Scope) (Value, error) {
	values := make([]Value, 0, len(e.elements))
	for _, expr := range e.elements {
		value, err := expr.Eval(local)
		if err != nil {
			return nil, err
		}
		values = append(values, value)
	}
	return NewArray(values...), nil
}

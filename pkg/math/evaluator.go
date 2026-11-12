package math

import (
	"fmt"
	"math"
	"strconv"
	"strings"
	"unicode"
)

// TokenType defines the type of lexical token.
type TokenType int

const (
	TokenNumber TokenType = iota
	TokenOperator
	TokenFunction
	TokenLParen
	TokenRParen
	TokenPercent
	TokenEOF
)

// Token represents a lexical token in the expression.
type Token struct {
	Type  TokenType
	Value string
	Pos   int
}

// Evaluator evaluates safe arithmetic expressions without external calls.
type Evaluator struct{}

// NewEvaluator creates a new mathematical evaluator.
func NewEvaluator() *Evaluator {
	return &Evaluator{}
}

// Evaluate takes an arithmetic string and returns the evaluated result or an error message.
func (e *Evaluator) Evaluate(input string) string {
	clean := strings.TrimSpace(input)
	if clean == "" {
		return ""
	}

	tokens, err := tokenize(clean)
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if len(tokens) == 0 {
		return ""
	}

	p := &parser{tokens: tokens, pos: 0}
	val, err := p.parseExpression()
	if err != nil {
		return fmt.Sprintf("Error: %v", err)
	}

	if p.pos < len(p.tokens) && p.tokens[p.pos].Type != TokenEOF {
		return "Error: Unexpected token after expression"
	}

	if math.IsNaN(val) {
		return "Error: Not a number (NaN)"
	}
	if math.IsInf(val, 0) {
		return "Error: Division by zero"
	}

	return formatResult(val)
}

func tokenize(input string) ([]Token, error) {
	var tokens []Token
	runes := []rune(input)
	n := len(runes)
	i := 0

	for i < n {
		r := runes[i]

		// Skip whitespace
		if unicode.IsSpace(r) {
			i++
			continue
		}

		// Numbers (digits and decimal point)
		if unicode.IsDigit(r) || (r == '.' && i+1 < n && unicode.IsDigit(runes[i+1])) {
			start := i
			hasDot := false
			for i < n && (unicode.IsDigit(runes[i]) || runes[i] == '.') {
				if runes[i] == '.' {
					if hasDot {
						return nil, fmt.Errorf("multiple decimal points in number at pos %d", i)
					}
					hasDot = true
				}
				i++
			}
			tokens = append(tokens, Token{Type: TokenNumber, Value: string(runes[start:i]), Pos: start})
			continue
		}

		// Identifiers (functions or constants or mod)
		if unicode.IsLetter(r) {
			start := i
			for i < n && (unicode.IsLetter(runes[i]) || unicode.IsDigit(runes[i])) {
				i++
			}
			name := strings.ToLower(string(runes[start:i]))
			if name == "mod" {
				tokens = append(tokens, Token{Type: TokenOperator, Value: "mod", Pos: start})
			} else if name == "pi" {
				tokens = append(tokens, Token{Type: TokenNumber, Value: fmt.Sprintf("%g", math.Pi), Pos: start})
			} else if name == "e" {
				tokens = append(tokens, Token{Type: TokenNumber, Value: fmt.Sprintf("%g", math.E), Pos: start})
			} else {
				tokens = append(tokens, Token{Type: TokenFunction, Value: name, Pos: start})
			}
			continue
		}

		// Parentheses
		if r == '(' {
			tokens = append(tokens, Token{Type: TokenLParen, Value: "(", Pos: i})
			i++
			continue
		}
		if r == ')' {
			tokens = append(tokens, Token{Type: TokenRParen, Value: ")", Pos: i})
			i++
			continue
		}

		// Percentage
		if r == '%' {
			tokens = append(tokens, Token{Type: TokenPercent, Value: "%", Pos: i})
			i++
			continue
		}

		// Operators
		if r == '+' || r == '-' || r == '*' || r == '/' || r == '^' {
			tokens = append(tokens, Token{Type: TokenOperator, Value: string(r), Pos: i})
			i++
			continue
		}

		return nil, fmt.Errorf("unrecognized character '%c' at position %d", r, i)
	}

	tokens = append(tokens, Token{Type: TokenEOF, Value: "", Pos: n})
	return tokens, nil
}

type parser struct {
	tokens []Token
	pos    int
}

func (p *parser) peek() Token {
	if p.pos >= len(p.tokens) {
		return Token{Type: TokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *parser) advance() Token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

type termResult struct {
	val       float64
	isPercent bool
}

// parseExpression handles addition and subtraction (lowest precedence)
// Also handles relative percentage: X + Y% -> X + (X * Y / 100)
func (p *parser) parseExpression() (float64, error) {
	res, err := p.parseExpressionResult()
	if err != nil {
		return 0, err
	}
	return res.val, nil
}

func (p *parser) parseExpressionResult() (termResult, error) {
	left, err := p.parseTerm()
	if err != nil {
		return termResult{}, err
	}

	for {
		tok := p.peek()
		if tok.Type == TokenOperator && (tok.Value == "+" || tok.Value == "-") {
			op := p.advance().Value
			right, err := p.parseTerm()
			if err != nil {
				return termResult{}, err
			}

			if right.isPercent {
				if op == "+" {
					left.val += left.val * right.val
				} else {
					left.val -= left.val * right.val
				}
			} else {
				if op == "+" {
					left.val += right.val
				} else {
					left.val -= right.val
				}
			}
			left.isPercent = false
		} else {
			break
		}
	}

	return left, nil
}

// parseTerm handles multiplication, division, modulo
func (p *parser) parseTerm() (termResult, error) {
	left, err := p.parsePower()
	if err != nil {
		return termResult{}, err
	}

	for {
		tok := p.peek()
		if tok.Type == TokenOperator && (tok.Value == "*" || tok.Value == "/" || tok.Value == "mod") {
			op := p.advance().Value
			right, err := p.parsePower()
			if err != nil {
				return termResult{}, err
			}

			if op == "*" {
				left.val *= right.val
			} else if op == "/" {
				if right.val == 0 {
					return termResult{}, fmt.Errorf("division by zero")
				}
				left.val /= right.val
			} else if op == "mod" {
				if right.val == 0 {
					return termResult{}, fmt.Errorf("modulo by zero")
				}
				left.val = math.Mod(left.val, right.val)
			}
			left.isPercent = false
		} else {
			break
		}
	}

	return left, nil
}

// parsePower handles exponentiation ^ (right associative)
func (p *parser) parsePower() (termResult, error) {
	left, err := p.parseFactor()
	if err != nil {
		return termResult{}, err
	}

	if p.peek().Type == TokenOperator && p.peek().Value == "^" {
		p.advance()
		right, err := p.parsePower()
		if err != nil {
			return termResult{}, err
		}
		return termResult{val: math.Pow(left.val, right.val), isPercent: false}, nil
	}

	return left, nil
}

// parseFactor handles unary operators, numbers, parentheses, functions, and percentages
func (p *parser) parseFactor() (termResult, error) {
	tok := p.peek()

	// Unary operators
	if tok.Type == TokenOperator && tok.Value == "+" {
		p.advance()
		return p.parseFactor()
	}
	if tok.Type == TokenOperator && tok.Value == "-" {
		p.advance()
		res, err := p.parseFactor()
		if err != nil {
			return termResult{}, err
		}
		res.val = -res.val
		return res, nil
	}

	// Functions: sqrt, abs, round, sin, cos, tan, log
	if tok.Type == TokenFunction {
		fnName := p.advance().Value
		if p.peek().Type != TokenLParen {
			return termResult{}, fmt.Errorf("expected '(' after function %s", fnName)
		}
		p.advance() // consume '('
		arg, err := p.parseExpression()
		if err != nil {
			return termResult{}, err
		}
		if p.peek().Type != TokenRParen {
			return termResult{}, fmt.Errorf("missing ')' after function argument")
		}
		p.advance() // consume ')'

		var res float64
		switch fnName {
		case "sqrt":
			if arg < 0 {
				return termResult{}, fmt.Errorf("cannot calculate square root of negative number")
			}
			res = math.Sqrt(arg)
		case "abs":
			res = math.Abs(arg)
		case "round":
			res = math.Round(arg)
		case "floor":
			res = math.Floor(arg)
		case "ceil":
			res = math.Ceil(arg)
		case "sin":
			res = math.Sin(arg)
		case "cos":
			res = math.Cos(arg)
		case "tan":
			res = math.Tan(arg)
		case "log", "ln":
			if arg <= 0 {
				return termResult{}, fmt.Errorf("logarithm of non-positive number")
			}
			res = math.Log(arg)
		case "log10":
			if arg <= 0 {
				return termResult{}, fmt.Errorf("logarithm of non-positive number")
			}
			res = math.Log10(arg)
		default:
			return termResult{}, fmt.Errorf("unknown function '%s'", fnName)
		}

		isPct := false
		if p.peek().Type == TokenPercent {
			p.advance()
			res = res / 100.0
			isPct = true
		}
		return termResult{val: res, isPercent: isPct}, nil
	}

	// Parenthesized expression
	if tok.Type == TokenLParen {
		p.advance()
		val, err := p.parseExpression()
		if err != nil {
			return termResult{}, err
		}
		if p.peek().Type != TokenRParen {
			return termResult{}, fmt.Errorf("unmatched opening parenthesis")
		}
		p.advance()

		isPct := false
		if p.peek().Type == TokenPercent {
			p.advance()
			val = val / 100.0
			isPct = true
		}
		return termResult{val: val, isPercent: isPct}, nil
	}

	// Numbers
	if tok.Type == TokenNumber {
		p.advance()
		val, err := strconv.ParseFloat(tok.Value, 64)
		if err != nil {
			return termResult{}, fmt.Errorf("invalid number format: %s", tok.Value)
		}

		isPct := false
		if p.peek().Type == TokenPercent {
			p.advance()
			val = val / 100.0
			isPct = true
		}
		return termResult{val: val, isPercent: isPct}, nil
	}

	return termResult{}, fmt.Errorf("syntax error near '%s'", tok.Value)
}

func formatResult(val float64) string {
	// If it is an integer within safe range
	if val == math.Trunc(val) && !math.IsNaN(val) && !math.IsInf(val, 0) && math.Abs(val) < 1e15 {
		return strconv.FormatInt(int64(val), 10)
	}

	// Format float with up to 8 decimal places and remove trailing zeros
	s := strconv.FormatFloat(val, 'f', 8, 64)
	s = strings.TrimRight(s, "0")
	s = strings.TrimRight(s, ".")
	return s
}

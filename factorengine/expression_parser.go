package factorengine

import (
	"fmt"
	"strconv"
	"strings"
	"unicode"
)

type tokenType int

const (
	tokenEOF tokenType = iota
	tokenIdentifier
	tokenNumber
	tokenString
	tokenBool
	tokenNull
	tokenLParen
	tokenRParen
	tokenLBracket
	tokenRBracket
	tokenLBrace
	tokenRBrace
	tokenComma
	tokenColon
	tokenQuestion
	tokenOperator
)

type token struct {
	typ tokenType
	raw string
}

type expressionParser struct {
	tokens []token
	pos    int
}

func ParseExpression(raw string) (Expr, error) {
	tokens, err := tokenizeExpression(raw)
	if err != nil {
		return nil, err
	}
	p := &expressionParser{tokens: tokens}
	expr, err := p.parseExpression(0)
	if err != nil {
		return nil, err
	}
	if p.peek().typ != tokenEOF {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unexpected token %q", p.peek().raw)}
	}
	return expr, nil
}

var operatorPrecedence = map[string]int{
	"||": 1,
	"&&": 2,
	"==": 3,
	"!=": 3,
	"<":  4,
	"<=": 4,
	">":  4,
	">=": 4,
	"+":  5,
	"-":  5,
	"*":  6,
	"/":  6,
}

func (p *expressionParser) parseExpression(minPrec int) (Expr, error) {
	left, err := p.parsePrefix()
	if err != nil {
		return nil, err
	}
	for {
		next := p.peek()
		if next.typ == tokenQuestion {
			if minPrec > 0 {
				break
			}
			p.consume()
			thenExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			if p.consume().typ != tokenColon {
				return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing ternary colon"}
			}
			elseExpr, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			left = ConditionalExpr{Cond: left, Then: thenExpr, Else: elseExpr}
			continue
		}
		if next.typ != tokenOperator {
			break
		}
		prec, ok := operatorPrecedence[next.raw]
		if !ok || prec < minPrec {
			break
		}
		op := p.consume().raw
		right, err := p.parseExpression(prec + 1)
		if err != nil {
			return nil, err
		}
		left = BinaryExpr{Op: op, Left: left, Right: right}
	}
	return left, nil
}

func (p *expressionParser) parsePrefix() (Expr, error) {
	tok := p.consume()
	switch tok.typ {
	case tokenIdentifier:
		if p.peek().typ == tokenLParen {
			return p.parseFunctionCall(tok.raw)
		}
		ref, err := ParseFactorRef(tok.raw)
		if err != nil {
			return nil, err
		}
		return FactorRefExpr{Ref: ref}, nil
	case tokenNumber:
		if strings.Contains(tok.raw, ".") {
			value, err := strconv.ParseFloat(tok.raw, 64)
			if err != nil {
				return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("invalid number %q", tok.raw)}
			}
			return LiteralExpr{Value: value, Type: ValueTypeDouble}, nil
		}
		value, err := strconv.ParseInt(tok.raw, 10, 64)
		if err != nil {
			return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("invalid integer %q", tok.raw)}
		}
		if value >= -2147483648 && value <= 2147483647 {
			return LiteralExpr{Value: int(value), Type: ValueTypeInt}, nil
		}
		return LiteralExpr{Value: value, Type: ValueTypeLong}, nil
	case tokenString:
		return LiteralExpr{Value: tok.raw, Type: ValueTypeString}, nil
	case tokenBool:
		return LiteralExpr{Value: tok.raw == "true", Type: ValueTypeBool}, nil
	case tokenNull:
		return LiteralExpr{Value: nil, Type: ValueTypeNull}, nil
	case tokenOperator:
		if tok.raw != "!" && tok.raw != "-" {
			return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unexpected operator %q", tok.raw)}
		}
		expr, err := p.parseExpression(7)
		if err != nil {
			return nil, err
		}
		return UnaryExpr{Op: tok.raw, Expr: expr}, nil
	case tokenLParen:
		expr, err := p.parseExpression(0)
		if err != nil {
			return nil, err
		}
		if p.consume().typ != tokenRParen {
			return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing closing parenthesis"}
		}
		return expr, nil
	case tokenLBracket:
		return p.parseListLiteral()
	case tokenLBrace:
		return p.parseMapLiteral()
	default:
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unexpected token %q", tok.raw)}
	}
}

func (p *expressionParser) parseListLiteral() (Expr, error) {
	var elements []Expr
	if p.peek().typ != tokenRBracket {
		for {
			elem, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			elements = append(elements, elem)
			if p.peek().typ == tokenComma {
				p.consume()
				continue
			}
			break
		}
	}
	if p.consume().typ != tokenRBracket {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing closing bracket"}
	}
	return ListExpr{Elements: elements}, nil
}

func (p *expressionParser) parseMapLiteral() (Expr, error) {
	var entries []MapEntryExpr
	if p.peek().typ != tokenRBrace {
		for {
			keyTok := p.consume()
			if keyTok.typ != tokenString {
				return nil, ValidationError{Code: ErrExpressionInvalid, Message: "map literal key must be a string"}
			}
			if p.consume().typ != tokenColon {
				return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing map entry colon"}
			}
			value, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			entries = append(entries, MapEntryExpr{Key: keyTok.raw, Value: value})
			if p.peek().typ == tokenComma {
				p.consume()
				continue
			}
			break
		}
	}
	if p.consume().typ != tokenRBrace {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing closing brace"}
	}
	return MapExpr{Entries: entries}, nil
}

func (p *expressionParser) parseFunctionCall(name string) (Expr, error) {
	if p.consume().typ != tokenLParen {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing function opening parenthesis"}
	}

	var args []Expr
	if p.peek().typ != tokenRParen {
		for {
			arg, err := p.parseExpression(0)
			if err != nil {
				return nil, err
			}
			args = append(args, arg)
			if p.peek().typ == tokenComma {
				p.consume()
				continue
			}
			break
		}
	}

	if p.consume().typ != tokenRParen {
		return nil, ValidationError{Code: ErrExpressionInvalid, Message: "missing closing parenthesis"}
	}
	return FunctionCallExpr{Name: name, Args: args}, nil
}

func (p *expressionParser) peek() token {
	if p.pos >= len(p.tokens) {
		return token{typ: tokenEOF}
	}
	return p.tokens[p.pos]
}

func (p *expressionParser) consume() token {
	tok := p.peek()
	if p.pos < len(p.tokens) {
		p.pos++
	}
	return tok
}

func tokenizeExpression(raw string) ([]token, error) {
	var tokens []token
	for i := 0; i < len(raw); {
		ch := raw[i]
		if unicode.IsSpace(rune(ch)) {
			i++
			continue
		}
		if isIdentifierStart(ch) {
			start := i
			i++
			for i < len(raw) && isIdentifierPart(raw[i]) {
				i++
			}
			word := raw[start:i]
			switch word {
			case "true", "false":
				tokens = append(tokens, token{typ: tokenBool, raw: word})
			case "null":
				tokens = append(tokens, token{typ: tokenNull, raw: word})
			default:
				tokens = append(tokens, token{typ: tokenIdentifier, raw: word})
			}
			continue
		}
		if isDigit(ch) {
			start := i
			i++
			for i < len(raw) && isDigit(raw[i]) {
				i++
			}
			if i < len(raw) && raw[i] == '.' {
				i++
				for i < len(raw) && isDigit(raw[i]) {
					i++
				}
			}
			tokens = append(tokens, token{typ: tokenNumber, raw: raw[start:i]})
			continue
		}
		if ch == '"' || ch == '\'' {
			quote := ch
			i++
			start := i
			for i < len(raw) && raw[i] != quote {
				i++
			}
			if i >= len(raw) {
				return nil, ValidationError{Code: ErrExpressionInvalid, Message: "unterminated string literal"}
			}
			tokens = append(tokens, token{typ: tokenString, raw: raw[start:i]})
			i++
			continue
		}
		if ch == '(' {
			tokens = append(tokens, token{typ: tokenLParen, raw: "("})
			i++
			continue
		}
		if ch == ')' {
			tokens = append(tokens, token{typ: tokenRParen, raw: ")"})
			i++
			continue
		}
		if ch == '[' {
			tokens = append(tokens, token{typ: tokenLBracket, raw: "["})
			i++
			continue
		}
		if ch == ']' {
			tokens = append(tokens, token{typ: tokenRBracket, raw: "]"})
			i++
			continue
		}
		if ch == '{' {
			tokens = append(tokens, token{typ: tokenLBrace, raw: "{"})
			i++
			continue
		}
		if ch == '}' {
			tokens = append(tokens, token{typ: tokenRBrace, raw: "}"})
			i++
			continue
		}
		if ch == ',' {
			tokens = append(tokens, token{typ: tokenComma, raw: ","})
			i++
			continue
		}
		if ch == ':' {
			tokens = append(tokens, token{typ: tokenColon, raw: ":"})
			i++
			continue
		}
		if ch == '?' {
			tokens = append(tokens, token{typ: tokenQuestion, raw: "?"})
			i++
			continue
		}
		if i+1 < len(raw) {
			double := raw[i : i+2]
			switch double {
			case "&&", "||", "==", "!=", "<=", ">=":
				tokens = append(tokens, token{typ: tokenOperator, raw: double})
				i += 2
				continue
			}
		}
		switch ch {
		case '<', '>', '+', '-', '*', '/', '!':
			tokens = append(tokens, token{typ: tokenOperator, raw: string(ch)})
			i++
		default:
			return nil, ValidationError{Code: ErrExpressionInvalid, Message: fmt.Sprintf("unexpected character %q", ch)}
		}
	}
	tokens = append(tokens, token{typ: tokenEOF})
	return tokens, nil
}

func isIdentifierStart(ch byte) bool {
	return ch == '_' || unicode.IsLetter(rune(ch))
}

func isIdentifierPart(ch byte) bool {
	return ch == '_' || ch == '.' || unicode.IsLetter(rune(ch)) || unicode.IsDigit(rune(ch))
}

func isDigit(ch byte) bool {
	return ch >= '0' && ch <= '9'
}

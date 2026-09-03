// Package lexer —— Method 语言词法分析器（手写实现，无 flex/bison 依赖）
//
// 特性：
//   - 40+ 关键字（大小写敏感），非关键字标识符返回 NAME
//   - 整数（十进制 / 0x 十六进制）、浮点（含科学计数法）
//   - 字符串（"..." 与 '...'，支持 \n \t \\ \" \' \0 等转义）
//   - 三种注释：// 行注释、# 行注释、/* */ 块注释
//   - 行号跟踪
package lexer

import (
	"fmt"
	"strconv"
	"unicode"
)

// TokenKind —— 词法 Token 类型
type TokenKind int

const (
	// 所有命名 TokenKind 从 256 开始，避免和单字符 ASCII (0..127) 冲突
	TEOF TokenKind = iota + 256
	TError

	// 字面量
	TInt
	TFloat
	TString
	TName

	// 字面量关键字
	TTrue
	TFalse
	TNull

	// 逻辑关键字
	TAnd
	TOr
	TNot
	TIs
	TIn

	// 流程 / 结构关键字
	TAs
	TAssert
	TAsync
	TAwait
	TBreak
	TClass
	TContinue
	TMethod
	TDel
	TElse
	TExcept
	TFinally
	TFor
	TFrom
	TGlobal
	TIf
	TImport
	TLambda
	TNonlocal
	TPass
	TRaise
	TReturn
	TTry
	TWhile
	TWith
	TYield
	TPublic
	TThis

	// OOP 关键字
	TNew
	TExtends
	TImplements
	TAbstract
	TInterface
	TPrivate
	TProtected
	TStatic
	TSuper
	TInstanceof
	TVar
	TGo
	// Go 风格集成关键字
	TDefer
	TRange

	// 多字符运算符
	TEq      // ==
	TNe      // !=
	TLe      // <=
	TGe      // >=
	TPlusEq  // +=
	TMinusEq // -=
	TStarEq  // *=
	TSlashEq // /=
	TModEq   // %=
	TScope   // ::
	TArrow   // ->

	// 单字符：直接用 ASCII 作为 TokenKind 便于 Parser switch
	// '+' '-' '*' '/' '%' '=' '<' '>' '!' '(' ')' '{' '}' '[' ']' '.' ',' ';' ':'
)

// Token —— 词法单元
type Token struct {
	Kind TokenKind
	Text string  // 名称 / 字符串值 / 原文
	IVal int64   // 整数值
	FVal float64 // 浮点值
	Line int
	Raw  rune // 单字符 token 的原始字符
}

// Lexer —— 词法分析器
type Lexer struct {
	src      []rune
	pos      int
	line     int
	keywords map[string]TokenKind
	lastTok  *Token // 最近返回的 token（用于错误上下文）
}

var keywordTable = map[string]TokenKind{
	"false":      TFalse,
	"null":       TNull,
	"true":       TTrue,
	"and":        TAnd,
	"as":         TAs,
	"assert":     TAssert,
	"async":      TAsync,
	"await":      TAwait,
	"break":      TBreak,
	"class":      TClass,
	"continue":   TContinue,
	"method":     TMethod,
	"del":        TDel,
	"else":       TElse,
	"except":     TExcept,
	"finally":    TFinally,
	"for":        TFor,
	"from":       TFrom,
	"global":     TGlobal,
	"if":         TIf,
	"import":     TImport,
	"in":         TIn,
	"is":         TIs,
	"lambda":     TLambda,
	"nonlocal":   TNonlocal,
	"not":        TNot,
	"or":         TOr,
	"pass":       TPass,
	"raise":      TRaise,
	"return":     TReturn,
	"try":        TTry,
	"while":      TWhile,
	"with":       TWith,
	"yield":      TYield,
	"public":     TPublic,
	"this":       TThis,
	"new":        TNew,
	"extends":    TExtends,
	"implements": TImplements,
	"abstract":   TAbstract,
	"interface":  TInterface,
	"private":    TPrivate,
	"protected":  TProtected,
	"static":     TStatic,
	"super":      TSuper,
	"instanceof": TInstanceof,
	"var":        TVar,
	"go":         TGo,
	"defer":      TDefer,
	"range":      TRange,
}

// New 创建词法分析器
func New(src string) *Lexer {
	return &Lexer{
		src:      []rune(src),
		pos:      0,
		line:     1,
		keywords: keywordTable,
	}
}

// Line 返回当前行号
func (l *Lexer) Line() int { return l.line }

func (l *Lexer) peek() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	return l.src[l.pos]
}

func (l *Lexer) peekAt(offset int) rune {
	p := l.pos + offset
	if p >= len(l.src) || p < 0 {
		return 0
	}
	return l.src[p]
}

func (l *Lexer) advance() rune {
	if l.pos >= len(l.src) {
		return 0
	}
	c := l.src[l.pos]
	l.pos++
	if c == '\n' {
		l.line++
	}
	return c
}

func (l *Lexer) skip(n int) {
	for i := 0; i < n; i++ {
		l.advance()
	}
}

// Next 返回下一个 Token；到达文件末尾时返回 TEOF
func (l *Lexer) Next() *Token {
	for {
		if l.pos >= len(l.src) {
			t := &Token{Kind: TEOF, Line: l.line}
			l.lastTok = t
			return t
		}
		c := l.peek()
		// 跳过 UTF-8 BOM
		if l.pos == 0 && c == 0xFEFF {
			l.advance()
			continue
		}
		// 空白
		if c == ' ' || c == '\t' || c == '\r' || c == '\f' || c == '\v' {
			l.advance()
			continue
		}
		if c == '\n' {
			l.advance()
			continue
		}
		// 注释
		if c == '/' {
			if l.peekAt(1) == '/' {
				l.skip(2)
				for l.pos < len(l.src) && l.peek() != '\n' {
					l.advance()
				}
				continue
			}
			if l.peekAt(1) == '*' {
				l.skip(2)
				for l.pos < len(l.src) {
					if l.peek() == '*' && l.peekAt(1) == '/' {
						l.skip(2)
						break
					}
					l.advance()
				}
				continue
			}
		}
		if c == '#' {
			for l.pos < len(l.src) && l.peek() != '\n' {
				l.advance()
			}
			continue
		}
		// 字面量
		if unicode.IsDigit(c) || (c == '.' && unicode.IsDigit(l.peekAt(1))) {
			return l.scanNumber()
		}
		if c == '"' || c == '\'' {
			return l.scanString()
		}
		// 标识符 / 关键字
		if unicode.IsLetter(c) || c == '_' {
			return l.scanIdent()
		}
		// 多字符运算符（先判断）
		tok, ok := l.tryMultiChar()
		if ok {
			l.lastTok = tok
			return tok
		}
		// 单字符
		if c == '+' || c == '-' || c == '*' || c == '/' || c == '%' ||
			c == '=' || c == '<' || c == '>' || c == '!' ||
			c == '(' || c == ')' || c == '{' || c == '}' ||
			c == '[' || c == ']' || c == '.' || c == ',' || c == ';' || c == ':' ||
			c == '&' {
			line := l.line
			l.advance()
			t := &Token{Kind: TokenKind(c), Raw: c, Line: line}
			l.lastTok = t
			return t
		}
		// 未知字符
		line := l.line
		l.advance()
		t := &Token{Kind: TError, Text: fmt.Sprintf("unrecognized character '%c'", c), Line: line}
		l.lastTok = t
		return t
	}
}

func (l *Lexer) scanNumber() *Token {
	start := l.pos
	line := l.line
	isHex := false
	// 检测 0x
	if l.peek() == '0' && (l.peekAt(1) == 'x' || l.peekAt(1) == 'X') {
		l.skip(2)
		isHex = true
	}
	isFloat := false
	for l.pos < len(l.src) {
		c := l.peek()
		if isHex {
			if ('0' <= c && c <= '9') || ('a' <= c && c <= 'f') || ('A' <= c && c <= 'F') {
				l.advance()
				continue
			}
			break
		}
		if unicode.IsDigit(c) {
			l.advance()
			continue
		}
		if c == '.' {
			// 避免与成员访问 . 歧义：后面必须是数字
			next := l.peekAt(1)
			if !isFloat && unicode.IsDigit(next) {
				isFloat = true
				l.advance()
				continue
			}
			break
		}
		if (c == 'e' || c == 'E') && !isFloat {
			next := l.peekAt(1)
			if next == '+' || next == '-' {
				if unicode.IsDigit(l.peekAt(2)) {
					isFloat = true
					l.skip(2)
					continue
				}
			} else if unicode.IsDigit(next) {
				isFloat = true
				l.advance()
				continue
			}
			break
		}
		break
	}
	text := string(l.src[start:l.pos])
	tok := &Token{Line: line}
	if isFloat {
		f, err := strconv.ParseFloat(text, 64)
		if err != nil {
			tok.Kind = TError
			tok.Text = "invalid float: " + text
			return tok
		}
		tok.Kind = TFloat
		tok.FVal = f
		return tok
	}
	tok.Kind = TInt
	if isHex {
		v, err := strconv.ParseInt(text[2:], 16, 64)
		if err != nil {
			tok.Kind = TError
			tok.Text = "invalid hex: " + text
			return tok
		}
		tok.IVal = v
	} else {
		v, err := strconv.ParseInt(text, 10, 64)
		if err != nil {
			tok.Kind = TError
			tok.Text = "invalid int: " + text
			return tok
		}
		tok.IVal = v
	}
	return tok
}

func (l *Lexer) scanString() *Token {
	line := l.line
	quote := l.advance() // consume " or '
	var buf []rune
	for l.pos < len(l.src) {
		c := l.peek()
		if c == quote {
			l.advance()
			return &Token{Kind: TString, Text: string(buf), Line: line}
		}
		if c == '\\' {
			l.advance()
			nc := l.peek()
			switch nc {
			case 'n':
				buf = append(buf, '\n')
			case 't':
				buf = append(buf, '\t')
			case 'r':
				buf = append(buf, '\r')
			case '\\':
				buf = append(buf, '\\')
			case '"':
				buf = append(buf, '"')
			case '\'':
				buf = append(buf, '\'')
			case '0':
				buf = append(buf, 0)
			case 'f':
				buf = append(buf, '\f')
			case 'v':
				buf = append(buf, '\v')
			default:
				buf = append(buf, '\\', nc)
			}
			l.advance()
			continue
		}
		if c == '\n' {
			break
		}
		buf = append(buf, c)
		l.advance()
	}
	return &Token{Kind: TError, Text: "unterminated string", Line: line}
}

func (l *Lexer) scanIdent() *Token {
	start := l.pos
	line := l.line
	for l.pos < len(l.src) {
		c := l.peek()
		if unicode.IsLetter(c) || unicode.IsDigit(c) || c == '_' {
			l.advance()
			continue
		}
		break
	}
	text := string(l.src[start:l.pos])
	if kw, ok := l.keywords[text]; ok {
		return &Token{Kind: kw, Text: text, Line: line}
	}
	return &Token{Kind: TName, Text: text, Line: line}
}

func (l *Lexer) tryMultiChar() (*Token, bool) {
	c := l.peek()
	line := l.line
	match := func(s string, kind TokenKind) (*Token, bool) {
		if l.pos+len(s) > len(l.src) {
			return nil, false
		}
		for i := 0; i < len(s); i++ {
			if rune(s[i]) != l.src[l.pos+i] {
				return nil, false
			}
		}
		l.pos += len(s)
		return &Token{Kind: kind, Text: s, Line: line}, true
	}
	switch c {
	case '=':
		if l.peekAt(1) == '=' {
			return match("==", TEq)
		}
	case '!':
		if l.peekAt(1) == '=' {
			return match("!=", TNe)
		}
	case '<':
		if l.peekAt(1) == '=' {
			return match("<=", TLe)
		}
	case '>':
		if l.peekAt(1) == '=' {
			return match(">=", TGe)
		}
	case '+':
		if l.peekAt(1) == '=' {
			return match("+=", TPlusEq)
		}
	case '-':
		if l.peekAt(1) == '=' {
			return match("-=", TMinusEq)
		}
		if l.peekAt(1) == '>' {
			return match("->", TArrow)
		}
	case '*':
		if l.peekAt(1) == '=' {
			return match("*=", TStarEq)
		}
	case '/':
		if l.peekAt(1) == '=' {
			return match("/=", TSlashEq)
		}
	case '%':
		if l.peekAt(1) == '=' {
			return match("%=", TModEq)
		}
	case ':':
		if l.peekAt(1) == ':' {
			return match("::", TScope)
		}
	}
	return nil, false
}

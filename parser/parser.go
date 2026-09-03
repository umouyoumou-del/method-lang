// Package parser —— Method 语言递归下降语法分析器（将 Token 流 → AST）
//
// 语法设计：
//   - Python 风格：and/or/not/in/is/lambda/pass/del/yield 等关键字
//   - Java/C++ 风格：class/method/public/this；花括号 { }；分号 ; 结束
//   - 可选类型标注 name : type
package parser

import (
	"fmt"

	"method/ast"
	"method/lexer"
)

// Parser —— 语法分析器
type Parser struct {
	lex    *lexer.Lexer
	tokens []*lexer.Token // 预读缓存
	pos    int
	errors []string
	// suppressTuple>0 表示处于逗号分隔表达式上下文（调用参数/列表字面量等），
	// parseAssignment 不得把 "a, b" 误判为多目标赋值
	suppressTuple int
}

// New 创建 Parser
func New(l *lexer.Lexer) *Parser {
	return &Parser{lex: l, tokens: nil, pos: 0}
}

// Errors 返回解析错误
func (p *Parser) Errors() []string { return p.errors }

func (p *Parser) peek(n int) *lexer.Token {
	for len(p.tokens) <= p.pos+n {
		t := p.lex.Next()
		p.tokens = append(p.tokens, t)
		if t.Kind == lexer.TEOF {
			break
		}
	}
	idx := p.pos + n
	if idx >= len(p.tokens) {
		// 返回末尾 EOF
		if len(p.tokens) > 0 {
			return p.tokens[len(p.tokens)-1]
		}
		return &lexer.Token{Kind: lexer.TEOF, Line: p.lex.Line()}
	}
	return p.tokens[idx]
}

func (p *Parser) cur() *lexer.Token { return p.peek(0) }

func (p *Parser) advance() *lexer.Token {
	t := p.cur()
	p.pos++
	return t
}

func (p *Parser) line() int {
	if t := p.cur(); t != nil {
		return t.Line
	}
	return 0
}

func (p *Parser) errorf(format string, args ...any) {
	msg := fmt.Sprintf("line %d: %s", p.line(), fmt.Sprintf(format, args...))
	p.errors = append(p.errors, msg)
}

// checkKind 判断当前 token 类型是否匹配
func (p *Parser) isKind(kind lexer.TokenKind) bool {
	return p.cur().Kind == kind
}

// checkRune 判断当前是否为单字符 token
func (p *Parser) isRune(r rune) bool {
	return p.cur().Kind == lexer.TokenKind(r)
}

// accept 若匹配则消费并返回 true
func (p *Parser) accept(kind lexer.TokenKind) bool {
	if p.isKind(kind) {
		p.advance()
		return true
	}
	return false
}

func (p *Parser) acceptRune(r rune) bool {
	if p.isRune(r) {
		p.advance()
		return true
	}
	return false
}

// expect 要求下一个 token 匹配，否则报错并返回 nil
func (p *Parser) expect(kind lexer.TokenKind, what string) *lexer.Token {
	t := p.cur()
	if t.Kind != kind {
		p.errorf("expected %s, got %s", what, tokenStr(t))
		return nil
	}
	p.advance()
	return t
}

func (p *Parser) expectRune(r rune, what string) bool {
	if !p.isRune(r) {
		p.errorf("expected %s, got %s", what, tokenStr(p.cur()))
		return false
	}
	p.advance()
	return true
}

func tokenStr(t *lexer.Token) string {
	switch t.Kind {
	case lexer.TEOF:
		return "<EOF>"
	case lexer.TError:
		return t.Text
	case lexer.TName:
		return "name '" + t.Text + "'"
	case lexer.TString:
		return "string"
	case lexer.TInt:
		return "integer"
	case lexer.TFloat:
		return "float"
	}
	if t.Kind < 256 && t.Raw != 0 {
		return fmt.Sprintf("'%c'", t.Raw)
	}
	return tokenName(t.Kind)
}

func tokenName(k lexer.TokenKind) string {
	switch k {
	case lexer.TTrue:
		return "true"
	case lexer.TFalse:
		return "false"
	case lexer.TNull:
		return "null"
	case lexer.TAnd:
		return "and"
	case lexer.TOr:
		return "or"
	case lexer.TNot:
		return "not"
	case lexer.TIs:
		return "is"
	case lexer.TIn:
		return "in"
	case lexer.TAs:
		return "as"
	case lexer.TAssert:
		return "assert"
	case lexer.TBreak:
		return "break"
	case lexer.TClass:
		return "class"
	case lexer.TContinue:
		return "continue"
	case lexer.TMethod:
		return "method"
	case lexer.TDel:
		return "del"
	case lexer.TElse:
		return "else"
	case lexer.TExcept:
		return "except"
	case lexer.TFinally:
		return "finally"
	case lexer.TFor:
		return "for"
	case lexer.TFrom:
		return "from"
	case lexer.TGlobal:
		return "global"
	case lexer.TIf:
		return "if"
	case lexer.TImport:
		return "import"
	case lexer.TLambda:
		return "lambda"
	case lexer.TNonlocal:
		return "nonlocal"
	case lexer.TPass:
		return "pass"
	case lexer.TRaise:
		return "raise"
	case lexer.TReturn:
		return "return"
	case lexer.TTry:
		return "try"
	case lexer.TWhile:
		return "while"
	case lexer.TWith:
		return "with"
	case lexer.TYield:
		return "yield"
	case lexer.TPublic:
		return "public"
	case lexer.TThis:
		return "this"
	case lexer.TNew:
		return "new"
	case lexer.TExtends:
		return "extends"
	case lexer.TImplements:
		return "implements"
	case lexer.TAbstract:
		return "abstract"
	case lexer.TInterface:
		return "interface"
	case lexer.TPrivate:
		return "private"
	case lexer.TProtected:
		return "protected"
	case lexer.TStatic:
		return "static"
	case lexer.TSuper:
		return "super"
	case lexer.TInstanceof:
		return "instanceof"
	case lexer.TVar:
		return "var"
	case lexer.TGo:
		return "go"
	case lexer.TEq:
		return "'=='"
	case lexer.TNe:
		return "'!='"
	case lexer.TLe:
		return "'<='"
	case lexer.TGe:
		return "'>='"
	case lexer.TPlusEq:
		return "'+='"
	case lexer.TMinusEq:
		return "'-='"
	case lexer.TStarEq:
		return "'*='"
	case lexer.TSlashEq:
		return "'/='"
	case lexer.TModEq:
		return "'%='"
	case lexer.TScope:
		return "'::'"
	case lexer.TArrow:
		return "'->'"
	}
	return "?"
}

// ===== 顶层 =====

// ParseProgram 解析整个程序
func (p *Parser) ParseProgram() *ast.Node {
	prog := ast.New(ast.NProgram, "", p.line())
	for !p.isKind(lexer.TEOF) {
		s := p.parseStmt()
		if s != nil {
			prog.Add(s)
		} else {
			// 同步：前进一个 token
			p.advance()
		}
	}
	return prog
}

// ===== 语句 =====

func (p *Parser) parseStmt() *ast.Node {
	t := p.cur()
	switch t.Kind {
	case lexer.TImport, lexer.TFrom:
		return p.parseImport()
	case lexer.TClass:
		return p.parseClass(0)
	case lexer.TInterface:
		return p.parseInterface(0)
	case lexer.TMethod:
		return p.parseMethod(0)
	case lexer.TIf:
		return p.parseIf()
	case lexer.TWhile:
		return p.parseWhile()
	case lexer.TFor:
		return p.parseFor()
	case lexer.TTry:
		return p.parseTry()
	case lexer.TReturn:
		return p.parseReturn()
	case lexer.TBreak:
		line := p.line()
		p.advance()
		p.expectRune(';', "';' after break")
		return ast.New(ast.NBreak, "", line)
	case lexer.TContinue:
		line := p.line()
		p.advance()
		p.expectRune(';', "';' after continue")
		return ast.New(ast.NContinue, "", line)
	case lexer.TPass:
		line := p.line()
		p.advance()
		p.expectRune(';', "';' after pass")
		return ast.New(ast.NPass, "", line)
	case lexer.TVar:
		return p.parseVarDecl()
	case lexer.TGo:
		return p.parseGoStmt()
	case lexer.TRaise:
		return p.parseRaise()
	case lexer.TAssert:
		return p.parseAssert()
	case lexer.TWith:
		return p.parseWith()
	case lexer.TGlobal:
		return p.parseGlobal()
	case lexer.TNonlocal:
		return p.parseNonlocal()
	case lexer.TPublic, lexer.TPrivate, lexer.TProtected,
		lexer.TStatic, lexer.TAbstract, lexer.TAsync:
		mods := p.parseModifiers()
		if p.isKind(lexer.TClass) {
			return p.parseClass(mods)
		}
		if p.isKind(lexer.TInterface) {
			return p.parseInterface(mods)
		}
		if p.isKind(lexer.TMethod) {
			return p.parseMethod(mods)
		}
		// field: modifiers NAME ':' type ...
		if p.isKind(lexer.TName) && (p.peek(1).Kind == lexer.TokenKind(':') || p.peek(1).Kind == lexer.TEq) {
			return p.parseField(mods)
		}
		// 作为表达式的一部分（如 x = public 非法，报错）
		return p.parseExprStmt()
	case lexer.TokenKind('{'):
		return p.parseBlock()
	}
	return p.parseExprStmt()
}

// parseModifiers 解析访问修饰符组合：bit[1:0]=access(0=pkg,1=pub,2=priv,3=prot),
// bit2=static, bit3=abstract, bit4=async
func (p *Parser) parseModifiers() int {
	m := 0
	done := false
	for !done {
		switch p.cur().Kind {
		case lexer.TPublic:
			m = (m &^ 3) | 1
			p.advance()
		case lexer.TPrivate:
			m = (m &^ 3) | 2
			p.advance()
		case lexer.TProtected:
			m = (m &^ 3) | 3
			p.advance()
		case lexer.TStatic:
			m |= 1 << 2
			p.advance()
		case lexer.TAbstract:
			m |= 1 << 3
			p.advance()
		case lexer.TAsync:
			m |= 1 << 4
			p.advance()
		default:
			done = true
		}
	}
	return m
}

func (p *Parser) parseImport() *ast.Node {
	line := p.line()
	var module string
	if p.accept(lexer.TFrom) {
		if t := p.expect(lexer.TName, "module name after 'from'"); t != nil {
			module = t.Text
		}
		p.expect(lexer.TImport, "'import'")
	} else {
		p.advance() // 'import'
	}
	n := ast.New(ast.NImport, module, line)
	list := ast.New(ast.NList, "imports", line)
	// import items
	first := true
	for {
		if !first && !p.acceptRune(',') {
			break
		}
		t := p.cur()
		if t.Kind != lexer.TName {
			if first {
				p.errorf("expected import item name")
			}
			break
		}
		p.advance()
		item := ast.New(ast.NName, t.Text, t.Line)
		if p.accept(lexer.TAs) {
			at := p.expect(lexer.TName, "alias name after 'as'")
			if at != nil {
				item.Add(ast.New(ast.NName, at.Text, at.Line))
			}
		}
		list.Add(item)
		first = false
		// 如果下一个是 ; 或 } 或 EOF，退出
		if p.isRune(';') || p.isKind(lexer.TEOF) || p.isRune('}') {
			break
		}
	}
	n.MoveChildren(list)
	p.acceptRune(';')
	return n
}

func (p *Parser) parseClass(mods int) *ast.Node {
	line := p.line()
	p.expect(lexer.TClass, "'class'")
	name := p.expect(lexer.TName, "class name")
	if name == nil {
		return nil
	}
	n := ast.New(ast.NClass, name.Text, line)
	n.IsPublic = mods
	// extends
	var extList *ast.Node
	if p.accept(lexer.TExtends) {
		if t := p.expect(lexer.TName, "parent class name"); t != nil {
			extList = ast.New(ast.NName, t.Text, t.Line)
		}
	}
	if extList == nil {
		extList = ast.New(ast.NList, "extends", line)
	}
	n.Add(extList)
	// implements
	var implList *ast.Node
	if p.accept(lexer.TImplements) {
		implList = ast.New(ast.NList, "implements", line)
		first := true
		for {
			if !first && !p.acceptRune(',') {
				break
			}
			t := p.expect(lexer.TName, "interface name")
			if t == nil {
				break
			}
			implList.Add(ast.New(ast.NName, t.Text, t.Line))
			first = false
			if p.isRune('{') || p.isRune('(') {
				break
			}
		}
	}
	if implList == nil {
		implList = ast.New(ast.NList, "implements", line)
	}
	n.Add(implList)
	// class params (constructor shorthand)
	params := ast.New(ast.NList, "params", line)
	if p.isRune('(') {
		p.advance()
		params = p.parseParamList()
		p.expectRune(')', "')' after class params")
	}
	n.Add(params)
	// body
	body := ast.New(ast.NBlock, "class_body", line)
	if p.expectRune('{', "'{' after class declaration") {
		for !p.isRune('}') && !p.isKind(lexer.TEOF) {
			m := p.parseClassMember()
			if m != nil {
				body.Add(m)
			}
		}
		p.expectRune('}', "'}' to close class body")
	}
	n.Add(body)
	return n
}

func (p *Parser) parseClassMember() *ast.Node {
	// 修饰符可能前置
	mods := p.parseModifiers()
	switch p.cur().Kind {
	case lexer.TClass:
		return p.parseClass(mods)
	case lexer.TInterface:
		return p.parseInterface(mods)
	case lexer.TMethod:
		return p.parseMethod(mods)
	case lexer.TVar:
		return p.parseVarField(mods)
	}
	// 字段: modifiers NAME ':' type opt_init ;
	if p.isKind(lexer.TName) && (p.peek(1).Kind == lexer.TokenKind(':') || p.peek(1).Kind == lexer.TEq) {
		return p.parseField(mods)
	}
	// 其他语句（Python 风格：类体内可直书语句）
	return p.parseStmt()
}

// parseVarField 解析类体内 var 字段声明（OOP 集成）：
//
//	var name;                  → 实例字段，默认值 0
//	var name = expr;           → 带初始值
//	var name : Type;           → 带类型（基本类型名或类名，仅注记）
//	var name : Type = expr;
//	private var name;          → 访问修饰符可组合
//	static var name = 5;       → 静态字段
//	private static var name;   → 多修饰符
//
// 降为 NField 节点（与显式字段声明同构），编译器按既有字段路径处理
// （伪码段、VTable/FieldTable、TotalInstanceSlots、mergeFromParent 全部复用）。
func (p *Parser) parseVarField(mods int) *ast.Node {
	line := p.line()
	p.advance() // 'var'
	t := p.expect(lexer.TName, "field name after 'var'")
	if t == nil {
		return nil
	}
	n := ast.New(ast.NField, t.Text, line)
	n.IsPublic = mods
	// 类型（支持类名）；无类型用空 NList 占位，保证子节点位置布局固定：
	// Children[0]=类型（或占位），Children[1]=初始值（或占位）
	if p.acceptRune(':') {
		typ := p.parseType()
		n.Add(typ)
	} else {
		n.Add(ast.New(ast.NList, "", line))
	}
	// 初始值
	if p.acceptRune('=') {
		e := p.parseExpr()
		n.Add(e)
	} else {
		n.Add(ast.New(ast.NList, "", line))
	}
	p.expectRune(';', "';' after var field declaration")
	return n
}

func (p *Parser) parseField(mods int) *ast.Node {
	line := p.line()
	t := p.expect(lexer.TName, "field name")
	if t == nil {
		return nil
	}
	n := ast.New(ast.NField, t.Text, line)
	n.IsPublic = mods
	// 类型；无类型用空 NList 占位（布局同 parseVarField：[0]=类型 [1]=初始值）
	if p.acceptRune(':') {
		typ := p.parseType()
		n.Add(typ)
	} else {
		n.Add(ast.New(ast.NList, "", line))
	}
	// 初始值
	if p.acceptRune('=') {
		e := p.parseExpr()
		n.Add(e)
	} else {
		n.Add(ast.New(ast.NList, "", line))
	}
	p.expectRune(';', "';' after field declaration")
	return n
}

func (p *Parser) parseInterface(mods int) *ast.Node {
	line := p.line()
	p.expect(lexer.TInterface, "'interface'")
	name := p.expect(lexer.TName, "interface name")
	if name == nil {
		return nil
	}
	n := ast.New(ast.NInterface, name.Text, line)
	n.IsPublic = mods
	// extends (多继承)
	var extList *ast.Node
	if p.accept(lexer.TExtends) {
		extList = ast.New(ast.NList, "extends", line)
		first := true
		for {
			if !first && !p.acceptRune(',') {
				break
			}
			t := p.expect(lexer.TName, "parent interface name")
			if t == nil {
				break
			}
			extList.Add(ast.New(ast.NName, t.Text, t.Line))
			first = false
			if p.isRune('{') {
				break
			}
		}
	}
	if extList == nil {
		extList = ast.New(ast.NList, "extends", line)
	}
	n.Add(extList)
	// iface body
	body := ast.New(ast.NBlock, "iface_body", line)
	if p.expectRune('{', "'{' after interface declaration") {
		for !p.isRune('}') && !p.isKind(lexer.TEOF) {
			// interface 只允许抽象方法
			m := p.parseIfaceMember()
			if m != nil {
				body.Add(m)
			} else {
				p.advance()
			}
		}
		p.expectRune('}', "'}' to close interface body")
	}
	n.Add(body)
	return n
}

func (p *Parser) parseIfaceMember() *ast.Node {
	mods := p.parseModifiers()
	if !p.isKind(lexer.TMethod) {
		p.errorf("expected method declaration in interface")
		return nil
	}
	m := p.parseMethod(mods)
	// 接口方法强制 public + abstract
	m.IsPublic = (m.IsPublic &^ 3) | 1 // access=public
	m.IsPublic |= 1 << 3               // abstract
	// body 置 nil（若为空）
	if m != nil && len(m.Children) >= 2 {
		if len(m.Children) == 2 && m.Children[1] == nil {
			// 已经是抽象
		}
	}
	return m
}

func (p *Parser) parseMethod(mods int) *ast.Node {
	line := p.line()
	p.expect(lexer.TMethod, "'method'")
	name := p.expect(lexer.TName, "method name")
	if name == nil {
		return nil
	}
	n := ast.New(ast.NMethod, name.Text, line)
	n.IsPublic = mods
	// params
	if !p.expectRune('(', "'(' after method name") {
		return n
	}
	params := p.parseParamList()
	p.expectRune(')', "')' after params")
	n.Add(params)
	// 可选返回值类型列表：method f(a, b) (int, str) { ... } → retN = 类型个数
	if p.isRune('(') {
		p.advance()
		retN := 0
		for !p.isRune(')') && !p.isKind(lexer.TEOF) {
			typ := p.parseType()
			if typ != nil {
				retN++
			}
			if !p.acceptRune(',') {
				break
			}
		}
		p.expectRune(')', "')' to close return type list")
		n.IVal = int64(retN)
	}
	// body or ;
	if p.acceptRune(';') {
		n.Add(nil)
		return n
	}
	body := p.parseBlock()
	n.Add(body)
	return n
}

func (p *Parser) parseParamList() *ast.Node {
	line := p.line()
	list := ast.New(ast.NList, "params", line)
	if p.isRune(')') {
		return list
	}
	for {
		t := p.expect(lexer.TName, "parameter name")
		if t == nil {
			// 恢复：跳过直到 )
			for !p.isRune(')') && !p.isKind(lexer.TEOF) {
				p.advance()
			}
			return list
		}
		param := ast.New(ast.NParam, t.Text, t.Line)
		// opt_type
		if p.acceptRune(':') {
			typ := p.parseType()
			param.Add(typ)
		} else {
			param.Add(nil)
		}
		// opt_init
		if p.acceptRune('=') {
			e := p.parseExpr()
			param.Add(e)
		} else {
			param.Add(nil)
		}
		list.Add(param)
		if !p.acceptRune(',') {
			break
		}
	}
	return list
}

func (p *Parser) parseType() *ast.Node {
	line := p.line()
	t := p.expect(lexer.TName, "type name")
	if t == nil {
		return ast.New(ast.NName, "", line)
	}
	n := ast.New(ast.NName, t.Text, t.Line)
	for p.isRune('[') && p.peek(1).Kind == lexer.TokenKind(']') {
		p.skip2()
		arr := ast.New(ast.NList, "[]", line)
		arr.Add(n)
		n = arr
	}
	return n
}

func (p *Parser) skip2() {
	p.advance()
	p.advance()
}

func (p *Parser) parseBlock() *ast.Node {
	line := p.line()
	if !p.expectRune('{', "'{' to begin block") {
		// 恢复：创建空 block
		return ast.New(ast.NBlock, "", line)
	}
	block := ast.New(ast.NBlock, "", line)
	for !p.isRune('}') && !p.isKind(lexer.TEOF) {
		s := p.parseStmt()
		if s != nil {
			block.Add(s)
		} else {
			p.advance()
		}
	}
	p.expectRune('}', "'}' to close block")
	return block
}

func (p *Parser) parseIf() *ast.Node {
	line := p.line()
	p.expect(lexer.TIf, "'if'")
	cond := p.parseExpr()
	thenBlock := p.parseBlock()
	n := ast.New(ast.NIf, "", line)
	n.Add(cond)
	n.Add(thenBlock)
	if p.accept(lexer.TElse) {
		if p.isKind(lexer.TIf) {
			elseIf := p.parseIf()
			n.Add(elseIf)
		} else {
			elseBlock := p.parseBlock()
			n.Add(elseBlock)
		}
	} else {
		n.Add(nil)
	}
	return n
}

func (p *Parser) parseWhile() *ast.Node {
	line := p.line()
	p.expect(lexer.TWhile, "'while'")
	cond := p.parseExpr()
	body := p.parseBlock()
	n := ast.New(ast.NWhile, "", line)
	n.Add(cond)
	n.Add(body)
	return n
}

func (p *Parser) parseFor() *ast.Node {
	line := p.line()
	p.expect(lexer.TFor, "'for'")
	// for NAME in expr block
	// or for ( init ; cond ; update ) block
	if p.isRune('(') {
		p.advance()
		var init, cond, update *ast.Node
		if !p.isRune(';') {
			init = p.parseExpr()
		}
		p.expectRune(';', "';' in for")
		if !p.isRune(';') {
			cond = p.parseExpr()
		}
		p.expectRune(';', "';' in for")
		if !p.isRune(')') {
			update = p.parseExpr()
		}
		p.expectRune(')', "')' after for header")
		body := p.parseBlock()
		n := ast.New(ast.NForC, "", line)
		n.Add(init)
		n.Add(cond)
		n.Add(update)
		n.Add(body)
		return n
	}
	// for-in
	t := p.expect(lexer.TName, "loop variable name")
	if t == nil {
		return nil
	}
	p.expect(lexer.TIn, "'in'")
	iter := p.parseExpr()
	body := p.parseBlock()
	n := ast.New(ast.NForIn, "", line)
	n.Add(ast.New(ast.NName, t.Text, t.Line))
	n.Add(iter)
	n.Add(body)
	return n
}

func (p *Parser) parseTry() *ast.Node {
	line := p.line()
	p.expect(lexer.TTry, "'try'")
	tryBlock := p.parseBlock()
	n := ast.New(ast.NTry, "", line)
	n.Add(tryBlock)
	// except list
	for p.accept(lexer.TExcept) {
		excLine := p.line()
		var excBlock *ast.Node
		var excName, asName string
		if p.isKind(lexer.TName) {
			excName = p.cur().Text
			p.advance()
			if p.accept(lexer.TAs) {
				t := p.expect(lexer.TName, "exception variable name")
				if t != nil {
					asName = t.Text
				}
			}
			excBlock = p.parseBlock()
		} else if p.accept(lexer.TAs) {
			// except as e { }：无类型名，仅绑定消息变量
			t := p.expect(lexer.TName, "exception variable name")
			if t != nil {
				asName = t.Text
			}
			excBlock = p.parseBlock()
		} else {
			excBlock = p.parseBlock()
		}
		exc := ast.New(ast.NBlock, excName, excLine)
		if asName != "" {
			exc.Add(ast.New(ast.NName, asName, excLine))
		}
		exc.Add(excBlock)
		n.Add(exc)
	}
	if p.accept(lexer.TFinally) {
		finLine := p.line()
		fb := p.parseBlock()
		// 用 NFinally 标记节点包裹，与无名 except（NBlock）区分
		fn := ast.New(ast.NFinally, "", finLine)
		fn.Add(fb)
		n.Add(fn)
	}
	return n
}

func (p *Parser) parseReturn() *ast.Node {
	line := p.line()
	p.advance() // return
	n := ast.New(ast.NReturn, "", line)
	// return 的逗号属于多返回值列表而非多目标赋值，期间抑制 tuple 赋值解析
	p.suppressTuple++
	if !p.isRune(';') && !p.isRune('}') && !p.isKind(lexer.TEOF) {
		e := p.parseExpr()
		if e == nil {
			p.suppressTuple--
			return n
		}
		if p.isRune(',') {
			// 多返回值：return a, b, c;
			list := ast.New(ast.NList, "rets", line)
			list.Add(e)
			for p.acceptRune(',') {
				extra := p.parseExpr()
				if extra == nil {
					break
				}
				list.Add(extra)
			}
			n.Add(list)
		} else {
			n.Add(e)
		}
	}
	p.suppressTuple--
	p.acceptRune(';')
	return n
}

func (p *Parser) parseRaise() *ast.Node {
	line := p.line()
	p.advance()
	n := ast.New(ast.NRaise, "", line)
	if !p.isRune(';') && !p.isRune('}') && !p.isKind(lexer.TEOF) {
		e := p.parseExpr()
		n.Add(e)
	}
	p.acceptRune(';')
	return n
}

func (p *Parser) parseAssert() *ast.Node {
	line := p.line()
	p.advance()
	e := p.parseExpr()
	n := ast.New(ast.NAssert, "", line)
	n.Add(e)
	p.expectRune(';', "';' after assert")
	return n
}

func (p *Parser) parseWith() *ast.Node {
	line := p.line()
	p.advance()
	n := ast.New(ast.NWith, "", line)
	first := true
	p.suppressTuple++
	for {
		if !first && !p.acceptRune(',') {
			break
		}
		e := p.parseExpr()
		if e == nil {
			break
		}
		if p.accept(lexer.TAs) {
			t := p.expect(lexer.TName, "name after 'as'")
			if t != nil {
				e.Add(ast.New(ast.NName, t.Text, t.Line))
			}
		}
		n.Add(e)
		first = false
		if p.isRune('{') {
			break
		}
	}
	p.suppressTuple--
	body := p.parseBlock()
	n.Add(body)
	return n
}

func (p *Parser) parseGlobal() *ast.Node {
	line := p.line()
	p.advance()
	n := ast.New(ast.NGlobal, "global", line)
	for {
		t := p.expect(lexer.TName, "global name")
		if t == nil {
			break
		}
		n.Add(ast.New(ast.NName, t.Text, t.Line))
		if !p.acceptRune(',') {
			break
		}
	}
	p.expectRune(';', "';' after global")
	return n
}

func (p *Parser) parseNonlocal() *ast.Node {
	line := p.line()
	p.advance()
	n := ast.New(ast.NNonlocal, "nonlocal", line)
	for {
		t := p.expect(lexer.TName, "nonlocal name")
		if t == nil {
			break
		}
		n.Add(ast.New(ast.NName, t.Text, t.Line))
		if !p.acceptRune(',') {
			break
		}
	}
	p.expectRune(';', "';' after nonlocal")
	return n
}

func (p *Parser) parseExprStmt() *ast.Node {
	line := p.line()
	e := p.parseExpr()
	if e == nil {
		return nil
	}
	// 多目标赋值 a, b = f()：是语句而非表达式，不包裹 NExprStmt
	if e.Kind == ast.NMultiAssign {
		p.acceptRune(';')
		return e
	}
	n := ast.New(ast.NExprStmt, "", line)
	n.Add(e)
	p.acceptRune(';')
	return n
}

// parseGoStmt 解析 go 语句：
//
//	go methodName(arg1, arg2, ...);
//
// 生成 NGo 节点（children[0]=方法名, children[1]=参数列表），VM 以独立线程异步执行。
func (p *Parser) parseGoStmt() *ast.Node {
	line := p.line()
	p.advance() // 'go'
	t := p.expect(lexer.TName, "method name after 'go'")
	if t == nil {
		return nil
	}
	n := ast.New(ast.NGo, "", line)
	name := ast.New(ast.NName, t.Text, t.Line)
	n.Add(name)
	args := ast.New(ast.NList, "", t.Line)
	if !p.isRune('(') {
		p.errorf("expected '(' after go method name")
		return nil
	}
	p.advance()
	p.suppressTuple++
	for !p.isRune(')') {
		a := p.parseAssignment()
		if a == nil {
			p.suppressTuple--
			return nil
		}
		args.Add(a)
		if !p.isRune(',') {
			break
		}
		p.advance()
	}
	p.suppressTuple--
	if !p.isRune(')') {
		p.errorf("expected ')' to close go args")
		return nil
	}
	p.advance()
	n.Add(args)
	p.acceptRune(';')
	return n
}

// parseVarDecl 解析 var 声明：
//
//	var x;                  → x = 0
//	var x = expr;           → x = expr
//	var x : type;           → x = 0（类型仅作注记）
//	var x : type = expr;    → x = expr
//	var q, r = divmod(..);  → NMultiAssign（多返回值同时接收）
//
// 单值统一降为 NAssign（Text="var"），slot 分配沿用既有赋值路径。
func (p *Parser) parseVarDecl() *ast.Node {
	line := p.line()
	p.advance() // 'var'
	t := p.expect(lexer.TName, "variable name after 'var'")
	if t == nil {
		return nil
	}
	names := []*ast.Node{ast.New(ast.NName, t.Text, t.Line)}
	// 可选类型标注 ': type'（仅单变量；多变量声明不做类型标注）
	for p.acceptRune(',') {
		nt := p.expect(lexer.TName, "variable name after ','")
		if nt == nil {
			return nil
		}
		names = append(names, ast.New(ast.NName, nt.Text, nt.Line))
	}
	if len(names) == 1 && p.isRune(':') {
		p.advance()
		if p.expect(lexer.TName, "type name after ':'") == nil {
			return nil
		}
	}
	var rhs *ast.Node
	if p.isRune('=') {
		p.advance()
		rhs = p.parseAssignment()
		if rhs == nil {
			return nil
		}
	} else {
		rhs = ast.New(ast.NInt, "0", line)
	}
	p.acceptRune(';')
	if len(names) == 1 {
		n := ast.New(ast.NAssign, "var", line)
		n.Add(names[0])
		n.Add(rhs)
		return n
	}
	// 多目标：var a, b = f()
	multi := ast.New(ast.NMultiAssign, "var", line)
	lhsList := ast.New(ast.NList, "lhs", line)
	for _, nm := range names {
		lhsList.Add(nm)
	}
	multi.Add(lhsList)
	multi.Add(rhs)
	return multi
}

// ===== 表达式（显式优先级，最低→最高）=====

// expr = assignment
func (p *Parser) parseExpr() *ast.Node {
	return p.parseAssignment()
}

// assignment = yield_expr | lambda | or_expr (('=' | aug_op) assignment)?
func (p *Parser) parseAssignment() *ast.Node {
	line := p.line()
	// 先判断 yield 以避免与 or_expr 混淆
	if p.isKind(lexer.TYield) {
		return p.parseYield()
	}
	left := p.parseOrExpr()
	if left == nil {
		return nil
	}
	// 多目标赋值：a, b, c = f()（仅不在逗号分隔上下文内时）
	if left.Kind == ast.NName && p.isRune(',') && p.suppressTuple == 0 {
		lhsList := ast.New(ast.NList, "lhs", line)
		lhsList.Add(left)
		for p.acceptRune(',') {
			nt := p.expect(lexer.TName, "assignment target name after ','")
			if nt == nil {
				return left
			}
			lhsList.Add(ast.New(ast.NName, nt.Text, nt.Line))
		}
		if p.acceptRune('=') {
			rhs := p.parseAssignment()
			multi := ast.New(ast.NMultiAssign, "=", line)
			multi.Add(lhsList)
			if rhs != nil {
				multi.Add(rhs)
			}
			return multi
		}
		p.errorf("expected '=' after multiple assignment targets")
		return left
	}
	// 赋值 / 复合赋值
	cur := p.cur()
	if p.isRune('=') {
		p.advance()
		rhs := p.parseAssignment()
		n := ast.New(ast.NAssign, "=", line)
		n.Add(left)
		n.Add(rhs)
		return n
	}
	op := ""
	switch cur.Kind {
	case lexer.TPlusEq:
		op = "+="
	case lexer.TMinusEq:
		op = "-="
	case lexer.TStarEq:
		op = "*="
	case lexer.TSlashEq:
		op = "/="
	case lexer.TModEq:
		op = "%="
	}
	if op != "" {
		p.advance()
		rhs := p.parseAssignment()
		n := ast.New(ast.NAugAssign, op, line)
		n.Add(left)
		n.Add(rhs)
		return n
	}
	return left
}

func (p *Parser) parseYield() *ast.Node {
	line := p.line()
	p.advance()
	n := ast.New(ast.NYield, "", line)
	if !p.isRune(';') && !p.isRune('}') && !p.isKind(lexer.TEOF) && !p.isRune(')') && !p.isRune(',') {
		e := p.parseExpr()
		n.Add(e)
	}
	return n
}

func (p *Parser) parseLambda() *ast.Node {
	line := p.line()
	p.advance()
	params := ast.New(ast.NList, "params", line)
	// block 形式：lambda(params) { block } 或 lambda { block }
	if p.isRune('{') {
		// 无参 block 形式：lambda { block }
		body := p.parseBlock()
		n := ast.New(ast.NLambda, "block", line)
		n.Add(params)
		n.Add(body)
		return n
	}
	if p.isRune('(') {
		// 可能是 (params) { block } 或 (params) : expr
		p.advance()
		for !p.isRune(')') && !p.isKind(lexer.TEOF) {
			t := p.expect(lexer.TName, "lambda parameter name")
			if t == nil {
				break
			}
			params.Add(ast.New(ast.NName, t.Text, t.Line))
			if !p.acceptRune(',') {
				break
			}
		}
		p.expectRune(')', "')' to close lambda parameter list")
		if p.isRune('{') {
			// (params) { block }
			body := p.parseBlock()
			n := ast.New(ast.NLambda, "block", line)
			n.Add(params)
			n.Add(body)
			return n
		}
		// (params) : expr —— 走原单表达式路径
		p.expectRune(':', "':' in lambda")
		body := p.parseExpr()
		n := ast.New(ast.NLambda, "", line)
		n.Add(params)
		n.Add(body)
		return n
	}
	// 原单表达式形式：lambda params: expr
	if !p.isRune(':') {
		for {
			t := p.expect(lexer.TName, "lambda parameter name")
			if t == nil {
				break
			}
			params.Add(ast.New(ast.NName, t.Text, t.Line))
			if !p.acceptRune(',') {
				break
			}
		}
	}
	p.expectRune(':', "':' in lambda")
	body := p.parseExpr()
	n := ast.New(ast.NLambda, "", line)
	n.Add(params)
	n.Add(body)
	return n
}

// or_expr = and_expr ('or' and_expr)*
func (p *Parser) parseOrExpr() *ast.Node {
	line := p.line()
	left := p.parseAndExpr()
	for p.accept(lexer.TOr) {
		right := p.parseAndExpr()
		n := ast.New(ast.NBinary, "or", line)
		n.Add(left)
		n.Add(right)
		left = n
	}
	return left
}

// and_expr = not_expr ('and' not_expr)*
func (p *Parser) parseAndExpr() *ast.Node {
	line := p.line()
	left := p.parseNotExpr()
	for p.accept(lexer.TAnd) {
		right := p.parseNotExpr()
		n := ast.New(ast.NBinary, "and", line)
		n.Add(left)
		n.Add(right)
		left = n
	}
	return left
}

// not_expr = 'not' not_expr | comparison
func (p *Parser) parseNotExpr() *ast.Node {
	line := p.line()
	if p.accept(lexer.TNot) {
		operand := p.parseNotExpr()
		n := ast.New(ast.NUnary, "not", line)
		n.Add(operand)
		return n
	}
	return p.parseComparison()
}

// comparison = additive (cmp_op additive | 'instanceof' type)*
func (p *Parser) parseComparison() *ast.Node {
	line := p.line()
	left := p.parseAdditive()
	for {
		cur := p.cur()
		op := ""
		switch cur.Kind {
		case lexer.TEq:
			op = "=="
		case lexer.TNe:
			op = "!="
		case lexer.TLe:
			op = "<="
		case lexer.TGe:
			op = ">="
		case lexer.TIs:
			op = "is"
		case lexer.TIn:
			op = "in"
		}
		if cur.Kind == lexer.TokenKind('<') {
			op = "<"
		}
		if cur.Kind == lexer.TokenKind('>') {
			op = ">"
		}
		if op != "" {
			p.advance()
			right := p.parseAdditive()
			n := ast.New(ast.NCompare, op, line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		if cur.Kind == lexer.TInstanceof {
			p.advance()
			typ := p.parseType()
			n := ast.New(ast.NInstanceOf, "", line)
			n.Add(left)
			n.Add(typ)
			left = n
			continue
		}
		break
	}
	return left
}

// additive = multiplicative (('+' | '-') multiplicative)*
func (p *Parser) parseAdditive() *ast.Node {
	line := p.line()
	left := p.parseMultiplicative()
	for {
		if p.isRune('+') {
			p.advance()
			right := p.parseMultiplicative()
			n := ast.New(ast.NBinary, "+", line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		if p.isRune('-') {
			p.advance()
			right := p.parseMultiplicative()
			n := ast.New(ast.NBinary, "-", line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		break
	}
	return left
}

// multiplicative = unary (('*' | '/' | '%') unary)*
func (p *Parser) parseMultiplicative() *ast.Node {
	line := p.line()
	left := p.parseUnary()
	for {
		if p.isRune('*') {
			p.advance()
			right := p.parseUnary()
			n := ast.New(ast.NBinary, "*", line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		if p.isRune('/') {
			p.advance()
			right := p.parseUnary()
			n := ast.New(ast.NBinary, "/", line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		if p.isRune('%') {
			p.advance()
			right := p.parseUnary()
			n := ast.New(ast.NBinary, "%", line)
			n.Add(left)
			n.Add(right)
			left = n
			continue
		}
		break
	}
	return left
}

// unary = ('-' | '+' | '&' | '*' | 'del' | 'await') unary | postfix
func (p *Parser) parseUnary() *ast.Node {
	line := p.line()
	switch p.cur().Kind {
	case lexer.TokenKind('-'):
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "-", line)
		n.Add(operand)
		return n
	case lexer.TokenKind('+'):
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "+", line)
		n.Add(operand)
		return n
	case lexer.TokenKind('&'):
		// &x — 取地址（指针）
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "&", line)
		n.Add(operand)
		return n
	case lexer.TokenKind('*'):
		// *p — 解引用（仅在 unary 位置识别，不影响乘法）
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "*", line)
		n.Add(operand)
		return n
	case lexer.TDel:
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "del", line)
		n.Add(operand)
		return n
	case lexer.TAwait:
		p.advance()
		operand := p.parseUnary()
		n := ast.New(ast.NUnary, "await", line)
		n.Add(operand)
		return n
	}
	return p.parsePostfix()
}

// postfix = primary ('.' attr | '(' args ')' | '[' expr ']')*
//
//	| 'new' NAME '(' args ')'
func (p *Parser) parsePostfix() *ast.Node {
	line := p.line()
	var left *ast.Node
	// new Foo(args)
	if p.isKind(lexer.TNew) {
		p.advance()
		clazz := p.expect(lexer.TName, "class name after 'new'")
		if clazz == nil {
			return nil
		}
		n := ast.New(ast.NNew, clazz.Text, clazz.Line)
		var args *ast.Node
		if p.isRune('(') {
			p.advance()
			args = p.parseArgList()
			p.expectRune(')', "')' after new args")
		} else {
			args = ast.New(ast.NList, "args", line)
		}
		n.Add(args)
		left = n
	} else {
		left = p.parsePrimary()
	}
	for {
		if p.isRune('.') {
			p.advance()
			// 属性名：允许 OOP 关键字作属性名
			attrText := ""
			switch p.cur().Kind {
			case lexer.TName, lexer.TNew, lexer.TExtends, lexer.TImplements,
				lexer.TAbstract, lexer.TInterface, lexer.TPrivate,
				lexer.TProtected, lexer.TStatic, lexer.TSuper, lexer.TInstanceof,
				lexer.TVar, lexer.TGo:
				if p.cur().Kind == lexer.TName {
					attrText = p.cur().Text
				} else {
					attrText = tokenKeywordAsAttr(p.cur().Kind)
				}
				attrLine := p.line()
				p.advance()
				n := ast.New(ast.NMember, ".", attrLine)
				n.Add(left)
				n.Add(ast.New(ast.NName, attrText, attrLine))
				left = n
			default:
				p.errorf("expected attribute name after '.'")
				break
			}
			continue
		}
		if p.isRune('(') {
			p.advance()
			args := p.parseArgList()
			p.expectRune(')', "')' after call args")
			n := ast.New(ast.NCall, "", line)
			n.Add(left)
			n.MoveChildren(args)
			left = n
			continue
		}
		if p.isRune('[') {
			p.advance()
			idx := p.parseExpr()
			p.expectRune(']', "']' after index")
			n := ast.New(ast.NIndex, "[]", line)
			n.Add(left)
			n.Add(idx)
			left = n
			continue
		}
		break
	}
	return left
}

func tokenKeywordAsAttr(k lexer.TokenKind) string {
	switch k {
	case lexer.TNew:
		return "new"
	case lexer.TExtends:
		return "extends"
	case lexer.TImplements:
		return "implements"
	case lexer.TAbstract:
		return "abstract"
	case lexer.TInterface:
		return "interface"
	case lexer.TPrivate:
		return "private"
	case lexer.TProtected:
		return "protected"
	case lexer.TStatic:
		return "static"
	case lexer.TSuper:
		return "super"
	case lexer.TInstanceof:
		return "instanceof"
	case lexer.TVar:
		return "var"
	}
	return ""
}

func (p *Parser) parseArgList() *ast.Node {
	line := p.line()
	list := ast.New(ast.NList, "args", line)
	if p.isRune(')') {
		return list
	}
	p.suppressTuple++
	for {
		e := p.parseExpr()
		if e == nil {
			break
		}
		list.Add(e)
		if !p.acceptRune(',') {
			break
		}
	}
	p.suppressTuple--
	return list
}

// primary
func (p *Parser) parsePrimary() *ast.Node {
	t := p.cur()
	switch t.Kind {
	case lexer.TInt:
		p.advance()
		return ast.NewInt(t.IVal, t.Line)
	case lexer.TFloat:
		p.advance()
		return ast.NewFloat(t.FVal, t.Line)
	case lexer.TString:
		p.advance()
		return ast.New(ast.NString, t.Text, t.Line)
	case lexer.TTrue:
		p.advance()
		return ast.NewBool(true, t.Line)
	case lexer.TLambda:
		// lambda 在 primary 位置识别，使其后可跟 postfix（立即调用等）
		return p.parseLambda()
	case lexer.TFalse:
		p.advance()
		return ast.NewBool(false, t.Line)
	case lexer.TNull:
		p.advance()
		return ast.New(ast.NNull, "", t.Line)
	case lexer.TThis:
		p.advance()
		return ast.New(ast.NThis, "", t.Line)
	case lexer.TSuper:
		p.advance()
		return ast.New(ast.NSuper, "", t.Line)
	case lexer.TName:
		p.advance()
		return ast.New(ast.NName, t.Text, t.Line)
	}
	if p.isRune('(') {
		p.advance()
		e := p.parseExpr()
		p.expectRune(')', "')' to close parenthesized expr")
		return e
	}
	if p.isRune('[') {
		line := p.line()
		p.advance()
		args := ast.New(ast.NList, "", line)
		if !p.isRune(']') {
			p.suppressTuple++
			for {
				e := p.parseExpr()
				if e == nil {
					break
				}
				args.Add(e)
				if !p.acceptRune(',') {
					break
				}
			}
			p.suppressTuple--
		}
		p.expectRune(']', "']' to close list literal")
		listNode := ast.New(ast.NListExpr, "", line)
		listNode.MoveChildren(args)
		return listNode
	}
	p.errorf("unexpected token in expression: %s", tokenStr(t))
	return nil
}

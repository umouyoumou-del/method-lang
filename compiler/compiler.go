// Package compiler —— Method 语言编译器：AST → MLR 字节码
package compiler

import (
	"encoding/binary"
	"fmt"
	"os"
	"strings"

	"method/ast"
	"method/bytecode"
)

// Builder —— 字节码构建器
type Builder struct {
	code             []byte
	labels           map[string]int
	forwardRefs      map[string][]int // label name -> list of code offsets to patch (jump offset field position)
	methodBlockStart uint32
	exports          []bytecode.Export
	pseudo           []byte
	pendingClass     *pendingClass
	strTable         []string
	strIndex         map[string]int
}

type pendingClass struct {
	name     string
	parentID int32
	ifaces   []int32
	fields   []pendingField
	methods  []pendingMethod
}
type pendingField struct {
	name     string
	isStatic bool
}
type pendingMethod struct {
	name      string
	off       int32
	numParams uint8
	numLocals uint8
	isStatic  bool
}

// NewBuilder 创建空的构建器
func NewBuilder() *Builder {
	b := &Builder{
		labels:      map[string]int{},
		forwardRefs: map[string][]int{},
		strTable:    []string{""},
		strIndex:    map[string]int{"": 0},
	}
	return b
}

// InternString 把字符串加入字符串表，返回索引（>=1，0 永远是空串）
func (b *Builder) InternString(s string) int32 {
	if i, ok := b.strIndex[s]; ok {
		return int32(i)
	}
	i := len(b.strTable)
	b.strTable = append(b.strTable, s)
	b.strIndex[s] = i
	return int32(i)
}

// StringTable 返回构建好的字符串表（按 build() 顺序）
func (b *Builder) StringTable() []string { return b.strTable }

// === Code emission primitives ===

func (b *Builder) emit(op bytecode.Op) { b.code = append(b.code, byte(op)) }
func (b *Builder) emitU8(v uint8)      { b.code = append(b.code, v) }
func (b *Builder) emitU16(v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	b.code = append(b.code, buf[:]...)
}
func (b *Builder) emitI64(v int64) {
	var buf [8]byte
	binary.LittleEndian.PutUint64(buf[:], uint64(v))
	b.code = append(b.code, buf[:]...)
}
func (b *Builder) emitI32(v int32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	b.code = append(b.code, buf[:]...)
}

func (b *Builder) patchOffset(at int, off int32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(off))
	copy(b.code[at:at+4], buf[:])
}

// === Opcode wrappers ===

func (b *Builder) Nop()                  { b.emit(bytecode.OpNop) }
func (b *Builder) PushI64(v int64)       { b.emit(bytecode.OpPushI64); b.emitI64(v) }
func (b *Builder) Pop()                  { b.emit(bytecode.OpPop) }
func (b *Builder) Dup()                  { b.emit(bytecode.OpDup) }
func (b *Builder) Swap()                 { b.emit(bytecode.OpSwap) }
func (b *Builder) LoadLocal(slot uint8)  { b.emit(bytecode.OpLoadLocal); b.emitU8(slot) }
func (b *Builder) StoreLocal(slot uint8) { b.emit(bytecode.OpStoreLocal); b.emitU8(slot) }
func (b *Builder) AddLocalLocal(a, bb uint8) {
	b.emit(bytecode.OpAddLocalLocal)
	b.emitU8(a)
	b.emitU8(bb)
}
func (b *Builder) IncLocalImm8(slot uint8, imm int8) {
	b.emit(bytecode.OpIncLocalImm8)
	b.emitU8(slot)
	b.emitU8(uint8(imm))
}
func (b *Builder) AddI64()    { b.emit(bytecode.OpAddI64) }
func (b *Builder) SubI64()    { b.emit(bytecode.OpSubI64) }
func (b *Builder) MulI64()    { b.emit(bytecode.OpMulI64) }
func (b *Builder) DivI64()    { b.emit(bytecode.OpDivI64) }
func (b *Builder) ModI64()    { b.emit(bytecode.OpModI64) }
func (b *Builder) NegI64()    { b.emit(bytecode.OpNegI64) }
func (b *Builder) PrintI64()  { b.emit(bytecode.OpPrintI64) }
func (b *Builder) PrintChar() { b.emit(bytecode.OpPrintChar) }
func (b *Builder) Halt()      { b.emit(bytecode.OpHalt) }

func (b *Builder) Jmp(off int32) { b.emit(bytecode.OpJmp); b.emitI32(off) }
func (b *Builder) Jz(off int32)  { b.emit(bytecode.OpJz); b.emitI32(off) }
func (b *Builder) Jnz(off int32) { b.emit(bytecode.OpJnz); b.emitI32(off) }

func (b *Builder) CmpEq() { b.emit(bytecode.OpCmpEq) }
func (b *Builder) CmpLt() { b.emit(bytecode.OpCmpLt) }
func (b *Builder) CmpGt() { b.emit(bytecode.OpCmpGt) }
func (b *Builder) CmpLe() { b.emit(bytecode.OpCmpLe) }
func (b *Builder) CmpGe() { b.emit(bytecode.OpCmpGe) }
func (b *Builder) CmpNe() { b.emit(bytecode.OpCmpNe) }

func (b *Builder) Call(off int32) { b.emit(bytecode.OpCall); b.emitI32(off) }
func (b *Builder) Ret()           { b.emit(bytecode.OpRet) }

func (b *Builder) StrNew()     { b.emit(bytecode.OpStrNew) }
func (b *Builder) StrAppendC() { b.emit(bytecode.OpStrAppendC) }
func (b *Builder) StrLen()     { b.emit(bytecode.OpStrLen) }
func (b *Builder) StrGetC()    { b.emit(bytecode.OpStrGetC) }
func (b *Builder) StrDelete()  { b.emit(bytecode.OpStrDelete) }
func (b *Builder) PrintStr()   { b.emit(bytecode.OpPrintStr) }

func (b *Builder) NewObj()      { b.emit(bytecode.OpNewObj) }
func (b *Builder) GetAttr()     { b.emit(bytecode.OpGetAttr) }
func (b *Builder) SetAttr()     { b.emit(bytecode.OpSetAttr) }
func (b *Builder) Invoke()      { b.emit(bytecode.OpInvoke) }
func (b *Builder) InstanceOf()  { b.emit(bytecode.OpInstanceOf) }
func (b *Builder) GetStatic()   { b.emit(bytecode.OpGetStatic) }
func (b *Builder) SetStatic()   { b.emit(bytecode.OpSetStatic) }
func (b *Builder) InvokeSuper() { b.emit(bytecode.OpInvokeSuper) }
func (b *Builder) ObjRelease()  { b.emit(bytecode.OpObjRelease) }

func (b *Builder) Go()      { b.emit(bytecode.OpGo) }
func (b *Builder) ChanNew() { b.emit(bytecode.OpChanNew) }
func (b *Builder) ChanPut() { b.emit(bytecode.OpChanPut) }
func (b *Builder) ChanGet() { b.emit(bytecode.OpChanGet) }

func (b *Builder) SystemExec()     { b.emit(bytecode.OpSystemExec) }
func (b *Builder) SystemReadFile() { b.emit(bytecode.OpSystemReadFile) }

// ===== 容器指令 =====
func (b *Builder) ListNew()      { b.emit(bytecode.OpListNew) }
func (b *Builder) ListPush()     { b.emit(bytecode.OpListPush) }
func (b *Builder) ListGet()      { b.emit(bytecode.OpListGet) }
func (b *Builder) ListSet()      { b.emit(bytecode.OpListSet) }
func (b *Builder) ListPop()      { b.emit(bytecode.OpListPop) }
func (b *Builder) ListLen()      { b.emit(bytecode.OpListLen) }
func (b *Builder) ListDeleteAt() { b.emit(bytecode.OpListDeleteAt) }
func (b *Builder) ListRelease()  { b.emit(bytecode.OpListRelease) }

func (b *Builder) DictNew()     { b.emit(bytecode.OpDictNew) }
func (b *Builder) DictPut()     { b.emit(bytecode.OpDictPut) }
func (b *Builder) DictGet()     { b.emit(bytecode.OpDictGet) }
func (b *Builder) DictHas()     { b.emit(bytecode.OpDictHas) }
func (b *Builder) DictDelete()  { b.emit(bytecode.OpDictDelete) }
func (b *Builder) DictLen()     { b.emit(bytecode.OpDictLen) }
func (b *Builder) DictRelease() { b.emit(bytecode.OpDictRelease) }

// ===== 字符串增强 =====
func (b *Builder) StrFind()       { b.emit(bytecode.OpStrFind) }
func (b *Builder) StrSlice()      { b.emit(bytecode.OpStrSlice) }
func (b *Builder) StrEqual()      { b.emit(bytecode.OpStrEqual) }
func (b *Builder) StrNewFromIdx() { b.emit(bytecode.OpStrNewFromIdx) }
func (b *Builder) StrTrim()       { b.emit(bytecode.OpStrTrim) }
func (b *Builder) StrReplace()    { b.emit(bytecode.OpStrReplace) }
func (b *Builder) StrAppendStr()  { b.emit(bytecode.OpStrAppendStr) }

// ===== 类型转换 / 时间 =====
func (b *Builder) Atoi()  { b.emit(bytecode.OpAtoi) }
func (b *Builder) Itoa()  { b.emit(bytecode.OpItoa) }
func (b *Builder) Sleep() { b.emit(bytecode.OpSleep) }
func (b *Builder) Now()   { b.emit(bytecode.OpNow) }

// ===== HTTP =====
func (b *Builder) HttpRequest()   { b.emit(bytecode.OpHttpRequest) }
func (b *Builder) HttpGetCookie() { b.emit(bytecode.OpHttpGetCookie) }
func (b *Builder) HttpClear()     { b.emit(bytecode.OpHttpClear) }
func (b *Builder) HttpSetUA()     { b.emit(bytecode.OpHttpSetUA) }
func (b *Builder) HttpAddHdr()    { b.emit(bytecode.OpHttpAddHdr) }

// === Label / jump support ===

// CodeSize 返回当前已生成字节码长度
func (b *Builder) CodeSize() int { return len(b.code) }

// Label 在当前偏移定义一个标签
func (b *Builder) Label(name string) {
	off := len(b.code)
	b.labels[name] = off
	// 回填前向引用
	if refs, ok := b.forwardRefs[name]; ok {
		for _, at := range refs {
			// 跳转指令结构：[op(1)] [i32 offset(4)]，offset 字段位置=at
			// offset = target - (at + 4) 即 target = 指令末尾之后的位置
			patchVal := int32(off - (at + 4))
			b.patchOffset(at, patchVal)
		}
		delete(b.forwardRefs, name)
	}
}

// LabelAt 在指定偏移处定义标签
func (b *Builder) LabelAt(name string, offset int) { b.labels[name] = offset }

// emitJumpTo 写一个带标签名的跳转（支持前向/后向）。通用工具。
func (b *Builder) emitJumpTo(op bytecode.Op, name string) {
	if off, ok := b.labels[name]; ok {
		// 后向
		b.emit(op)
		at := len(b.code) // offset field 位置
		// patch = target - (at + 4)
		b.emitI32(int32(off - (at + 4)))
		return
	}
	b.emit(op)
	at := len(b.code) // offset field 位置：占位 0，等 label() 回填
	b.emitI32(0)
	b.forwardRefs[name] = append(b.forwardRefs[name], at)
}

func (b *Builder) JmpTo(name string) { b.emitJumpTo(bytecode.OpJmp, name) }
func (b *Builder) JzTo(name string)  { b.emitJumpTo(bytecode.OpJz, name) }
func (b *Builder) JnzTo(name string) { b.emitJumpTo(bytecode.OpJnz, name) }

// 异常处理：PushHandlerTo 支持前向标签回填（布局 [op][i32] 与跳转一致）
func (b *Builder) PushHandlerTo(name string) { b.emitJumpTo(bytecode.OpPushHandler, name) }
func (b *Builder) PopHandler()               { b.emit(bytecode.OpPopHandler) }
func (b *Builder) Raise()                    { b.emit(bytecode.OpRaise) }

func (b *Builder) SetMethodBlockStart(off uint32) { b.methodBlockStart = off }
func (b *Builder) MethodBlockStart() uint32       { return b.methodBlockStart }

func (b *Builder) AddExport(name string, off int32, numLocals, numParams uint8) {
	b.exports = append(b.exports, bytecode.Export{
		Name: name, CodeOffset: off, NumLocals: numLocals, NumParams: numParams,
	})
}

// === Pseudo segment: ClassMeta ===

func (b *Builder) pseudoWriteU16(v uint16) {
	var buf [2]byte
	binary.LittleEndian.PutUint16(buf[:], v)
	b.pseudo = append(b.pseudo, buf[:]...)
}
func (b *Builder) pseudoWriteI32(v int32) {
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(v))
	b.pseudo = append(b.pseudo, buf[:]...)
}
func (b *Builder) pseudoWriteBytes(p []byte) {
	b.pseudo = append(b.pseudo, p...)
}
func (b *Builder) pseudoWriteStr(s string) {
	b.pseudoWriteI32(int32(len(s)))
	b.pseudoWriteBytes([]byte(s))
}

// PseudoBeginClass 开始写一个 ClassMeta
func (b *Builder) PseudoBeginClass(name string, parentID int32) {
	b.pendingClass = &pendingClass{name: name, parentID: parentID}
}
func (b *Builder) PseudoAddInterface(id int32) {
	b.pendingClass.ifaces = append(b.pendingClass.ifaces, id)
}
func (b *Builder) PseudoAddField(name string, isStatic bool) {
	b.pendingClass.fields = append(b.pendingClass.fields, pendingField{name, isStatic})
}
func (b *Builder) PseudoAddMethod(name string, off int32, np, nl uint8, isStatic bool) {
	b.pendingClass.methods = append(b.pendingClass.methods, pendingMethod{name, off, np, nl, isStatic})
}
func (b *Builder) PseudoEndClass() {
	if b.pendingClass == nil {
		return
	}
	pc := b.pendingClass
	b.pseudoWriteU16(bytecode.PseudoTagClassMeta)
	b.pseudoWriteStr(pc.name)
	b.pseudoWriteI32(pc.parentID)
	// interfaces count + ids
	b.pseudoWriteI32(int32(len(pc.ifaces)))
	for _, id := range pc.ifaces {
		b.pseudoWriteI32(id)
	}
	// fields count
	b.pseudoWriteI32(int32(len(pc.fields)))
	for _, f := range pc.fields {
		b.pseudoWriteStr(f.name)
		flags := uint8(0)
		if f.isStatic {
			flags |= 1
		}
		b.pseudo = append(b.pseudo, flags)
	}
	// methods count
	b.pseudoWriteI32(int32(len(pc.methods)))
	for _, m := range pc.methods {
		b.pseudoWriteStr(m.name)
		b.pseudoWriteI32(m.off)
		b.pseudo = append(b.pseudo, m.numParams, m.numLocals)
		flags := uint8(0)
		if m.isStatic {
			flags |= 1
		}
		b.pseudo = append(b.pseudo, flags)
	}
	b.pendingClass = nil
}
func (b *Builder) PseudoFinalize() {
	b.pseudoWriteU16(bytecode.PseudoTagEnd)
}
func (b *Builder) Pseudo() []byte { return b.pseudo }

// === Final build ===

// Build 组装完整的 Program
func (b *Builder) Build() *bytecode.Program {
	return &bytecode.Program{
		Valid:            true,
		Version:          bytecode.BytecodeVersion,
		Code:             append([]byte(nil), b.code...),
		Pseudo:           append([]byte(nil), b.pseudo...),
		MethodBlockStart: b.methodBlockStart,
		Exports:          append([]bytecode.Export(nil), b.exports...),
		StringTable:      append([]string(nil), b.strTable...),
	}
}

// SaveToFile 把 Build() 结果保存为 .mlr 文件
func (b *Builder) SaveToFile(path string) error {
	p := b.Build()
	data := p.Serialize()
	return os.WriteFile(path, data, 0644)
}

// ============================================================================
//  Compiler —— AST → Builder → Program
// ============================================================================

// Compiler 包含编译上下文
type Compiler struct {
	builder *Builder
	// 顶层方法
	methods      []*methodInfo
	methodLabels map[string]string // callable name → label
	methodParams map[string]int    // label → num params
	// 类
	classes       []*classInfo
	classNameToID map[string]int
	// 编译当前上下文
	currentLocals map[string]uint8 // 当前方法/函数体内名字→slot
	nextSlot      uint8
	numParams     int
	insideMethod  bool
	warnings      []string
	errors        []string
}

type methodInfo struct {
	name       string
	label      string
	numParams  int
	paramNames []string
	node       *ast.Node // N_METHOD body node (Children[1] = block)
}

type classInfo struct {
	name        string
	id          int
	isInterface bool
	parentName  string
	ifaceNames  []string
	parentID    int
	ifaceIDs    []int
	fields      []classField
	methods     []classMethod
	ast         *ast.Node
	modifiers   int
}
type classField struct {
	name     string
	isStatic bool
	initExpr *ast.Node
}
type classMethod struct {
	name        string
	label       string
	isStatic    bool
	isAbstract  bool
	numParams   int
	paramNames  []string
	body        *ast.Node
	astNode     *ast.Node
	numLocals   uint8
	entryOffset int32
}

// NewCompiler 创建编译器
func NewCompiler() *Compiler {
	return &Compiler{
		builder:       NewBuilder(),
		methodLabels:  map[string]string{},
		methodParams:  map[string]int{},
		classNameToID: map[string]int{},
	}
}

func (c *Compiler) Warnings() []string { return c.warnings }
func (c *Compiler) Errors() []string   { return c.errors }
func (c *Compiler) warnf(f string, a ...any) {
	c.warnings = append(c.warnings, fmt.Sprintf(f, a...))
}
func (c *Compiler) errorf(f string, a ...any) {
	c.errors = append(c.errors, fmt.Sprintf(f, a...))
}

// Compile 编译 Program AST 并返回 Program
func (c *Compiler) Compile(prog *ast.Node) *bytecode.Program {
	if prog == nil {
		return nil
	}
	// Pass -1: 扫描类/接口
	c.scanClasses(prog)
	c.resolveClassParents()
	// Pass -0.5: 字段初始值准备——带初始值实例字段但无构造函数的类，合成 init
	c.prepareFieldInitializers()

	// Pass 0: 收集顶层方法
	for _, s := range prog.Children {
		if s != nil && s.Kind == ast.NMethod {
			c.collectMethod(s)
		}
	}
	// Pass 0b: 注册类方法标签
	for _, cl := range c.classes {
		for _, m := range cl.methods {
			callName := cl.name + "." + m.name
			c.methodLabels[callName] = m.label
			c.methodParams[m.label] = m.numParams
			if _, ok := c.methodLabels[m.name]; !ok {
				c.methodLabels[m.name] = m.label
			}
		}
	}
	// Pass 1a: 静态字段初始值（先于所有顶层语句，语义同 Java 静态初始化）
	c.emitStaticFieldInitializers()

	// Pass 1: 顶层语句（非方法/类/接口）
	for _, s := range prog.Children {
		if s == nil {
			continue
		}
		if s.Kind == ast.NMethod || s.Kind == ast.NClass || s.Kind == ast.NInterface || s.Kind == ast.NImport {
			continue
		}
		c.compileStmt(s)
	}
	c.builder.Halt()
	c.builder.SetMethodBlockStart(uint32(c.builder.CodeSize()))

	// Pass 2: 编译顶层方法体
	for _, m := range c.methods {
		c.compileMethodBody(m)
	}

	// Pass 2b: 编译类方法体
	for _, cl := range c.classes {
		for i := range cl.methods {
			m := &cl.methods[i]
			if m.isAbstract || m.body == nil {
				continue
			}
			c.compileClassMethodBody(cl, m)
		}
	}

	// Pass 3: 写 Pseudo 段
	c.writePseudoSegment()

	return c.builder.Build()
}

// ===== Class scanning =====

func (c *Compiler) scanClasses(n *ast.Node) {
	if n == nil {
		return
	}
	if n.Kind == ast.NClass {
		cl := &classInfo{ast: n, modifiers: n.IsPublic, name: n.Text}
		// child 0: extends (N_NAME or N_LIST)
		if len(n.Children) >= 1 && n.Children[0] != nil {
			ext := n.Children[0]
			if ext.Kind == ast.NName {
				cl.parentName = ext.Text
			} else if ext.Kind == ast.NList {
				if len(ext.Children) >= 1 && ext.Children[0].Kind == ast.NName {
					cl.parentName = ext.Children[0].Text
				}
			}
		}
		// child 1: implements (N_LIST)
		if len(n.Children) >= 2 && n.Children[1] != nil && n.Children[1].Kind == ast.NList {
			for _, nm := range n.Children[1].Children {
				if nm != nil && nm.Kind == ast.NName {
					cl.ifaceNames = append(cl.ifaceNames, nm.Text)
				}
			}
		}
		// child 3: body (child 2 = params)
		if len(n.Children) >= 4 && n.Children[3] != nil {
			c.collectClassMembers(cl, n.Children[3])
		}
		c.classes = append(c.classes, cl)
	} else if n.Kind == ast.NInterface {
		cl := &classInfo{isInterface: true, ast: n, modifiers: n.IsPublic, name: n.Text}
		if len(n.Children) >= 1 && n.Children[0] != nil {
			ext := n.Children[0]
			if ext.Kind == ast.NName {
				cl.parentName = ext.Text
			} else if ext.Kind == ast.NList {
				for i, nm := range ext.Children {
					if nm != nil && nm.Kind == ast.NName {
						if i == 0 {
							cl.parentName = nm.Text
						} else {
							cl.ifaceNames = append(cl.ifaceNames, nm.Text)
						}
					}
				}
			}
		}
		if len(n.Children) >= 2 && n.Children[1] != nil {
			c.collectClassMembers(cl, n.Children[1])
		}
		c.classes = append(c.classes, cl)
	}
	for _, ch := range n.Children {
		c.scanClasses(ch)
	}
}

func (c *Compiler) collectClassMembers(cl *classInfo, body *ast.Node) {
	if body == nil {
		return
	}
	for _, m := range body.Children {
		if m == nil {
			continue
		}
		if m.Kind == ast.NField {
			f := classField{name: m.Text}
			mods := m.IsPublic
			f.isStatic = (mods>>2)&1 != 0
			// 布局：Children[0]=类型（或空 NList 占位），Children[1]=初始值（或占位）
			if len(m.Children) >= 2 && m.Children[1] != nil &&
				!(m.Children[1].Kind == ast.NList && len(m.Children[1].Children) == 0) {
				f.initExpr = m.Children[1]
			}
			cl.fields = append(cl.fields, f)
		} else if m.Kind == ast.NMethod {
			mm := classMethod{
				name:    m.Text,
				astNode: m,
			}
			mods := m.IsPublic
			mm.isStatic = (mods>>2)&1 != 0
			mm.isAbstract = (mods>>3)&1 != 0
			if len(m.Children) >= 1 && m.Children[0] != nil {
				params := m.Children[0]
				for _, p := range params.Children {
					if p != nil && p.Kind == ast.NParam {
						mm.paramNames = append(mm.paramNames, p.Text)
					}
				}
			}
			mm.numParams = len(mm.paramNames)
			if len(m.Children) >= 2 {
				mm.body = m.Children[1]
			}
			if mm.body == nil {
				mm.isAbstract = true
			}
			mm.label = "C_" + cl.name + "_" + mm.name
			cl.methods = append(cl.methods, mm)
		}
	}
}

func (c *Compiler) resolveClassParents() {
	for _, cl := range c.classes {
		cl.id = len(c.classNameToID)
		c.classNameToID[cl.name] = cl.id
	}
	for _, cl := range c.classes {
		cl.parentID = -1 // 无父类默认 -1（0 是合法类 ID）
		if cl.parentName != "" {
			if id, ok := c.classNameToID[cl.parentName]; ok {
				cl.parentID = id
			}
		}
		for _, nm := range cl.ifaceNames {
			if id, ok := c.classNameToID[nm]; ok {
				cl.ifaceIDs = append(cl.ifaceIDs, id)
			}
		}
	}
}

// prepareFieldInitializers 字段初始值准备：
// 若类需要运行实例字段初始值（自身或任一祖先带初始值实例字段）但没有显式
// 构造函数（init 或与类同名的方法），合成一个空的 init 方法——Pass 2b 编译
// 它时注入字段初始化代码，VM 侧 opNewObj 通过 InitOffset 调用。
func (c *Compiler) prepareFieldInitializers() {
	for _, cl := range c.classes {
		if cl.isInterface {
			continue
		}
		if !c.classNeedsFieldInitRun(cl) {
			continue
		}
		hasCtor := false
		for i := range cl.methods {
			if cl.methods[i].name == "init" || cl.methods[i].name == cl.name {
				hasCtor = true
				break
			}
		}
		if !hasCtor {
			cl.methods = append(cl.methods, classMethod{
				name:  "init",
				label: "C_" + cl.name + "_init",
				body:  ast.New(ast.NBlock, "synth_init", 0),
			})
		}
	}
}

// classNeedsFieldInitRun 判断该类（含祖先链）是否存在带初始值的实例字段。
// 深度上限防继承环。
func (c *Compiler) classNeedsFieldInitRun(cl *classInfo) bool {
	for cur := cl; cur != nil && !cur.isInterface; cur = c.parentClassOf(cur) {
		for _, f := range cur.fields {
			if !f.isStatic && f.initExpr != nil {
				return true
			}
		}
	}
	return false
}

// parentClassOf 返回父类 info（parentID 即 classes 下标），无父类返回 nil
func (c *Compiler) parentClassOf(cl *classInfo) *classInfo {
	if cl.parentID >= 0 && cl.parentID < len(c.classes) {
		return c.classes[cl.parentID]
	}
	return nil
}

// emitStaticFieldInitializers 在顶层代码开头注入静态字段初始值赋值：
//
//	PushI64(classID); PushI64(attrIdx); <initExpr>; SetStatic（弹 3 压 0）
func (c *Compiler) emitStaticFieldInitializers() {
	for _, cl := range c.classes {
		if cl.isInterface {
			continue
		}
		for _, f := range cl.fields {
			if !f.isStatic || f.initExpr == nil {
				continue
			}
			c.builder.PushI64(int64(cl.id))
			aidx := c.builder.InternString(f.name)
			c.builder.PushI64(int64(aidx))
			c.compileExpr(f.initExpr)
			c.builder.SetStatic()
		}
	}
}

// emitCtorFieldInitializers 在构造函数开头注入实例字段初始值：
// 从根类到本类依次内联各祖先的实例字段初始值（父先子后），
// 再执行构造函数体（无隐式 super 调用，仅保证字段初始值语义）。
func (c *Compiler) emitCtorFieldInitializers(cl *classInfo) {
	var chain []*classInfo
	for cur := cl; cur != nil && !cur.isInterface; cur = c.parentClassOf(cur) {
		chain = append(chain, cur)
		if len(chain) > 256 {
			break // 继承环防护
		}
	}
	for i := len(chain) - 1; i >= 0; i-- {
		c.emitInstanceFieldInitializers(chain[i])
	}
}

// emitInstanceFieldInitializers 注入单个类的实例字段初始值：
//
//	LoadLocal(0); PushI64(attrIdx); <initExpr>; SetAttr（弹 3 压 1）; Pop
//
// 非静态方法 slot 0 = this。
func (c *Compiler) emitInstanceFieldInitializers(cl *classInfo) {
	for _, f := range cl.fields {
		if f.isStatic || f.initExpr == nil {
			continue
		}
		c.builder.LoadLocal(0) // this
		aidx := c.builder.InternString(f.name)
		c.builder.PushI64(int64(aidx))
		c.compileExpr(f.initExpr)
		c.builder.SetAttr()
		c.builder.Pop()
	}
}

func (c *Compiler) writePseudoSegment() {
	for _, cl := range c.classes {
		c.builder.PseudoBeginClass(cl.name, int32(cl.parentID))
		for _, id := range cl.ifaceIDs {
			c.builder.PseudoAddInterface(int32(id))
		}
		for _, f := range cl.fields {
			c.builder.PseudoAddField(f.name, f.isStatic)
		}
		for _, m := range cl.methods {
			nl := m.numLocals
			if nl == 0 {
				nl = uint8(m.numParams + 8)
				if !m.isStatic {
					nl++ // this
				}
				if nl > 255 {
					nl = 255
				}
			}
			c.builder.PseudoAddMethod(m.name, m.entryOffset, uint8(m.numParams), nl, m.isStatic)
		}
		c.builder.PseudoEndClass()
	}
	c.builder.PseudoFinalize()
}

// ===== Method collection =====

func (c *Compiler) collectMethod(n *ast.Node) {
	name := n.Text
	info := &methodInfo{
		name:  name,
		label: "M_" + name,
		node:  n,
	}
	if len(n.Children) >= 1 && n.Children[0] != nil {
		params := n.Children[0]
		for _, p := range params.Children {
			if p != nil && p.Kind == ast.NParam {
				info.paramNames = append(info.paramNames, p.Text)
			}
		}
	}
	info.numParams = len(info.paramNames)
	c.methods = append(c.methods, info)
	c.methodLabels[name] = info.label
	c.methodParams[info.label] = info.numParams
}

// ===== Stmt compilation =====

func (c *Compiler) compileStmt(n *ast.Node) {
	if n == nil {
		return
	}
	switch n.Kind {
	case ast.NBlock:
		for _, s := range n.Children {
			c.compileStmt(s)
		}
	case ast.NIf:
		c.compileIf(n)
	case ast.NWhile:
		c.compileWhile(n)
	case ast.NForC:
		c.compileForC(n)
	case ast.NForIn:
		c.compileForIn(n)
	case ast.NReturn:
		c.compileReturn(n)
	case ast.NBreak:
		// 需要 breakLabel 栈，这里简单化：出警告
		c.warnf("break not fully supported outside explicit loop label context")
	case ast.NContinue:
		c.warnf("continue not fully supported outside explicit loop label context")
	case ast.NPass:
		// no-op
	case ast.NAssert:
		if len(n.Children) >= 1 {
			c.compileExpr(n.Children[0])
			// 断言失败时打印 -1 并 halt
			endLabel := freshLabel("assert_end")
			c.builder.JnzTo(endLabel)
			c.builder.PushI64(-1)
			c.builder.PrintI64()
			c.builder.Halt()
			c.builder.Label(endLabel)
		}
	case ast.NExprStmt:
		if len(n.Children) >= 1 {
			c.compileExpr(n.Children[0])
			// 弹出表达式语句的值（丢弃）
			c.builder.Pop()
		}
	case ast.NGo:
		c.compileGoStmt(n)
	case ast.NAssign:
		c.compileAssign(n)
	case ast.NAugAssign:
		c.compileAugAssign(n)
	case ast.NTry:
		c.compileTry(n)
	case ast.NRaise:
		c.compileRaise(n)
	case ast.NGlobal, ast.NNonlocal, ast.NWith,
		ast.NImport, ast.NClass, ast.NInterface, ast.NMethod:
		// 已在更外层处理，或暂未实现
	default:
		// 纯表达式语句
		c.compileExpr(n)
		c.builder.Pop()
	}
}

// compileGoStmt 编译 go 语句：
//
//	go methodName(arg1, ...);
//
// 栈序（底→顶）：arg0..argN-1, argc, method_str_idx（顶）
// opGo 弹出 method_str_idx → argc → args，查 Exports 起独立线程执行。
func (c *Compiler) compileGoStmt(n *ast.Node) {
	nameNode := nth(n, 0)
	argsNode := nth(n, 1)
	if nameNode == nil {
		c.warnf("go statement missing method name")
		return
	}
	var args []*ast.Node
	if argsNode != nil {
		args = argsNode.Children
	}
	for _, a := range args {
		c.compileExpr(a)
	}
	c.builder.PushI64(int64(len(args)))
	midx := c.builder.InternString(nameNode.Text)
	c.builder.PushI64(int64(midx))
	c.builder.Go()
	// go 语句无返回值，不压占位（compileStmt 的 NGo 分支不做语句尾 Pop）
}

// compileRaise 编译 raise 语句：
//
//	raise "msg";    → [msg_str_idx] OpRaise
//	raise;          → [空串] OpRaise
func (c *Compiler) compileRaise(n *ast.Node) {
	if len(n.Children) >= 1 && n.Children[0] != nil {
		c.compileExpr(n.Children[0])
	} else {
		c.builder.PushI64(0) // 空串 str_idx
	}
	c.builder.Raise()
}

// compileTry 编译 try/except/finally。
//
// 异常模型：异常 = 消息字符串（str_idx 压栈传给 handler）。
// except 不区分类型——第一个 except 捕获全部异常（其余 except 不可达，告警）；
// finally 在正常路径与异常路径各内联一份，异常路径执行完后重抛给外层。
//
// 生成布局（hasFinally 时）：
//
//	PushHandler H1          // H1 = 首个 except；无 except 则 = FE
//	<try body>
//	PopHandler
//	Jmp FN                  // 正常路径 → finally
//	H1:                     // 进入时栈顶 = msg str_idx
//	  [PushHandler FE]      // except 体内再抛异常时由 finally 兜底
//	  StoreLocal e | Pop    // as 名绑定消息 / 丢弃
//	  <except body>
//	  [PopHandler; Jmp FN]
//	FE:                     // 异常路径 finally（栈顶 = msg）
//	  StoreLocal $fin_exc   // 暂存消息到隐藏局部
//	  <finally body>        // 副本 1
//	  LoadLocal $fin_exc
//	  Raise                 // 重抛（本层 handler 已弹出 → 向外层传播）
//	FN:
//	  <finally body>        // 副本 2
//	  Jmp END
//	END:
func (c *Compiler) compileTry(n *ast.Node) {
	tryBlock := nth(n, 0)
	var excepts []*ast.Node // NBlock（Text=类型名, Children=[asName?, body]）
	var finallyBlock *ast.Node
	for i := 1; i < len(n.Children); i++ {
		ch := n.Children[i]
		if ch == nil {
			continue
		}
		if ch.Kind == ast.NFinally {
			finallyBlock = nth(ch, 0)
		} else {
			excepts = append(excepts, ch)
		}
	}
	if len(excepts) == 0 && finallyBlock == nil {
		// 裸 try：无保护语义，直接展开
		c.compileStmt(tryBlock)
		return
	}
	if len(excepts) > 1 {
		c.warnf("only the first except is reachable (exceptions carry no type info)")
	}

	hasFinally := finallyBlock != nil
	h1Label := freshLabel("except")
	endLabel := freshLabel("try_end")
	finLabel := freshLabel("fin")
	finExcLabel := freshLabel("fin_exc")
	// 保护目标：有 except → 第一个 except；仅 finally → finally 异常路径
	guardLabel := h1Label
	if len(excepts) == 0 {
		guardLabel = finExcLabel
	}

	// try 体
	c.builder.PushHandlerTo(guardLabel)
	c.compileStmt(tryBlock)
	c.builder.PopHandler()
	if hasFinally {
		c.builder.JmpTo(finLabel)
	} else {
		c.builder.JmpTo(endLabel)
	}

	// except handler（第一个）
	if len(excepts) > 0 {
		c.builder.Label(h1Label)
		if hasFinally {
			c.builder.PushHandlerTo(finExcLabel)
		}
		exc := excepts[0]
		excBody := exc.Children[len(exc.Children)-1]
		// except NAME as e：绑定消息到 e；无名/无 as：丢弃消息
		if len(exc.Children) >= 2 && exc.Children[0] != nil && exc.Children[0].Kind == ast.NName {
			slot := c.localSlot(exc.Children[0].Text)
			c.builder.StoreLocal(slot)
		} else {
			c.builder.Pop()
		}
		c.compileStmt(excBody)
		if hasFinally {
			c.builder.PopHandler()
			c.builder.JmpTo(finLabel)
		} else {
			c.builder.JmpTo(endLabel)
		}
	}

	if hasFinally {
		// 异常路径 finally：暂存消息 → 执行 → 重抛
		hidden := c.localSlot("$finally_exc")
		c.builder.Label(finExcLabel)
		c.builder.StoreLocal(hidden)
		c.compileStmt(finallyBlock)
		c.builder.LoadLocal(hidden)
		c.builder.Raise()
		// 正常路径 finally
		c.builder.Label(finLabel)
		c.compileStmt(finallyBlock)
		c.builder.JmpTo(endLabel)
	}
	c.builder.Label(endLabel)
}

func (c *Compiler) compileIf(n *ast.Node) {
	// cond then else
	cond := nth(n, 0)
	thenBlock := nth(n, 1)
	elseBlock := nth(n, 2)
	c.compileExpr(cond)
	elseLabel := freshLabel("else")
	endLabel := freshLabel("endif")
	c.builder.JzTo(elseLabel)
	c.compileStmt(thenBlock)
	c.builder.JmpTo(endLabel)
	c.builder.Label(elseLabel)
	if elseBlock != nil {
		c.compileStmt(elseBlock)
	}
	c.builder.Label(endLabel)
}

func (c *Compiler) compileWhile(n *ast.Node) {
	cond := nth(n, 0)
	body := nth(n, 1)
	startLabel := freshLabel("while_start")
	endLabel := freshLabel("while_end")
	c.enterLoopLabels(endLabel, startLabel)
	c.builder.Label(startLabel)
	c.compileExpr(cond)
	c.builder.JzTo(endLabel)
	c.compileStmt(body)
	c.builder.JmpTo(startLabel)
	c.builder.Label(endLabel)
	c.leaveLoopLabels()
}

func (c *Compiler) compileForC(n *ast.Node) {
	init := nth(n, 0)
	cond := nth(n, 1)
	update := nth(n, 2)
	body := nth(n, 3)
	endLabel := freshLabel("for_end")
	updateLabel := freshLabel("for_update")
	if init != nil {
		c.compileStmt(init)
	}
	startLabel := freshLabel("for_start")
	c.enterLoopLabels(endLabel, updateLabel)
	c.builder.Label(startLabel)
	if cond != nil {
		c.compileExpr(cond)
		c.builder.JzTo(endLabel)
	}
	c.compileStmt(body)
	c.builder.Label(updateLabel)
	if update != nil {
		c.compileStmt(update)
	}
	c.builder.JmpTo(startLabel)
	c.builder.Label(endLabel)
	c.leaveLoopLabels()
}

func (c *Compiler) compileForIn(n *ast.Node) {
	// 仅支持列表字面量展开
	varName := ""
	if len(n.Children) >= 1 && n.Children[0] != nil && n.Children[0].Kind == ast.NName {
		varName = n.Children[0].Text
	}
	iter := nth(n, 1)
	body := nth(n, 2)
	if iter == nil || iter.Kind != ast.NListExpr {
		c.warnf("for-in only supported with list literal '[a,b,...]'")
		return
	}
	for _, elem := range iter.Children {
		slot := c.localSlot(varName)
		c.compileExpr(elem)
		c.builder.StoreLocal(slot)
		c.compileStmt(body)
	}
}

// break/continue label 栈（简化版，每循环一对）
type loopLabel struct{ breakLabel, continueLabel string }

var loopStack []loopLabel
var labelCounter int

func freshLabel(prefix string) string {
	labelCounter++
	return fmt.Sprintf("%s_%d", prefix, labelCounter)
}
func (c *Compiler) enterLoopLabels(breakL, contL string) {
	loopStack = append(loopStack, loopLabel{breakL, contL})
}
func (c *Compiler) leaveLoopLabels() {
	if len(loopStack) > 0 {
		loopStack = loopStack[:len(loopStack)-1]
	}
}

func (c *Compiler) compileReturn(n *ast.Node) {
	if len(n.Children) >= 1 && n.Children[0] != nil {
		c.compileExpr(n.Children[0])
	} else {
		c.builder.PushI64(0)
	}
	c.builder.Ret()
}

// ===== Assignment =====

func (c *Compiler) compileAssign(n *ast.Node) {
	if len(n.Children) < 2 {
		return
	}
	lhs := n.Children[0]
	rhs := n.Children[1]
	// 先编译 RHS
	c.compileExpr(rhs)
	// 按 LHS 类型分派
	switch lhs.Kind {
	case ast.NName:
		slot := c.localSlot(lhs.Text)
		c.builder.StoreLocal(slot)
	case ast.NMember:
		// obj.attr = v
		if len(lhs.Children) >= 2 {
			obj := lhs.Children[0]
			attrName := ""
			if lhs.Children[1] != nil {
				attrName = lhs.Children[1].Text
			}
			// 静态字段写路径：ClassName.field = v
			// SetStatic 栈序（底→顶）：class_id, attr_idx, value（弹 3 压 0）
			if obj.Kind == ast.NName {
				if classID, ok := c.classNameToID[obj.Text]; ok {
					tmpSlot := c.allocTempSlot()
					c.builder.StoreLocal(tmpSlot) // 暂存 rhs（当前栈顶）
					c.builder.PushI64(int64(classID))
					aidx := c.builder.InternString(attrName)
					c.builder.PushI64(int64(aidx))
					c.builder.LoadLocal(tmpSlot)
					c.builder.SetStatic()
					// 赋值表达式值保留在栈（语句级 NExprStmt 的 Pop 丢弃）
					c.builder.LoadLocal(tmpSlot)
					return
				}
			}
			// 实例字段写路径：obj.attr = v → 栈需：obj, attr_idx, v → SetAttr
			{
				// 先暂存 rhs 到临时：栈顶是 rhs，存入 tmp
				tmpSlot := c.allocTempSlot()
				c.builder.StoreLocal(tmpSlot)
				// 编译 obj
				c.compileExpr(obj)
				// attr idx
				aidx := c.builder.InternString(attrName)
				c.builder.PushI64(int64(aidx))
				// 取出 rhs
				c.builder.LoadLocal(tmpSlot)
				c.builder.SetAttr()
			}
		}
	case ast.NIndex:
		// TODO: index assignment (arrays not supported yet)
		c.warnf("index assignment not supported")
		c.builder.Pop()
	default:
		c.warnf("invalid assignment target: %s", ast.NodeKindName(lhs.Kind))
		c.builder.Pop()
	}
}

func (c *Compiler) compileAugAssign(n *ast.Node) {
	if len(n.Children) < 2 {
		return
	}
	lhs := n.Children[0]
	rhs := n.Children[1]
	if lhs.Kind != ast.NName {
		// 通用：LHS 读 → 编译 RHS → op → 写回（简化：仅 NName 支持）
		c.warnf("aug-assign only supported for simple names")
		return
	}
	slot := c.localSlot(lhs.Text)
	c.builder.LoadLocal(slot)
	c.compileExpr(rhs)
	op := strings.TrimSuffix(n.Text, "=")
	c.emitArithOp(op)
	c.builder.StoreLocal(slot)
}

// ===== Expression compilation =====

func (c *Compiler) compileExpr(n *ast.Node) {
	if n == nil {
		c.builder.PushI64(0)
		return
	}
	switch n.Kind {
	case ast.NInt:
		c.builder.PushI64(n.IVal)
	case ast.NBool:
		if n.IVal != 0 {
			c.builder.PushI64(1)
		} else {
			c.builder.PushI64(0)
		}
	case ast.NNull:
		c.builder.PushI64(0)
	case ast.NFloat:
		// 简化：浮点数转 int（截断）
		c.warnf("float literal truncated to int64")
		c.builder.PushI64(int64(n.FVal))
	case ast.NString:
		// 字符串：编译期 Intern → 运行时 str_idx 压栈
		idx := c.builder.InternString(n.Text)
		// 通过 System 级内置 str_idx 获取：先 str.new 再逐字符 append
		c.emitBuildString(idx)
	case ast.NName:
		// 局部变量槽查找（方法内），或顶层作用域（toplevelLocals 全局表）
		if slot, ok := c.currentLocals[n.Text]; ok {
			c.builder.LoadLocal(slot)
			return
		}
		if c.currentLocals == nil {
			if slot, ok := toplevelLocals[n.Text]; ok {
				c.builder.LoadLocal(slot)
				return
			}
		}
		// 找不到 → 当作 0 + 警告
		c.warnf("undefined name '%s' (treated as 0)", n.Text)
		c.builder.PushI64(0)
	case ast.NThis:
		// this: slot 0 当非静态方法
		if c.insideMethod {
			c.builder.LoadLocal(0)
		} else {
			c.builder.PushI64(0)
		}
	case ast.NBinary:
		c.compileBinary(n)
	case ast.NUnary:
		c.compileUnary(n)
	case ast.NCompare:
		c.compileCompare(n)
	case ast.NAssign:
		c.compileAssign(n)
		// 表达式中赋值 → 值保留在栈
	case ast.NAugAssign:
		c.compileAugAssign(n)
	case ast.NCall:
		c.compileCall(n)
	case ast.NMember:
		c.compileMember(n)
	case ast.NNew:
		c.compileNew(n)
	case ast.NInstanceOf:
		// [obj_id, class_id] → 1/0
		if len(n.Children) >= 2 {
			c.compileExpr(n.Children[0])
			// type: N_NAME → class name → id
			classID := int32(-1)
			typeNode := n.Children[1]
			if typeNode.Kind == ast.NName {
				if id, ok := c.classNameToID[typeNode.Text]; ok {
					classID = int32(id)
				}
			}
			c.builder.PushI64(int64(classID))
			c.builder.InstanceOf()
		}
	case ast.NListExpr:
		// 简化：返回长度（后续可扩展为数组对象）
		c.builder.PushI64(int64(len(n.Children)))
	case ast.NSuper:
		c.warnf("super as expression not supported")
		c.builder.PushI64(0)
	case ast.NLambda, ast.NYield:
		c.warnf("%s not implemented", ast.NodeKindName(n.Kind))
		c.builder.PushI64(0)
	default:
		c.warnf("unsupported expression node: %s", ast.NodeKindName(n.Kind))
		c.builder.PushI64(0)
	}
}

// emitBuildString 把编译期 intern 的字符串 idx 转成运行时 str_idx：
// 通过 str.new + 逐个 appendChar 实现。简化实现：从 strTable 取出内容逐字符写入。
func (c *Compiler) emitBuildString(compiledIdx int32) {
	// 编译器 StringTable 直接成为运行时 strTable 前缀，所以 idx 完全一致
	c.builder.PushI64(int64(compiledIdx))
	c.builder.StrNewFromIdx()
}

func (c *Compiler) compileBinary(n *ast.Node) {
	op := n.Text
	if op == "and" || op == "or" {
		// 短路逻辑
		endLabel := freshLabel("logic_end")
		c.compileExpr(nth(n, 0))
		if op == "and" {
			// left 为 false → 结果 = left (跳过 right)
			c.builder.Dup()
			c.builder.JzTo(endLabel)
		} else {
			// left 为 true → 结果 = left
			c.builder.Dup()
			c.builder.JnzTo(endLabel)
		}
		c.builder.Pop()
		c.compileExpr(nth(n, 1))
		c.builder.Label(endLabel)
		return
	}
	c.compileExpr(nth(n, 0))
	c.compileExpr(nth(n, 1))
	c.emitArithOp(op)
}

func (c *Compiler) compileUnary(n *ast.Node) {
	op := n.Text
	c.compileExpr(nth(n, 0))
	switch op {
	case "-":
		c.builder.NegI64()
	case "+":
		// no-op
	case "not":
		// not x = x == 0
		endLabel := freshLabel("not_end")
		resLabel := freshLabel("not_res")
		c.builder.JzTo(resLabel)
		c.builder.PushI64(0)
		c.builder.JmpTo(endLabel)
		c.builder.Label(resLabel)
		c.builder.PushI64(1)
		c.builder.Label(endLabel)
	case "del":
		c.warnf("del not implemented")
		c.builder.Pop()
		c.builder.PushI64(0)
	case "await":
		c.warnf("await not implemented")
	default:
		c.warnf("unknown unary op: %s", op)
	}
}

func (c *Compiler) compileCompare(n *ast.Node) {
	op := n.Text
	c.compileExpr(nth(n, 0))
	c.compileExpr(nth(n, 1))
	switch op {
	case "==":
		c.builder.CmpEq()
	case "!=":
		c.builder.CmpNe()
	case "<":
		c.builder.CmpLt()
	case ">":
		c.builder.CmpGt()
	case "<=":
		c.builder.CmpLe()
	case ">=":
		c.builder.CmpGe()
	case "is":
		// identity: i64 相等
		c.builder.CmpEq()
	case "in":
		c.warnf("'in' operator not implemented (returns 0)")
		c.builder.Pop()
		c.builder.Pop()
		c.builder.PushI64(0)
	default:
		c.warnf("unknown compare op: %s", op)
		c.builder.Pop()
		c.builder.Pop()
		c.builder.PushI64(0)
	}
}

func (c *Compiler) emitArithOp(op string) {
	switch op {
	case "+":
		c.builder.AddI64()
	case "-":
		c.builder.SubI64()
	case "*":
		c.builder.MulI64()
	case "/":
		c.builder.DivI64()
	case "%":
		c.builder.ModI64()
	default:
		c.warnf("unknown arithmetic op: %s", op)
	}
}

// compileCall —— 支持：
//  1. 内置函数：system.print / system.print_char / str.new / str.append_c / str.len / str.get_c / str.delete
//  2. 用户定义方法：NAME(args) 或 NAME.NAME(args) → 查 methodLabels
//  3. 对象方法调用：obj.method(args) → Invoke
func (c *Compiler) compileCall(n *ast.Node) {
	if len(n.Children) < 1 {
		c.builder.PushI64(0)
		return
	}
	callee := n.Children[0]
	args := n.Children[1:]
	// 内置函数识别
	if callee.Kind == ast.NName {
		if c.tryCompileBuiltin(callee.Text, args) {
			return
		}
	}
	if callee.Kind == ast.NMember {
		// obj.method(args)
		if len(callee.Children) >= 2 {
			obj := callee.Children[0]
			methodNameNode := callee.Children[1]
			methodName := ""
			if methodNameNode != nil {
				methodName = methodNameNode.Text
			}
			// 先尝试识别内置模块方法：system.print / str.new / list.new / dict.new / http.request 等
			if obj.Kind == ast.NName {
				mods := map[string]bool{"system": true, "str": true, "math": true, "list": true, "dict": true, "http": true}
				if mods[obj.Text] {
					fullName := obj.Text + "." + methodName
					if c.tryCompileBuiltin(fullName, args) {
						return
					}
				}
			}
			// 判断是否静态方法调用 ClassName.staticMethod (obj = N_NAME 且是类名)
			if obj.Kind == ast.NName {
				if _, isClass := c.classNameToID[obj.Text]; isClass {
					// 静态方法：ClassName.method(args)
					callName := obj.Text + "." + methodName
					if label, ok := c.methodLabels[callName]; ok {
						for _, a := range args {
							c.compileExpr(a)
						}
						// Call to 绝对偏移（通过标签）
						c.emitCallByLabel(label, len(args))
						return
					}
				}
			}
			// 实例方法：栈（底→顶）：obj_id, method_str_idx, arg0..argN-1, argc
			// opInvoke pop 顺序：先 argc → 反向 argc args → method_idx → obj_id
			c.compileExpr(obj)
			midx := c.builder.InternString(methodName)
			c.builder.PushI64(int64(midx))
			for _, a := range args {
				c.compileExpr(a)
			}
			c.builder.PushI64(int64(len(args))) // argc 必须在最后（栈顶）
			c.builder.Invoke()
			return
		}
	}
	// 顶层用户方法
	if callee.Kind == ast.NName {
		if label, ok := c.methodLabels[callee.Text]; ok {
			for _, a := range args {
				c.compileExpr(a)
			}
			c.emitCallByLabel(label, len(args))
			return
		}
	}
	c.warnf("undefined function call")
	c.builder.PushI64(0)
}

func (c *Compiler) emitCallByLabel(label string, argc int) {
	// 调用约定（栈底→顶）：arg0, arg1, ... argN-1, argc（整数）
	// VM 的 OpCall 会：弹出 argc → 然后弹出 argc 个参数存入 locals[0..argc-1]
	c.builder.PushI64(int64(argc))
	op := bytecode.OpCall
	if off, ok := c.builder.labels[label]; ok {
		c.builder.emit(op)
		at := len(c.builder.code)
		// Call 语义：target = pc+5+off → off = target - (at+4)
		c.builder.emitI32(int32(off - (at + 4)))
		return
	}
	c.builder.emit(op)
	at := len(c.builder.code)
	c.builder.emitI32(0)
	c.builder.forwardRefs[label] = append(c.builder.forwardRefs[label], at)
}

// tryCompileBuiltin 尝试编译内置调用，成功返回 true
func (c *Compiler) tryCompileBuiltin(name string, args []*ast.Node) bool {
	switch name {
	// 高并发内置：Go 风格缓冲通道
	case "chan_new":
		// [capacity] → [chan_id]
		if len(args) >= 1 {
			c.compileExpr(args[0])
		} else {
			c.builder.PushI64(1)
		}
		c.builder.ChanNew()
		return true
	case "chan_put":
		// [chan_id, value] → [0]（发送；通道满则阻塞）
		if len(args) < 2 {
			c.warnf("chan_put requires 2 args")
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.ChanPut()
		c.builder.PushI64(0)
		return true
	case "chan_get":
		// [chan_id] → [value]（接收；通道空则阻塞）
		if len(args) < 1 {
			c.warnf("chan_get requires 1 arg")
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.ChanGet()
		return true
	case "system.print", "System_print":
		if len(args) < 1 {
			c.warnf("system.print requires 1 arg")
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.PrintI64()
		c.builder.PushI64(0) // return 0
		return true
	case "system.print_char", "System_print_char":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.PrintChar()
		c.builder.PushI64(0)
		return true
	case "system.println":
		// 打印第一个参数（整数）后加换行
		if len(args) >= 1 {
			c.compileExpr(args[0])
			c.builder.PrintI64()
		}
		c.builder.PushI64(int64('\n'))
		c.builder.PrintChar()
		c.builder.PushI64(0)
		return true
	case "system.print_str":
		// 打印字符串（str_idx）：输出字符串内容
		if len(args) >= 1 {
			c.compileExpr(args[0])
			c.builder.PrintStr()
		}
		c.builder.PushI64(0)
		return true
	case "system.exec":
		if len(args) < 1 {
			c.builder.PushI64(-1)
			return true
		}
		c.compileExpr(args[0])
		c.builder.SystemExec()
		return true
	case "system.read_file":
		// 读取文本文件：system.read_file(path) → [list_id]
		// list[0]=content_str_idx，list[1]=ok(0/1)（与 http.request 双值封装风格一致）
		if len(args) < 1 {
			c.builder.PushI64(0)
			c.builder.PushI64(0)
		} else {
			c.compileExpr(args[0])
			c.builder.SystemReadFile()
		}
		// 栈：[ok, content_idx] → 封装成 list
		c.builder.ListNew()
		c.builder.Swap()
		c.builder.ListPush()
		c.builder.Swap()
		c.builder.ListPush()
		return true
	case "str.new", "str_new":
		c.builder.StrNew()
		return true
	case "str.append_c", "str_append_c":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.StrAppendC()
		return true
	case "str.len", "str_len":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.StrLen()
		return true
	case "str.get_c", "str_get_c":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.StrGetC()
		return true
	case "str.delete", "str_delete":
		if len(args) < 1 {
			return true
		}
		c.compileExpr(args[0])
		c.builder.StrDelete()
		c.builder.PushI64(0)
		return true

	// ===== 列表容器 =====
	case "list.new", "list_new":
		c.builder.ListNew()
		return true
	case "list.push", "list_push":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.ListPush()
		c.builder.Pop() // VM push 了 list_id，语句级消耗掉
		c.builder.PushI64(0)
		return true
	case "list.get", "list_get":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.ListGet()
		return true
	case "list.set", "list_set":
		if len(args) < 3 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.compileExpr(args[2])
		c.builder.ListSet()
		c.builder.Pop() // VM push 了 list_id
		c.builder.PushI64(0)
		return true
	case "list.pop", "list_pop":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.ListPop()
		return true
	case "list.len", "list_len":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.ListLen()
		return true
	case "list.delete_at", "list_delete_at":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.ListDeleteAt()
		c.builder.Pop() // VM push 了 list_id
		c.builder.PushI64(0)
		return true

	// ===== 字典容器 =====
	case "dict.new", "dict_new":
		c.builder.DictNew()
		return true
	case "dict.put", "dict_put":
		if len(args) < 3 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.compileExpr(args[2])
		c.builder.DictPut()
		c.builder.Pop() // VM push 了 dict_id
		c.builder.PushI64(0)
		return true
	case "dict.get", "dict_get":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.DictGet()
		return true
	case "dict.has", "dict_has":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.DictHas()
		return true
	case "dict.delete", "dict_delete":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.DictDelete()
		c.builder.Pop() // VM push 了 dict_id
		c.builder.PushI64(0)
		return true
	case "dict.len", "dict_len":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.DictLen()
		return true

	// ===== 字符串增强 =====
	case "str.find", "str_find":
		if len(args) < 2 {
			c.builder.PushI64(-1)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.StrFind()
		return true
	case "str.slice", "str_slice":
		if len(args) < 3 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.compileExpr(args[2])
		c.builder.StrSlice()
		return true
	case "str.equal", "str_equal":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.StrEqual()
		return true
	case "str.new_from_idx", "str_new_from_idx":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.StrNewFromIdx()
		return true
	case "str.trim", "str_trim":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.StrTrim()
		return true
	case "str.replace", "str_replace":
		if len(args) < 3 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.compileExpr(args[2])
		c.builder.StrReplace()
		return true
	case "str.concat", "str_concat":
		if len(args) < 2 {
			c.builder.PushI64(int64(c.builder.InternString("")))
			c.builder.StrNewFromIdx()
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.StrAppendStr()
		return true

	// ===== 类型转换 / 时间 =====
	case "atoi", "str.to_int":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.Atoi()
		return true
	case "itoa", "int.to_str":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.Itoa()
		return true
	case "sleep":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.Sleep()
		c.builder.PushI64(0)
		return true
	case "now":
		c.builder.Now()
		return true

	// ===== HTTP =====
	case "http.set_ua":
		if len(args) < 1 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.builder.HttpSetUA()
		c.builder.PushI64(0)
		return true
	case "http.add_header":
		if len(args) < 2 {
			c.builder.PushI64(0)
			return true
		}
		c.compileExpr(args[0])
		c.compileExpr(args[1])
		c.builder.HttpAddHdr()
		c.builder.PushI64(0)
		return true
	case "http.get_cookie":
		if len(args) < 1 {
			c.builder.PushI64(int64(c.builder.InternString("")))
			c.builder.StrNewFromIdx()
			return true
		}
		c.compileExpr(args[0])
		c.builder.HttpGetCookie()
		return true
	case "http.clear":
		c.builder.HttpClear()
		c.builder.PushI64(0)
		return true
	case "http.request":
		// http.request(url, method, body) → [list_id]（list 包含 status, body_str_idx）
		// VM OpHttpRequest 压 [status, body_idx]，编译器在此后用 ListNew+ListPush* 封装
		if len(args) >= 3 {
			c.compileExpr(args[0])
			c.compileExpr(args[1])
			c.compileExpr(args[2])
		} else if len(args) == 2 {
			c.compileExpr(args[0])
			c.compileExpr(args[1])
			c.builder.PushI64(int64(c.builder.InternString("")))
			c.builder.StrNewFromIdx()
		} else if len(args) == 1 {
			c.compileExpr(args[0])
			c.builder.PushI64(int64(c.builder.InternString("")))
			c.builder.StrNewFromIdx()
			c.builder.PushI64(int64(c.builder.InternString("")))
			c.builder.StrNewFromIdx()
		} else {
			c.builder.ListNew()
			return true
		}
		c.builder.HttpRequest()
		// 栈：[status, body_idx] → 封装成 list
		c.builder.ListNew()
		// 现在栈：[status, body_idx, list_id]
		c.builder.Swap()
		// 栈：[status, list_id, body_idx]
		c.builder.ListPush()
		// 栈：[status, list_id] （list_id 现在包含 body_idx）
		c.builder.Swap()
		// 栈：[list_id, status]
		c.builder.ListPush()
		// 栈：[list_id] （list 现在包含 body_idx, status —— 先压的 body_idx 在 [0]，后压的 status 在 [1]）
		return true
	}
	return false
}

// compileMember —— x.y 读取属性：GetAttr
func (c *Compiler) compileMember(n *ast.Node) {
	if len(n.Children) < 2 {
		c.builder.PushI64(0)
		return
	}
	obj := n.Children[0]
	var attrName string
	if n.Children[1] != nil {
		attrName = n.Children[1].Text
	}
	// ClassName.staticField → GetStatic
	if obj.Kind == ast.NName {
		if classID, ok := c.classNameToID[obj.Text]; ok {
			c.builder.PushI64(int64(classID))
			aidx := c.builder.InternString(attrName)
			c.builder.PushI64(int64(aidx))
			c.builder.GetStatic()
			return
		}
	}
	// 实例属性
	c.compileExpr(obj)
	aidx := c.builder.InternString(attrName)
	c.builder.PushI64(int64(aidx))
	c.builder.GetAttr()
}

// compileNew —— new ClassName(args) → NewObj
// opNewObj 期望栈（底→顶）：class_id, arg0..argN-1, argc
// opNewObj pop 顺序：先弹 argc → 反向弹 N args → 弹 classID
func (c *Compiler) compileNew(n *ast.Node) {
	classID := int32(-1)
	if id, ok := c.classNameToID[n.Text]; ok {
		classID = int32(id)
	}
	c.builder.PushI64(int64(classID))
	var args []*ast.Node
	if len(n.Children) >= 1 && n.Children[0] != nil {
		args = n.Children[0].Children
	}
	// 先压全部 args（arg0 先压在底）
	for _, a := range args {
		c.compileExpr(a)
	}
	// argc 必须最后压（栈最顶，opNewObj 第一个 pop 拿到）
	argc := len(args)
	c.builder.PushI64(int64(argc))
	c.builder.NewObj()
}

// ===== Method body compilation =====

func (c *Compiler) compileMethodBody(m *methodInfo) {
	c.enterMethodScope()
	c.insideMethod = true
	c.numParams = m.numParams
	// 参数名 → slot：param[0] → slot 0, param[1] → slot 1...
	for i, pn := range m.paramNames {
		c.currentLocals[pn] = uint8(i)
	}
	if uint8(m.numParams) > c.nextSlot {
		c.nextSlot = uint8(m.numParams)
	}
	c.builder.Label(m.label)
	body := (*ast.Node)(nil)
	if m.node != nil && len(m.node.Children) >= 2 {
		body = m.node.Children[1]
	}
	if body != nil {
		c.compileStmt(body)
	}
	// 方法无 return 时兜底：返回 0
	c.builder.PushI64(0)
	c.builder.Ret()
	c.insideMethod = false
	c.leaveMethodScope()
	// 注册 export
	numLocals := c.nextSlot
	if numLocals < 8 {
		numLocals = 8
	}
	c.builder.AddExport(m.name, int32(labelOffset(c.builder, m.label)), numLocals, uint8(m.numParams))
}

func (c *Compiler) compileClassMethodBody(cl *classInfo, m *classMethod) {
	c.enterMethodScope()
	c.insideMethod = true
	c.numParams = m.numParams
	// 非静态方法：slot 0 = this，之后才是 params
	baseSlot := uint8(0)
	if !m.isStatic {
		c.currentLocals["this"] = 0
		baseSlot = 1
	}
	for i, pn := range m.paramNames {
		c.currentLocals[pn] = baseSlot + uint8(i)
	}
	c.nextSlot = baseSlot + uint8(m.numParams)
	c.builder.Label(m.label)
	entryOff := int32(c.builder.CodeSize())
	m.entryOffset = entryOff
	// 构造函数（init 或与类同名）：先注入祖先链实例字段初始值（父先子后）
	if m.name == "init" || m.name == cl.name {
		c.emitCtorFieldInitializers(cl)
	}
	if m.body != nil {
		c.compileStmt(m.body)
	}
	c.builder.PushI64(0)
	c.builder.Ret()
	m.numLocals = c.nextSlot
	if m.numLocals < 8 {
		m.numLocals = 8
	}
	c.insideMethod = false
	c.leaveMethodScope()
}

// ===== Local variable slots =====

func (c *Compiler) enterMethodScope() {
	c.currentLocals = map[string]uint8{}
	c.nextSlot = 0
}
func (c *Compiler) leaveMethodScope() {
	c.currentLocals = nil
	c.nextSlot = 0
}

func (c *Compiler) localSlot(name string) uint8 {
	if c.currentLocals == nil {
		// 顶层作用域：全局 slot 0-255（简化：同名共享槽）
		// 用一个持久化映射
		if c.methodLabels == nil {
			c.methodLabels = map[string]string{}
		}
		return c.toplevelSlot(name)
	}
	if slot, ok := c.currentLocals[name]; ok {
		return slot
	}
	slot := c.nextSlot
	c.nextSlot++
	c.currentLocals[name] = slot
	return slot
}

var toplevelLocals = map[string]uint8{}
var toplevelNextSlot uint8

func (c *Compiler) toplevelSlot(name string) uint8 {
	if slot, ok := toplevelLocals[name]; ok {
		return slot
	}
	slot := toplevelNextSlot
	toplevelNextSlot++
	toplevelLocals[name] = slot
	return slot
}

func (c *Compiler) allocTempSlot() uint8 {
	if c.currentLocals != nil {
		s := c.nextSlot
		c.nextSlot++
		return s
	}
	// 顶层临时：用 255
	return 255
}

func labelOffset(b *Builder, label string) int32 {
	if off, ok := b.labels[label]; ok {
		return int32(off)
	}
	return -1
}

func nth(n *ast.Node, i int) *ast.Node {
	if n == nil || i < 0 || i >= len(n.Children) {
		return nil
	}
	return n.Children[i]
}

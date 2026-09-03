// Package vm —— Method 语言虚拟机（栈式字节码解释器）
//
// 支持 40+ 操作码：算术 / 比较 / 控制流 / 局部变量 / 字符串表 / OOP（类/对象/继承/方法分派）
package vm

import (
	"crypto/tls"
	"encoding/binary"
	"fmt"
	"io"
	"net/http"
	"net/http/cookiejar"
	"net/url"
	"os"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode/utf8"

	"method/bytecode"
)

// Status —— 执行/加载结果
type Status int

const (
	StatusOk Status = iota
	StatusCorrupted
	StatusVersionMismatch
	StatusTruncated
	StatusInvalidOpcode
	StatusStackUnderflow
	StatusStackOverflow
	StatusDivByZero
	StatusUnsupported
	StatusIO
	StatusInvalidArgc
	StatusUncaught
)

var statusNames = map[Status]string{
	StatusOk:              "ok",
	StatusCorrupted:       "corrupted file",
	StatusVersionMismatch: "version mismatch",
	StatusTruncated:       "truncated code segment",
	StatusInvalidOpcode:   "invalid opcode",
	StatusStackUnderflow:  "stack underflow",
	StatusStackOverflow:   "stack overflow",
	StatusDivByZero:       "division by zero",
	StatusUnsupported:     "unsupported",
	StatusIO:              "io error",
	StatusInvalidArgc:     "invalid argc",
	StatusUncaught:        "uncaught exception",
}

func (s Status) String() string {
	if n, ok := statusNames[s]; ok {
		return n
	}
	return "?"
}

// ============================================================
//  Class metadata + registry
// ============================================================

type ClassMethod struct {
	CodeOffset int32
	NumParams  uint8
	NumLocals  uint8
	IsStatic   bool
}
type ClassField struct {
	IsStatic bool
	Slot     int
}
type ClassMeta struct {
	Name               string
	ID                 int32
	ParentID           int32
	Interfaces         []int32
	Fields             map[string]ClassField
	Methods            map[string]ClassMethod
	FieldOrder         []string
	StaticFieldOrder   []string
	VTable             map[string]ClassMethod
	FieldTable         map[string]ClassField
	TotalInstanceSlots int
	InitOffset         int32
	InitNumParams      uint8
}

type Object struct {
	ClassID  int32
	RefCount int32
	Fields   []int64
}

// Closure —— lambda block 形式闭包：entryPC + 捕获的外层变量快照
type Closure struct {
	EntryPC  int32
	NParams  uint8
	NLocals  uint8
	NCapture uint8
	Captures []int64
	RefCount int32
}

type ClassRegistry struct {
	classes    []ClassMeta
	nameToID   map[string]int32
	staticVals [][]int64
}

func NewClassRegistry() *ClassRegistry {
	return &ClassRegistry{
		nameToID: map[string]int32{},
	}
}

func (r *ClassRegistry) NumClasses() int { return len(r.classes) }

func (r *ClassRegistry) Find(id int32) *ClassMeta {
	if id < 0 || int(id) >= len(r.classes) {
		return nil
	}
	return &r.classes[id]
}
func (r *ClassRegistry) FindByName(name string) *ClassMeta {
	if id, ok := r.nameToID[name]; ok {
		return &r.classes[id]
	}
	return nil
}
func (r *ClassRegistry) StaticValues(id int32) *[]int64 {
	for int32(len(r.staticVals)) <= id {
		r.staticVals = append(r.staticVals, nil)
	}
	if r.staticVals[id] == nil {
		cm := r.Find(id)
		size := 0
		if cm != nil {
			size = len(cm.StaticFieldOrder)
		}
		r.staticVals[id] = make([]int64, size)
	}
	return &r.staticVals[id]
}

// LoadPseudo 解析 Pseudo 段
func (r *ClassRegistry) LoadPseudo(pseudo []byte) (Status, error) {
	off := 0
	for off+2 <= len(pseudo) {
		tag := binary.LittleEndian.Uint16(pseudo[off : off+2])
		off += 2
		if tag == bytecode.PseudoTagEnd {
			break
		}
		if tag != bytecode.PseudoTagClassMeta {
			return StatusCorrupted, fmt.Errorf("unknown pseudo tag %d", tag)
		}
		var err error
		var name string
		var parentID int32
		if name, off, err = readStr(pseudo, off); err != nil {
			return StatusTruncated, err
		}
		if parentID, off, err = readI32(pseudo, off); err != nil {
			return StatusTruncated, err
		}
		cm := ClassMeta{
			Name:       name,
			ID:         int32(len(r.classes)),
			ParentID:   parentID,
			Fields:     map[string]ClassField{},
			Methods:    map[string]ClassMethod{},
			InitOffset: -1, // 无构造函数标记（0 是合法偏移，必须用 -1 表示"未设置"）
		}
		// interfaces
		var nif int32
		if nif, off, err = readI32(pseudo, off); err != nil {
			return StatusTruncated, err
		}
		for i := int32(0); i < nif; i++ {
			var id int32
			if id, off, err = readI32(pseudo, off); err != nil {
				return StatusTruncated, err
			}
			cm.Interfaces = append(cm.Interfaces, id)
		}
		// fields
		var nf int32
		if nf, off, err = readI32(pseudo, off); err != nil {
			return StatusTruncated, err
		}
		instSlot := 0
		staticSlot := 0
		for i := int32(0); i < nf; i++ {
			var fname string
			if fname, off, err = readStr(pseudo, off); err != nil {
				return StatusTruncated, err
			}
			if off+1 > len(pseudo) {
				return StatusTruncated, fmt.Errorf("missing flags")
			}
			flags := pseudo[off]
			off++
			isStatic := (flags & 1) != 0
			f := ClassField{IsStatic: isStatic}
			if isStatic {
				f.Slot = staticSlot
				staticSlot++
				cm.StaticFieldOrder = append(cm.StaticFieldOrder, fname)
			} else {
				f.Slot = instSlot
				instSlot++
				cm.FieldOrder = append(cm.FieldOrder, fname)
			}
			cm.Fields[fname] = f
		}
		// methods
		var nm int32
		if nm, off, err = readI32(pseudo, off); err != nil {
			return StatusTruncated, err
		}
		for i := int32(0); i < nm; i++ {
			var mname string
			var moff int32
			if mname, off, err = readStr(pseudo, off); err != nil {
				return StatusTruncated, err
			}
			if moff, off, err = readI32(pseudo, off); err != nil {
				return StatusTruncated, err
			}
			if off+3 > len(pseudo) {
				return StatusTruncated, fmt.Errorf("truncated method meta")
			}
			np := pseudo[off]
			nl := pseudo[off+1]
			flags := pseudo[off+2]
			off += 3
			isStatic := (flags & 1) != 0
			m := ClassMethod{
				CodeOffset: moff,
				NumParams:  np,
				NumLocals:  nl,
				IsStatic:   isStatic,
			}
			cm.Methods[mname] = m
			// init / <init>
			if (mname == "init" || mname == cm.Name) && cm.InitOffset < 0 {
				cm.InitOffset = moff
				cm.InitNumParams = np
			}
		}
		r.classes = append(r.classes, cm)
		r.nameToID[cm.Name] = cm.ID
	}
	// 建立继承链（vtable / field_table）
	if st := r.resolveInheritance(); st != StatusOk {
		return st, nil
	}
	return StatusOk, nil
}

func (r *ClassRegistry) resolveInheritance() Status {
	for i := range r.classes {
		if st := r.mergeFromParent(&r.classes[i]); st != StatusOk {
			return StatusCorrupted
		}
	}
	return StatusOk
}

func (r *ClassRegistry) mergeFromParent(cm *ClassMeta) Status {
	if cm.VTable != nil {
		return StatusOk
	}
	cm.VTable = map[string]ClassMethod{}
	cm.FieldTable = map[string]ClassField{}
	// 先继承父
	if cm.ParentID >= 0 {
		parent := r.Find(cm.ParentID)
		if parent == nil {
			return StatusCorrupted
		}
		if st := r.mergeFromParent(parent); st != StatusOk {
			return st
		}
		for k, v := range parent.VTable {
			cm.VTable[k] = v
		}
		// 父实例字段 slot 继承（保持 slot 序号）
		maxSlot := -1
		for k, f := range parent.FieldTable {
			if !f.IsStatic {
				cm.FieldTable[k] = f
				if f.Slot > maxSlot {
					maxSlot = f.Slot
				}
			}
		}
		nextSlot := maxSlot + 1
		// 自己的实例字段追加：FieldOrder 中按顺序重排 slot（兼容）
		for _, fn := range cm.FieldOrder {
			f := cm.Fields[fn]
			f.Slot = nextSlot
			nextSlot++
			cm.FieldTable[fn] = f
			cm.Fields[fn] = f // 写回
		}
		cm.TotalInstanceSlots = nextSlot
	} else {
		// 无父：直接用 FieldOrder 编号
		slot := 0
		for _, fn := range cm.FieldOrder {
			f := cm.Fields[fn]
			f.Slot = slot
			slot++
			cm.FieldTable[fn] = f
			cm.Fields[fn] = f
		}
		cm.TotalInstanceSlots = slot
	}
	// 方法合并 / 覆写
	for k, v := range cm.Methods {
		cm.VTable[k] = v
	}
	return StatusOk
}

// read helpers
func readI32(data []byte, off int) (int32, int, error) {
	if off+4 > len(data) {
		return 0, off, fmt.Errorf("truncated i32")
	}
	v := int32(binary.LittleEndian.Uint32(data[off : off+4]))
	return v, off + 4, nil
}
func readStr(data []byte, off int) (string, int, error) {
	l, off, err := readI32(data, off)
	if err != nil {
		return "", off, err
	}
	if l < 0 {
		return "", off, fmt.Errorf("negative string length")
	}
	if int(l) > len(data)-off {
		return "", off, fmt.Errorf("truncated string")
	}
	s := string(data[off : off+int(l)])
	return s, off + int(l), nil
}

// ============================================================
//  Interpreter (stack-based VM)
// ============================================================

type Interpreter struct {
	// 栈
	stack []int64
	sp    int
	// 局部变量（每次调用前保存/恢复 128..255）
	locals    [256]int64
	frames    []savedFrame
	callStack []int // 返回地址（字节码偏移）
	// 字符串表：索引 0 = 空串
	strTable []string
	// 对象表
	classes   *ClassRegistry
	objects   []Object
	freeSlots []int32
	// 容器：列表 + 字典（key 为字符串，值为 int64）
	lists    [][]int64
	listFree []int32
	dicts    []map[string]int64
	dictFree []int32
	// 指针表（ref-cell 模型：&x 复制值到指针表，*p 读写指针表）
	ptrs    []int64
	ptrFree []int32
	// 闭包表（lambda block 形式：捕获外层 local 值的快照）
	closures    []Closure
	closureFree []int32
	// HTTP 全局配置
	httpClient  *http.Client
	httpUA      string
	httpHdrs    map[string]string
	httpCookies map[string]string // 从 jar 提取的 name→value 映射（OpHttpRequest 后自动更新）
	// 输出
	Output io.Writer
	// 调试
	MaxSteps int64
	Trace    bool // 环境变量 METHOD_TRACE 开启时打印每条指令执行轨迹
	// 高并发：共享运行时（跨线程的通道注册表 + 等待组）与当前程序
	Shared *Shared
	prog   *bytecode.Program
	isRoot bool // 根线程在结束时等待所有 go 子线程
	// 异常处理：handler 栈（try 进入时压入，正常结束/Ret 弹出）
	handlers []handlerFrame
	// GC 垃圾回收（标记-清扫，保守式）
	gcThreshold  int     // 触发阈值（分配次数）
	gcAllocCount int     // 自上次 GC 以来的分配计数
	gcMarked     []bool  // 对象标记位图
	gcListMarked []bool  // 列表标记位图
	gcDictMarked []bool  // 字典标记位图
	gcPtrMarked  []bool  // 指针标记位图
	gcClosureMarked []bool // 闭包标记位图
	gcDisabled   bool    // GC 关闭标志（构造函数执行期间临时关闭）
	gcStats      GCStats // 统计
}

// Chan —— 跨线程缓冲通道（Go 风格；满发送阻塞 / 空接收阻塞）
type Chan struct {
	mu  sync.Mutex
	cv  *sync.Cond
	buf []int64
	cap int
}

// Shared —— 所有线程共享的运行时状态
type Shared struct {
	mu       sync.Mutex
	chans    map[int64]*Chan
	nextChan int64
	wg       sync.WaitGroup
}

func NewShared() *Shared {
	return &Shared{
		chans:    make(map[int64]*Chan),
		nextChan: 1,
	}
}

// chanNew 创建缓冲通道，返回通道 ID
func (s *Shared) chanNew(capacity int64) int64 {
	if capacity < 1 {
		capacity = 1
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	id := s.nextChan
	s.nextChan++
	ch := &Chan{cap: int(capacity)}
	ch.cv = sync.NewCond(&ch.mu)
	s.chans[id] = ch
	return id
}

// chanPut 发送值；通道满则阻塞。通道不存在返回 false。
func (s *Shared) chanPut(id, v int64) bool {
	s.mu.Lock()
	ch, ok := s.chans[id]
	s.mu.Unlock()
	if !ok {
		return false
	}
	ch.mu.Lock()
	for len(ch.buf) >= ch.cap {
		ch.cv.Wait()
	}
	ch.buf = append(ch.buf, v)
	ch.cv.Signal()
	ch.mu.Unlock()
	return true
}

// chanGet 接收值；通道空则阻塞。通道不存在返回 (0, false)。
func (s *Shared) chanGet(id int64) (int64, bool) {
	s.mu.Lock()
	ch, ok := s.chans[id]
	s.mu.Unlock()
	if !ok {
		return 0, false
	}
	ch.mu.Lock()
	for len(ch.buf) == 0 {
		ch.cv.Wait()
	}
	v := ch.buf[0]
	ch.buf = ch.buf[1:]
	ch.cv.Signal()
	ch.mu.Unlock()
	return v, true
}

type savedFrame struct {
	saved         [256]int64
	pendingNewObj int32 // >=0 表示当前帧是构造函数调用，Ret 时用 objID 替换 retval 压栈
	closureID     int32 // >=0 表示当前帧是闭包调用，OpLoadCapture/StoreCapture 据此定位
}

// handlerFrame —— try 块进入时的快照（异常处理目标）
type handlerFrame struct {
	handlerPC int // 异常时跳转的 handler 起始 pc
	sp        int // try 进入时的操作数栈深度
	csLen     int // try 进入时的 callStack 深度
	framesLen int // try 进入时的 frames 深度（用于 Ret 时判定 handler 归属 / 跨帧恢复）
}

// GCStats —— GC 统计信息
type GCStats struct {
	TotalRuns     int   // GC 总运行次数
	LastFreed     int   // 上次回收的对象数
	LastListFreed int   // 上次回收的列表数
	LastDictFreed int   // 上次回收的字典数
	LastMarkTime  int64 // 上次标记阶段纳秒
	LastSweepTime int64 // 上次清扫阶段纳秒
}

func NewInterpreter() *Interpreter {
	jar, _ := cookiejar.New(nil)
	tr := &http.Transport{
		TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
	}
	return &Interpreter{
		stack:    make([]int64, 0, 256),
		strTable: []string{""},
		classes:  NewClassRegistry(),
		objects:  make([]Object, 1), // index 0 reserved (null)
		lists:    make([][]int64, 1),
		dicts:    make([]map[string]int64, 1),
		ptrs:     make([]int64, 1),
		closures: make([]Closure, 1), // index 0 reserved (invalid closure)
		Output:   os.Stdout,
		MaxSteps: 1 << 30,
		Trace:    os.Getenv("METHOD_TRACE") != "",
		Shared:   NewShared(),
		isRoot:   true,
		httpClient: &http.Client{
			Jar:       jar,
			Timeout:   30 * time.Second,
			Transport: tr,
		},
		httpUA:      "method/1.0",
		httpHdrs:    map[string]string{},
		httpCookies: map[string]string{},
		gcThreshold: 200, // 每 200 次分配自动触发一次 GC
	}
}

func (vm *Interpreter) Classes() *ClassRegistry { return vm.classes }

// Run 执行程序（根线程入口：加载类元数据 + 从 pc=0 顺序执行）
func (vm *Interpreter) Run(prog *bytecode.Program) (Status, error) {
	if !prog.Valid {
		return StatusCorrupted, fmt.Errorf("program not valid")
	}
	if prog.Version != bytecode.BytecodeVersion {
		return StatusVersionMismatch, nil
	}
	vm.prog = prog
	// 加载 Pseudo 段（类元数据）
	if len(prog.Pseudo) > 0 {
		st, err := vm.classes.LoadPseudo(prog.Pseudo)
		if st != StatusOk {
			return st, err
		}
	}
	// 合并字符串表（编译器的 intern 结果）
	if len(prog.StringTable) > 0 {
		// 以编译器的为准；但运行时自己也会扩展，所以追加（保留相同内容就去重由调用方处理）
		vm.strTable = append([]string(nil), prog.StringTable...)
	}
	return vm.runCode(prog, 0)
}

// runCode 从 startPC 开始执行字节码主循环。
//
// 根线程（isRoot）在退出前（无论正常/异常路径）等待全部 go 子线程结束；
// go 子线程由 opGo 创建，方法体 Ret 时 callStack 为空即自然结束。
func (vm *Interpreter) runCode(prog *bytecode.Program, startPC int) (st Status, err error) {
	defer func() {
		// 根线程退出时等待所有子线程（子线程的输出/通道操作全部完成）
		if vm.isRoot && vm.Shared != nil {
			vm.Shared.wg.Wait()
		}
	}()
	code := prog.Code
	pc := startPC
	steps := int64(0)
	for pc < len(code) {
		steps++
		if steps > vm.MaxSteps {
			return StatusUnsupported, fmt.Errorf("exceeded max steps (%d)", vm.MaxSteps)
		}
		op := bytecode.Op(code[pc])
		pc++
		if vm.Trace {
			csTail := vm.callStack
			if len(csTail) > 4 {
				csTail = csTail[len(csTail)-4:]
			}
			fmt.Fprintf(os.Stderr, "[pc=%d op=%s sp=%d frames=%d csLen=%d cs=%v]\n",
				pc-1, bytecode.OpName(op), vm.sp, len(vm.frames), len(vm.callStack), csTail)
		}
		switch op {
		case bytecode.OpNop:
		case bytecode.OpHalt:
			return StatusOk, nil
		case bytecode.OpPushI64:
			v, np, err := readI64(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			pc = np
			vm.push(v)
		case bytecode.OpPop:
			vm.popNo()
		case bytecode.OpDup:
			v, err := vm.peek()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(v)
		case bytecode.OpSwap:
			if vm.sp < 2 {
				return StatusStackUnderflow, fmt.Errorf("swap needs 2 items sp=%d", vm.sp)
			}
			vm.stack[vm.sp-1], vm.stack[vm.sp-2] = vm.stack[vm.sp-2], vm.stack[vm.sp-1]
		case bytecode.OpLoadLocal:
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			pc++
			vm.push(vm.locals[slot])
		case bytecode.OpStoreLocal:
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			pc++
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.locals[slot] = v
		case bytecode.OpAddLocalLocal:
			if pc+1 >= len(code) {
				return StatusTruncated, nil
			}
			a := code[pc]
			bb := code[pc+1]
			pc += 2
			vm.locals[a] += vm.locals[bb]
		case bytecode.OpIncLocalImm8:
			if pc+1 >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			imm := int8(code[pc+1])
			pc += 2
			vm.locals[slot] += int64(imm)
		case bytecode.OpJccLocalLocal:
			if pc+6 >= len(code) {
				return StatusTruncated, nil
			}
			a := code[pc]
			bb := code[pc+1]
			mode := code[pc+2]
			off := int32(binary.LittleEndian.Uint32(code[pc+3 : pc+7]))
			pc += 7
			va := vm.locals[a]
			vb := vm.locals[bb]
			cond := false
			switch mode {
			case 0:
				cond = va < vb
			case 1:
				cond = va <= vb
			case 2:
				cond = va > vb
			case 3:
				cond = va >= vb
			case 4:
				cond = va == vb
			case 5:
				cond = va != vb
			}
			if !cond {
				pc = pc + int(off)
			}
		case bytecode.OpAddI64:
			if err := vm.binOp(func(a, b int64) int64 { return a + b }); err != nil {
				return StatusStackUnderflow, err
			}
		case bytecode.OpSubI64:
			if err := vm.binOp(func(a, b int64) int64 { return a - b }); err != nil {
				return StatusStackUnderflow, err
			}
		case bytecode.OpMulI64:
			if err := vm.binOp(func(a, b int64) int64 { return a * b }); err != nil {
				return StatusStackUnderflow, err
			}
		case bytecode.OpDivI64:
			b, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			a, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if b == 0 {
				if newPC, ok := vm.throwException("division by zero"); ok {
					pc = newPC
					break
				}
				return StatusDivByZero, fmt.Errorf("divide by zero at pc=%d", pc)
			}
			vm.push(a / b)
		case bytecode.OpModI64:
			b, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			a, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if b == 0 {
				if newPC, ok := vm.throwException("mod by zero"); ok {
					pc = newPC
					break
				}
				return StatusDivByZero, fmt.Errorf("mod by zero at pc=%d", pc)
			}
			vm.push(a % b)
		case bytecode.OpNegI64:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(-v)
		case bytecode.OpPrintI64:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			fmt.Fprint(vm.out(), strconv.FormatInt(v, 10))
		case bytecode.OpPrintChar:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			fmt.Fprint(vm.out(), string(rune(v)))
		case bytecode.OpPrintStr:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			fmt.Fprint(vm.out(), vm.strByIdx(v))
		case bytecode.OpJmp:
			off, np, err := readI32(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			pc = np + int(off)
		case bytecode.OpJz:
			off, np, err := readI32(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if v == 0 {
				pc = np + int(off)
			} else {
				pc = np
			}
		case bytecode.OpJnz:
			off, np, err := readI32(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if v != 0 {
				pc = np + int(off)
			} else {
				pc = np
			}
		case bytecode.OpCmpEq:
			vm.cmpOp(func(a, b int64) bool { return a == b })
		case bytecode.OpCmpNe:
			vm.cmpOp(func(a, b int64) bool { return a != b })
		case bytecode.OpCmpLt:
			vm.cmpOp(func(a, b int64) bool { return a < b })
		case bytecode.OpCmpGt:
			vm.cmpOp(func(a, b int64) bool { return a > b })
		case bytecode.OpCmpLe:
			vm.cmpOp(func(a, b int64) bool { return a <= b })
		case bytecode.OpCmpGe:
			vm.cmpOp(func(a, b int64) bool { return a >= b })
		case bytecode.OpCall:
			off, np, err := readI32(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			// 栈布局（底→顶）：arg0, arg1, ..., argN-1, argc
			argc, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if argc < 0 || argc > 127 {
				return StatusInvalidArgc, fmt.Errorf("invalid argc %d", argc)
			}
			// 先保存当前帧（调用者的 locals 都要保留）
			vm.saveFrame()
			// 再把 argc 个实参反向搬运到 locals[0..argc-1]
			for i := argc - 1; i >= 0; i-- {
				v, perr := vm.pop()
				if perr != nil {
					return StatusStackUnderflow, perr
				}
				vm.locals[i] = v
			}
			vm.callStack = append(vm.callStack, np)
			pc = np + int(off)
		case bytecode.OpRet:
			st, err := vm.opReturnN(code, &pc, 1)
			if st != StatusOk {
				return st, err
			}
		case bytecode.OpRetN:
			// 布局：[op][u8 retN]
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			retN := code[pc]
			pc++
			st, err := vm.opReturnN(code, &pc, int(retN))
			if st != StatusOk {
				return st, err
			}
		// === 字符串表操作 ===
		case bytecode.OpStrNew:
			vm.strTable = append(vm.strTable, "")
			vm.push(int64(len(vm.strTable) - 1))
		case bytecode.OpStrAppendC:
			ch, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if idx > 0 && int(idx) < len(vm.strTable) {
				vm.strTable[idx] += string(rune(ch))
			}
			// 弹 [idx, ch] 压 0：编译器用 Dup 补回 idx，这里不能 push(idx)，
			// 否则每次 append 泄漏 1 个栈槽，污染后续 NewObj 的 classID
		case bytecode.OpStrLen:
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if idx > 0 && int(idx) < len(vm.strTable) {
				vm.push(int64(len([]rune(vm.strTable[idx]))))
			} else {
				vm.push(0)
			}
		case bytecode.OpStrGetC:
			pos, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			res := int64(0)
			if idx > 0 && int(idx) < len(vm.strTable) {
				runes := []rune(vm.strTable[idx])
				if pos >= 0 && int(pos) < len(runes) {
					res = int64(runes[pos])
				}
			}
			vm.push(res)
		case bytecode.OpStrDelete:
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if idx > 0 && int(idx) < len(vm.strTable) {
				vm.strTable[idx] = ""
			}
		// === System exec ===
		case bytecode.OpSystemExec:
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			cmd := ""
			if idx > 0 && int(idx) < len(vm.strTable) {
				cmd = vm.strTable[idx]
			}
			exitCode := int64(-1)
			if cmd != "" {
				var c *exec.Cmd
				if runtime.GOOS == "windows" {
					c = exec.Command("cmd.exe", "/c", cmd)
				} else {
					c = exec.Command("sh", "-c", cmd)
				}
				c.Stdout = vm.out()
				c.Stderr = os.Stderr
				if err := c.Run(); err != nil {
					if ee, ok := err.(*exec.ExitError); ok {
						exitCode = int64(ee.ExitCode())
					} else {
						exitCode = -1
					}
				} else {
					exitCode = 0
				}
			}
			vm.push(exitCode)
		case bytecode.OpSystemReadFile:
			// [path_str_idx] → [ok(0/1), content_str_idx]；软错误（不抛异常）
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			path := ""
			if idx > 0 && int(idx) < len(vm.strTable) {
				path = vm.strTable[idx]
			}
			if path == "" {
				vm.push(0)
				vm.push(vm.strIntern("read_file: empty path"))
				break
			}
			data, rerr := os.ReadFile(path)
			if rerr != nil {
				vm.push(0)
				vm.push(vm.strIntern(rerr.Error()))
				break
			}
			vm.push(1)
			vm.push(vm.strIntern(string(data)))
		// === OOP 操作 ===
		case bytecode.OpNewObj:
			st, err := vm.opNewObj(code, &pc)
			if st != StatusOk {
				return st, err
			}
		case bytecode.OpGetAttr:
			attrIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			objID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(vm.getAttr(objID, vm.strByIdx(attrIdx)))
		case bytecode.OpSetAttr:
			val, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			attrIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			objID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.setAttr(objID, vm.strByIdx(attrIdx), val)
			vm.push(val) // 赋值表达式的值：弹 3 压 1（编译器在语句级用 Pop 丢弃）
		case bytecode.OpInvoke:
			st, err := vm.opInvoke(code, &pc, false /*super*/)
			if st != StatusOk {
				return st, err
			}
		case bytecode.OpInvokeSuper:
			st, err := vm.opInvoke(code, &pc, true)
			if st != StatusOk {
				return st, err
			}
		case bytecode.OpInstanceOf:
			classID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			objID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(vm.instanceOf(objID, int32(classID)))
		case bytecode.OpGetStatic:
			attrIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			classID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(vm.getStatic(int32(classID), vm.strByIdx(attrIdx)))
		case bytecode.OpSetStatic:
			val, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			attrIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			classID, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.setStatic(int32(classID), vm.strByIdx(attrIdx), val)
		case bytecode.OpObjRelease:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.objRelease(int32(id))
		case bytecode.OpGo:
			st, err := vm.opGo(code, &pc)
			if st != StatusOk {
				return st, err
			}
		case bytecode.OpChanNew:
			capV, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(vm.Shared.chanNew(capV))
		case bytecode.OpChanPut:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.Shared.chanPut(id, v)
		case bytecode.OpChanGet:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			v, ok := vm.Shared.chanGet(id)
			if !ok {
				v = 0
			}
			vm.push(v)

		// ===== 异常处理 =====
		case bytecode.OpPushHandler:
			off, np, err := readI32(code, pc)
			if err != nil {
				return StatusTruncated, err
			}
			pc = np
			vm.handlers = append(vm.handlers, handlerFrame{
				handlerPC: pc + int(off), // 与 OpCall 一致：相对指令末尾
				sp:        vm.sp,
				csLen:     len(vm.callStack),
				framesLen: len(vm.frames),
			})
		case bytecode.OpPopHandler:
			if len(vm.handlers) > 0 {
				vm.handlers = vm.handlers[:len(vm.handlers)-1]
			}
		case bytecode.OpRaise:
			msgIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			msg := vm.strByIdx(msgIdx)
			if newPC, ok := vm.throwException(msg); ok {
				pc = newPC
				break
			}
			return StatusUncaught, fmt.Errorf("uncaught exception: %s", msg)

		// ===== GC 垃圾回收 =====
		case bytecode.OpGC:
			freed := vm.gcRun()
			vm.push(int64(freed))

		// ===== 指针操作 =====
		case bytecode.OpAddrOf:
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			pc++
			pid := vm.ptrAlloc()
			vm.ptrs[pid] = vm.locals[slot]
			vm.push(int64(pid))
		case bytecode.OpDerefLoad:
			pid, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if pid <= 0 || int(pid) >= len(vm.ptrs) {
				vm.push(0)
			} else {
				vm.push(vm.ptrs[pid])
			}
		case bytecode.OpDerefStore:
			val, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			pid, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if pid > 0 && int(pid) < len(vm.ptrs) {
				vm.ptrs[pid] = val
			}
			vm.push(val)

		// ===== 闭包（lambda block 形式）=====
		case bytecode.OpClosureNew:
			// 布局：[op][i32 entryPC][u8 nparams][u8 nlocals][u8 ncapture]
			// 栈：[cap0..capN-1]（cap0 在底）→ [closure_id]
			if pc+4+3 > len(code) {
				return StatusTruncated, nil
			}
			entryPC := int(binary.LittleEndian.Uint32(code[pc : pc+4]))
			pc += 4
			nparams := code[pc]
			pc++
			nlocals := code[pc]
			pc++
			ncapture := code[pc]
			pc++
			caps := make([]int64, ncapture)
			// 反向弹栈：栈顶是 capN-1
			for i := int(ncapture) - 1; i >= 0; i-- {
				v, err := vm.pop()
				if err != nil {
					return StatusStackUnderflow, err
				}
				caps[i] = v
			}
			cid := vm.closureAlloc()
			vm.closures[cid].EntryPC = int32(entryPC)
			vm.closures[cid].NParams = nparams
			vm.closures[cid].NLocals = nlocals
			vm.closures[cid].NCapture = ncapture
			vm.closures[cid].Captures = caps
			vm.push(int64(cid))
		case bytecode.OpClosureCall:
			// 栈序（底→顶）：[arg0..argM-1, argc, closure_id] → 结果留栈顶
			cid, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			argc, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if argc < 0 || argc > 127 {
				return StatusInvalidArgc, fmt.Errorf("invalid argc %d", argc)
			}
			if cid <= 0 || int(cid) >= len(vm.closures) || vm.closures[cid].RefCount <= 0 {
				// 无效闭包：弹掉参数，压 0
				for i := int64(0); i < argc; i++ {
					vm.pop()
				}
				vm.push(0)
				break
			}
			cl := vm.closures[cid]
			args := make([]int64, argc)
			for i := argc - 1; i >= 0; i-- {
				v, perr := vm.pop()
				if perr != nil {
					return StatusStackUnderflow, perr
				}
				args[i] = v
			}
			vm.saveFrame()
			vm.frames[len(vm.frames)-1].closureID = int32(cid)
			// args → locals[0..argc-1]（按 nparams 截断）
			for i := int64(0); i < argc && i < int64(cl.NParams); i++ {
				vm.locals[i] = args[i]
			}
			// captures → locals[nparams..nparams+ncapture-1]
			for i := uint8(0); i < cl.NCapture; i++ {
				slot := cl.NParams + i
				if int(slot) < 256 {
					vm.locals[slot] = cl.Captures[i]
				}
			}
			vm.callStack = append(vm.callStack, pc)
			pc = int(cl.EntryPC)
		case bytecode.OpLoadCapture:
			// 布局：[op][u8 slot]
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			pc++
			// 从当前帧的 closureID 取 captures[slot]
			cid := int32(-1)
			if len(vm.frames) > 0 {
				cid = vm.frames[len(vm.frames)-1].closureID
			}
			if cid < 0 || int(cid) >= len(vm.closures) || vm.closures[cid].RefCount <= 0 {
				vm.push(0)
			} else {
				cl := &vm.closures[cid]
				if int(slot) < len(cl.Captures) {
					vm.push(cl.Captures[slot])
				} else {
					vm.push(0)
				}
			}
		case bytecode.OpStoreCapture:
			// 布局：[op][u8 slot]，栈：[v] → []
			if pc >= len(code) {
				return StatusTruncated, nil
			}
			slot := code[pc]
			pc++
			val, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			cid := int32(-1)
			if len(vm.frames) > 0 {
				cid = vm.frames[len(vm.frames)-1].closureID
			}
			if cid >= 0 && int(cid) < len(vm.closures) && vm.closures[cid].RefCount > 0 {
				cl := &vm.closures[cid]
				if int(slot) < len(cl.Captures) {
					cl.Captures[slot] = val
				}
			}

		// ===== 列表容器 =====
		case bytecode.OpListNew:
			id, err := vm.listAlloc()
			if err != nil {
				return StatusStackOverflow, err
			}
			vm.push(id)
		case bytecode.OpListPush:
			// 栈：[list_id, val] → pop val, pop list_id, push list_id（支持链式）
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.lists[int(id)] = append(vm.lists[int(id)], v)
			vm.push(id)
		case bytecode.OpListGet:
			// 栈：[list_id, idx] → 先弹 idx（栈顶），再弹 list_id
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			l := vm.lists[int(id)]
			if int(idx) < 0 || int(idx) >= len(l) {
				vm.push(0)
			} else {
				vm.push(l[int(idx)])
			}
		case bytecode.OpListSet:
			// 栈：[list_id, idx, val] → pop val, pop idx, pop list_id, push list_id
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			l := vm.lists[int(id)]
			if int(idx) >= 0 && int(idx) < len(l) {
				l[int(idx)] = v
			}
			vm.push(id)
		case bytecode.OpListPop:
			// 栈：[list_id]
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			l := vm.lists[int(id)]
			if len(l) == 0 {
				vm.push(0)
			} else {
				last := l[len(l)-1]
				vm.lists[int(id)] = l[:len(l)-1]
				vm.push(last)
			}
		case bytecode.OpListLen:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(int64(len(vm.lists[int(id)])))
		case bytecode.OpListDeleteAt:
			// 栈：[list_id, idx] → pop idx, pop list_id, push list_id
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			l := vm.lists[int(id)]
			if int(idx) >= 0 && int(idx) < len(l) {
				vm.lists[int(id)] = append(l[:int(idx)], l[int(idx)+1:]...)
			}
			vm.push(id)
		case bytecode.OpListRelease:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.lists[int(id)] = nil
			vm.listFree = append(vm.listFree, int32(id))

		// ===== 字典容器 =====
		case bytecode.OpDictNew:
			id, err := vm.dictAlloc()
			if err != nil {
				return StatusStackOverflow, err
			}
			vm.push(id)
		case bytecode.OpDictPut:
			// 栈：[dict_id, key_str_idx, val] → pop val, pop key, pop dict_id, push dict_id
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			keyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.dicts[int(id)][vm.strByIdx(keyIdx)] = v
			vm.push(id)
		case bytecode.OpDictGet:
			// 栈：[dict_id, key_str_idx] → 先弹 key，再弹 dict_id
			keyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			var v int64
			if id >= 0 && int(id) < len(vm.dicts) {
				v = vm.dicts[int(id)][vm.strByIdx(keyIdx)]
			}
			vm.push(v)
		case bytecode.OpDictHas:
			// 栈：[dict_id, key_str_idx]
			keyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			ok := false
			if id >= 0 && int(id) < len(vm.dicts) {
				_, ok = vm.dicts[int(id)][vm.strByIdx(keyIdx)]
			}
			if ok {
				vm.push(1)
			} else {
				vm.push(0)
			}
		case bytecode.OpDictDelete:
			// 栈：[dict_id, key_str_idx] → pop key, pop dict_id, push dict_id
			keyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			delete(vm.dicts[int(id)], vm.strByIdx(keyIdx))
			vm.push(id)
		case bytecode.OpDictLen:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.push(int64(len(vm.dicts[int(id)])))
		case bytecode.OpDictRelease:
			id, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.dicts[int(id)] = nil
			vm.dictFree = append(vm.dictFree, int32(id))

		// ===== 字符串增强 =====
		case bytecode.OpStrFind:
			// 栈：[str, sub] → 先弹 sub（栈顶），再弹 str
			subIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			s := vm.strByIdx(sIdx)
			sub := vm.strByIdx(subIdx)
			bytePos := strings.Index(s, sub)
			if bytePos < 0 {
				vm.push(-1)
			} else {
				// 转换为 rune 索引（与 str.get_c/str.slice 一致）
				runePos := utf8.RuneCountInString(s[:bytePos])
				vm.push(int64(runePos))
			}
		case bytecode.OpStrSlice:
			// 栈：[str, start, end] → 先弹 end，再弹 start，最后弹 str
			end, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			start, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			runes := []rune(vm.strByIdx(sIdx))
			runeLen := int64(len(runes))
			if start < 0 {
				start = 0
			}
			if end > runeLen {
				end = runeLen
			}
			if start > end {
				start = end
			}
			newIdx := vm.strIntern(string(runes[start:end]))
			vm.push(newIdx)
		case bytecode.OpStrEqual:
			// 栈：[a, b] → 先弹 b（栈顶），再弹 a
			b, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			a, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			if vm.strByIdx(a) == vm.strByIdx(b) {
				vm.push(1)
			} else {
				vm.push(0)
			}
		case bytecode.OpStrNewFromIdx:
			idx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			// 编译器已经把字符串写入 StringTable，运行时 strTable = prog.StringTable + runtime 追加
			// 直接用 idx 即可；若越界则 push 空串
			if idx >= 0 && int(idx) < len(vm.strTable) {
				vm.push(idx)
			} else {
				vm.push(0)
			}
		case bytecode.OpStrTrim:
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			newIdx := vm.strIntern(strings.TrimSpace(vm.strByIdx(sIdx)))
			vm.push(newIdx)
		case bytecode.OpStrReplace:
			// 栈：[str, old, new] → 先弹 new，再弹 old，最后弹 str
			newIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			oldIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			newS := strings.ReplaceAll(vm.strByIdx(sIdx), vm.strByIdx(oldIdx), vm.strByIdx(newIdx))
			resIdx := vm.strIntern(newS)
			vm.push(resIdx)
		case bytecode.OpStrAppendStr:
			// 栈：[dst, src] → 先弹 src，再弹 dst → push concat(dst, src)
			srcIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			dstIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			concat := vm.strByIdx(dstIdx) + vm.strByIdx(srcIdx)
			resIdx := vm.strIntern(concat)
			vm.push(resIdx)

		// ===== 类型转换 / 时间 =====
		case bytecode.OpAtoi:
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			n, _ := strconv.ParseInt(vm.strByIdx(sIdx), 10, 64)
			vm.push(n)
		case bytecode.OpItoa:
			v, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			idx := vm.strIntern(strconv.FormatInt(v, 10))
			vm.push(idx)
		case bytecode.OpSleep:
			ms, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			time.Sleep(time.Duration(ms) * time.Millisecond)
		case bytecode.OpNow:
			vm.push(time.Now().UnixMilli())

		// ===== HTTP =====
		case bytecode.OpHttpRequest:
			// 栈：[url, method, body] → 先弹 body（栈顶），再弹 method，最后弹 url
			bodyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			methodIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			urlIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			u := vm.strByIdx(urlIdx)
			method := strings.ToUpper(vm.strByIdx(methodIdx))
			body := vm.strByIdx(bodyIdx)
			if method == "" {
				if body == "" {
					method = "GET"
				} else {
					method = "POST"
				}
			}
			// 构建请求
			var bodyR io.Reader
			if body != "" {
				bodyR = strings.NewReader(body)
			}
			req, err := http.NewRequest(method, u, bodyR)
			if err != nil {
				// 构建虚拟请求给 cookie jar 初始化
				vm.push(0)
				vm.push(vm.strIntern(""))
				break
			}
			if vm.httpUA != "" {
				req.Header.Set("User-Agent", vm.httpUA)
			}
			// 自动加 Referer（同源 origin）
			if req.Header.Get("Referer") == "" {
				if parsed, pe := url.Parse(u); pe == nil {
					req.Header.Set("Referer", parsed.Scheme+"://"+parsed.Host)
				} else {
					req.Header.Set("Referer", u)
				}
			}
			for k, v := range vm.httpHdrs {
				req.Header.Set(k, v)
			}
			if method == "POST" && body != "" && req.Header.Get("Content-Type") == "" {
				req.Header.Set("Content-Type", "application/x-www-form-urlencoded; charset=UTF-8")
			}
			resp, err := vm.httpClient.Do(req)
			if err != nil {
				vm.push(0)
				vm.push(vm.strIntern(""))
				break
			}
			defer resp.Body.Close()
			data, _ := io.ReadAll(resp.Body)
			bodyStr := vm.strIntern(string(data))
			// 刷新 cookie 映射（从 jar 中提取所有 cookie）
			vm.refreshCookies(u)
			vm.push(int64(resp.StatusCode))
			vm.push(bodyStr)
		case bytecode.OpHttpSetUA:
			sIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.httpUA = vm.strByIdx(sIdx)
		case bytecode.OpHttpAddHdr:
			keyIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			valIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			vm.httpHdrs[vm.strByIdx(keyIdx)] = vm.strByIdx(valIdx)
		case bytecode.OpHttpGetCookie:
			// 栈：[name] → [value]  从缓存的 cookie 映射中读取
			nameIdx, err := vm.pop()
			if err != nil {
				return StatusStackUnderflow, err
			}
			name := vm.strByIdx(nameIdx)
			val := vm.httpCookies[name]
			vm.push(vm.strIntern(val))
		case bytecode.OpHttpClear:
			// 清空 cookie jar 和缓存映射
			jar, _ := cookiejar.New(nil)
			vm.httpClient.Jar = jar
			vm.httpCookies = map[string]string{}
			for k := range vm.httpHdrs {
				delete(vm.httpHdrs, k)
			}

		default:
			return StatusInvalidOpcode, fmt.Errorf("invalid opcode 0x%02X (%s) at pc=%d", byte(op), bytecode.OpName(op), pc-1)
		}
	}
	return StatusOk, nil
}

// --- String lookup by str_idx (from compiler StringTable) ---
func (vm *Interpreter) strByIdx(idx int64) string {
	if idx >= 0 && int(idx) < len(vm.strTable) {
		return vm.strTable[idx]
	}
	return ""
}

// refreshCookies 从 cookie jar 提取指定 URL 的所有 cookie 到 httpCookies 映射。
func (vm *Interpreter) refreshCookies(rawURL string) {
	pu, err := url.Parse(rawURL)
	if err != nil {
		return
	}
	for _, c := range vm.httpClient.Jar.Cookies(pu) {
		vm.httpCookies[c.Name] = c.Value
	}
}

func (vm *Interpreter) out() io.Writer {
	if vm.Output == nil {
		return os.Stdout
	}
	return vm.Output
}

// --- Stack operations ---
func (vm *Interpreter) push(v int64) {
	vm.stack = append(vm.stack, v)
	vm.sp++
}
func (vm *Interpreter) pop() (int64, error) {
	if vm.sp == 0 {
		return 0, fmt.Errorf("pop on empty stack")
	}
	vm.sp--
	v := vm.stack[vm.sp]
	vm.stack = vm.stack[:vm.sp]
	return v, nil
}
func (vm *Interpreter) popNo() {
	if vm.sp > 0 {
		vm.sp--
		vm.stack = vm.stack[:vm.sp]
	}
}
func (vm *Interpreter) peek() (int64, error) {
	if vm.sp == 0 {
		return 0, fmt.Errorf("peek on empty stack")
	}
	return vm.stack[vm.sp-1], nil
}
func (vm *Interpreter) binOp(fn func(a, b int64) int64) error {
	b, err := vm.pop()
	if err != nil {
		return err
	}
	a, err := vm.pop()
	if err != nil {
		return err
	}
	vm.push(fn(a, b))
	return nil
}
func (vm *Interpreter) cmpOp(fn func(a, b int64) bool) {
	b, _ := vm.pop()
	a, _ := vm.pop()
	if fn(a, b) {
		vm.push(1)
	} else {
		vm.push(0)
	}
}

// --- Frame save/restore (locals 0..255) ---
func (vm *Interpreter) saveFrame() {
	var f savedFrame
	copy(f.saved[:], vm.locals[0:256])
	f.pendingNewObj = -1 // 默认：非构造函数帧
	f.closureID = -1     // 默认：非闭包帧
	vm.frames = append(vm.frames, f)
}
func (vm *Interpreter) restoreFrame() {
	if len(vm.frames) == 0 {
		return
	}
	f := vm.frames[len(vm.frames)-1]
	vm.frames = vm.frames[:len(vm.frames)-1]
	copy(vm.locals[0:256], f.saved[:])
}

// opReturnN —— OpRet(1)/OpRetN(n) 的统一返回路径：
// 从栈顶取 retN 个返回值 → 清理本帧 handler → 恢复调用帧 → 回写返回值。
// 帧深度为 curD=len(frames)；返回前运行归属于该帧的 defer（特性 3）。
func (vm *Interpreter) opReturnN(code []byte, pc *int, retN int) (Status, error) {
	if retN < 1 {
		retN = 1
	}
	// 丢弃当前帧及更深帧残留的 handler（return 穿出 try 时的泄漏防护）：
	// 当前帧的 handler 创建于 len(frames)==D（D=当前帧深度），Ret 时 len(frames)==D
	for len(vm.handlers) > 0 && vm.handlers[len(vm.handlers)-1].framesLen >= len(vm.frames) {
		vm.handlers = vm.handlers[:len(vm.handlers)-1]
	}
	rets := make([]int64, retN)
	for i := retN - 1; i >= 0; i-- {
		v, err := vm.pop()
		if err != nil {
			return StatusStackUnderflow, err
		}
		rets[i] = v
	}
	// 在 restoreFrame 弹出帧前，先 peek 当前帧是否是构造函数返回
	// （构造函数返回时，用 objID 替代 constructor retval 压栈）
	pending := int32(-1)
	if len(vm.frames) > 0 {
		pending = vm.frames[len(vm.frames)-1].pendingNewObj
	}
	vm.restoreFrame()
	if len(vm.callStack) == 0 {
		// 顶层 return → 退出
		return StatusOk, nil
	}
	top := len(vm.callStack) - 1
	*pc = vm.callStack[top]
	vm.callStack = vm.callStack[:top]
	if pending >= 0 {
		vm.push(int64(pending))
	} else {
		for i := 0; i < retN; i++ {
			vm.push(rets[i])
		}
	}
	return StatusOk, nil
}

// --- Exception handling ---

// throwException 把异常投递到最近的 handler：展开到 try 进入时的快照
// （sp/callStack/frames），把消息 str_idx 压栈，返回 handler 起始 pc。
//
// locals 处理：
//   - 异常与 try 同帧（len(frames)==framesLen）：不回滚 locals——try 内的赋值保留；
//   - 异常来自更深调用帧：恢复 frames[framesLen].saved，即 try 帧发起该调用时的
//     locals 快照（OpCall saveFrame 保存的正是调用者视角），深帧的改动被丢弃。
//
// 无 handler 时返回 ok=false（调用方转为硬错误）。
func (vm *Interpreter) throwException(msg string) (int, bool) {
	if len(vm.handlers) == 0 {
		return 0, false
	}
	h := vm.handlers[len(vm.handlers)-1]
	vm.handlers = vm.handlers[:len(vm.handlers)-1]
	if len(vm.frames) > h.framesLen {
		copy(vm.locals[0:256], vm.frames[h.framesLen].saved[:])
	}
	vm.frames = vm.frames[:h.framesLen]
	vm.callStack = vm.callStack[:h.csLen]
	vm.sp = h.sp
	idx := vm.strIntern(msg)
	vm.push(idx)
	return h.handlerPC, true
}

// --- 高并发：go 线程 ---

// opGo 弹出 [args..., argc, method_str_idx]，起独立线程异步执行顶层方法。
//
// 子线程 = 新 Interpreter（独立栈/locals/对象表/字符串表副本），
// 共享：类元数据（加载后只读）、Shared（通道注册表 + WaitGroup）、输出。
// 方法在 Program.Exports 中按名查找（编译器 AddExport 生成，v3 序列化保留）。
func (vm *Interpreter) opGo(code []byte, pc *int) (Status, error) {
	nameIdx, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	argcV, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	argc := int(argcV)
	if argc < 0 {
		argc = 0
	}
	args := make([]int64, argc)
	for i := argc - 1; i >= 0; i-- {
		args[i], err = vm.pop()
		if err != nil {
			return StatusStackUnderflow, err
		}
	}
	name := vm.strByIdx(nameIdx)
	var exp *bytecode.Export
	if vm.prog != nil {
		for i := range vm.prog.Exports {
			if vm.prog.Exports[i].Name == name {
				exp = &vm.prog.Exports[i]
				break
			}
		}
	}
	if exp == nil {
		// 方法未找到：静默 no-op（与 opInvoke miss 语义一致）
		if vm.Trace {
			fmt.Fprintf(os.Stderr, "[go MISS name=%q]\n", name)
		}
		return StatusOk, nil
	}
	vm.Shared.wg.Add(1)
	go func(exp bytecode.Export, args []int64, name string) {
		defer vm.Shared.wg.Done()
		child := NewInterpreter()
		child.classes = vm.classes                             // 类元数据只读共享
		child.Shared = vm.Shared                               // 通道注册表 + 等待组
		child.strTable = append([]string(nil), vm.strTable...) // 独立副本（StrNew 扩展不竞态）
		child.Output = vm.Output
		child.MaxSteps = vm.MaxSteps
		child.Trace = vm.Trace
		child.prog = vm.prog
		child.isRoot = false
		for i, a := range args {
			child.locals[i] = a // locals[0..argc-1] = 参数
		}
		st, err := child.runCode(vm.prog, int(exp.CodeOffset))
		if err != nil && st != StatusOk {
			fmt.Fprintf(os.Stderr, "method: thread '%s' exited with error: %v (status=%v)\n", name, err, st)
		}
	}(*exp, args, name)
	return StatusOk, nil
}

// --- OOP 对象操作 ---
// strIntern 把字符串加入运行时 strTable（去重），返回索引；若已存在则返回现有索引
func (vm *Interpreter) strIntern(s string) int64 {
	for i, existing := range vm.strTable {
		if existing == s {
			return int64(i)
		}
	}
	vm.strTable = append(vm.strTable, s)
	return int64(len(vm.strTable) - 1)
}

// listAlloc 分配新列表 id（复用 free slot，否则追加）
func (vm *Interpreter) listAlloc() (int64, error) {
	vm.gcMaybeTrigger()
	if len(vm.listFree) > 0 {
		id := vm.listFree[len(vm.listFree)-1]
		vm.listFree = vm.listFree[:len(vm.listFree)-1]
		vm.lists[id] = []int64{}
		return int64(id), nil
	}
	id := int32(len(vm.lists))
	vm.lists = append(vm.lists, []int64{})
	return int64(id), nil
}

// dictAlloc 分配新字典 id
func (vm *Interpreter) dictAlloc() (int64, error) {
	vm.gcMaybeTrigger()
	if len(vm.dictFree) > 0 {
		id := vm.dictFree[len(vm.dictFree)-1]
		vm.dictFree = vm.dictFree[:len(vm.dictFree)-1]
		vm.dicts[id] = map[string]int64{}
		return int64(id), nil
	}
	id := int32(len(vm.dicts))
	vm.dicts = append(vm.dicts, map[string]int64{})
	return int64(id), nil
}

// ptrAlloc 分配新指针单元格 id
func (vm *Interpreter) ptrAlloc() int32 {
	vm.gcMaybeTrigger()
	if len(vm.ptrFree) > 0 {
		id := vm.ptrFree[len(vm.ptrFree)-1]
		vm.ptrFree = vm.ptrFree[:len(vm.ptrFree)-1]
		vm.ptrs[id] = 0
		return id
	}
	id := int32(len(vm.ptrs))
	vm.ptrs = append(vm.ptrs, 0)
	return id
}

func (vm *Interpreter) objAlloc(classID int32) int32 {
	vm.gcMaybeTrigger()
	var obj Object
	obj.ClassID = classID
	obj.RefCount = 1
	cm := vm.classes.Find(classID)
	if cm != nil {
		obj.Fields = make([]int64, cm.TotalInstanceSlots)
	}
	var id int32
	if len(vm.freeSlots) > 0 {
		id = vm.freeSlots[len(vm.freeSlots)-1]
		vm.freeSlots = vm.freeSlots[:len(vm.freeSlots)-1]
		vm.objects[id] = obj
	} else {
		id = int32(len(vm.objects))
		vm.objects = append(vm.objects, obj)
	}
	return id
}

// closureAlloc 分配新闭包 id（复用 free slot，否则追加）
func (vm *Interpreter) closureAlloc() int32 {
	vm.gcMaybeTrigger()
	var id int32
	if len(vm.closureFree) > 0 {
		id = vm.closureFree[len(vm.closureFree)-1]
		vm.closureFree = vm.closureFree[:len(vm.closureFree)-1]
		vm.closures[id].RefCount = 1
	} else {
		id = int32(len(vm.closures))
		vm.closures = append(vm.closures, Closure{RefCount: 1})
	}
	return id
}
func (vm *Interpreter) objGet(id int32) *Object {
	if id <= 0 || int(id) >= len(vm.objects) {
		return nil
	}
	return &vm.objects[id]
}
func (vm *Interpreter) objRelease(id int32) {
	o := vm.objGet(id)
	if o == nil {
		return
	}
	o.RefCount--
	if o.RefCount <= 0 {
		o.ClassID = -1
		o.Fields = nil
		vm.freeSlots = append(vm.freeSlots, id)
	}
}

// ============================================================
//  GC 垃圾回收（标记-清扫，保守式）
// ============================================================

// gcRun 执行一次完整的 GC 周期：标记根集 → 递归追踪 → 清扫未标记对象
// 返回回收的对象总数
func (vm *Interpreter) gcRun() int {
	if vm.gcDisabled {
		return 0
	}

	nObj := len(vm.objects)
	if vm.Trace {
		fmt.Fprintf(vm.Output, "[GC] nObj=%d nList=%d nDict=%d nPtr=%d nClosure=%d sp=%d frames=%d\n",
			nObj, len(vm.lists), len(vm.dicts), len(vm.ptrs), len(vm.closures), vm.sp, len(vm.frames))
	}
	nList := len(vm.lists)
	nDict := len(vm.dicts)
	nPtr := len(vm.ptrs)
	nClosure := len(vm.closures)

	// 确保标记位图够大
	if cap(vm.gcMarked) < nObj {
		vm.gcMarked = make([]bool, nObj)
	} else {
		vm.gcMarked = vm.gcMarked[:nObj]
	}
	if cap(vm.gcListMarked) < nList {
		vm.gcListMarked = make([]bool, nList)
	} else {
		vm.gcListMarked = vm.gcListMarked[:nList]
	}
	if cap(vm.gcDictMarked) < nDict {
		vm.gcDictMarked = make([]bool, nDict)
	} else {
		vm.gcDictMarked = vm.gcDictMarked[:nDict]
	}
	if cap(vm.gcPtrMarked) < nPtr {
		vm.gcPtrMarked = make([]bool, nPtr)
	} else {
		vm.gcPtrMarked = vm.gcPtrMarked[:nPtr]
	}
	if cap(vm.gcClosureMarked) < nClosure {
		vm.gcClosureMarked = make([]bool, nClosure)
	} else {
		vm.gcClosureMarked = vm.gcClosureMarked[:nClosure]
	}
	// 重置标记
	for i := range vm.gcMarked {
		vm.gcMarked[i] = false
	}
	for i := range vm.gcListMarked {
		vm.gcListMarked[i] = false
	}
	for i := range vm.gcDictMarked {
		vm.gcDictMarked[i] = false
	}
	for i := range vm.gcPtrMarked {
		vm.gcPtrMarked[i] = false
	}
	for i := range vm.gcClosureMarked {
		vm.gcClosureMarked[i] = false
	}

	// ---- 标记阶段：从根集出发 ----
	// 根 1: 操作数栈
	for i := 0; i < vm.sp; i++ {
		vm.gcMarkValue(vm.stack[i])
	}
	// 根 2: 局部变量
	for i := 0; i < 256; i++ {
		if vm.Trace && vm.locals[i] != 0 {
			fmt.Fprintf(vm.Output, "[GC] local[%d] = %d\n", i, vm.locals[i])
		}
		vm.gcMarkValue(vm.locals[i])
	}
	// 根 3: 保存的调用帧局部变量
	for _, f := range vm.frames {
		for i := 0; i < 256; i++ {
			vm.gcMarkValue(f.saved[i])
		}
		if f.pendingNewObj >= 0 {
			vm.gcMarkValue(int64(f.pendingNewObj))
		}
		if f.closureID >= 0 {
			vm.gcMarkValue(int64(f.closureID))
		}
	}
	// 根 4: 静态字段
	for cid := int32(0); cid < int32(len(vm.classes.classes)); cid++ {
		cm := vm.classes.Find(cid)
		sv := vm.classes.StaticValues(cid)
		if sv != nil {
			if vm.Trace && len(*sv) > 0 {
				name := ""
				if cm != nil {
					name = cm.Name
				}
				fmt.Fprintf(vm.Output, "[GC] static fields class %d (%s): %v\n", cid, name, *sv)
			}
			for _, v := range *sv {
				vm.gcMarkValue(v)
			}
		}
	}

	// ---- 清扫阶段 ----
	freedObj := 0
	freedList := 0
	freedDict := 0
	freedPtr := 0
	freedClosure := 0

	for i := 1; i < nObj; i++ {
		o := &vm.objects[i]
		if o.ClassID < 0 {
			continue // 已释放的空槽
		}
		if !vm.gcMarked[i] {
			o.ClassID = -1
			o.Fields = nil
			vm.freeSlots = append(vm.freeSlots, int32(i))
			freedObj++
		}
	}
	for i := 1; i < nList; i++ {
		if vm.lists[i] == nil {
			continue
		}
		if !vm.gcListMarked[i] {
			vm.lists[i] = nil
			vm.listFree = append(vm.listFree, int32(i))
			freedList++
		}
	}
	for i := 1; i < nDict; i++ {
		if vm.dicts[i] == nil {
			continue
		}
		if !vm.gcDictMarked[i] {
			vm.dicts[i] = nil
			vm.dictFree = append(vm.dictFree, int32(i))
			freedDict++
		}
	}
	for i := 1; i < nPtr; i++ {
		if !vm.gcPtrMarked[i] {
			vm.ptrs[i] = 0
			vm.ptrFree = append(vm.ptrFree, int32(i))
			freedPtr++
		}
	}
	for i := 1; i < nClosure; i++ {
		if vm.closures[i].RefCount <= 0 {
			continue // 已回收的空槽
		}
		if !vm.gcClosureMarked[i] {
			vm.closures[i].RefCount = 0
			vm.closures[i].Captures = nil
			vm.closureFree = append(vm.closureFree, int32(i))
			freedClosure++
		}
	}

	// 更新统计
	vm.gcStats.TotalRuns++
	vm.gcStats.LastFreed = freedObj
	vm.gcStats.LastListFreed = freedList
	vm.gcStats.LastDictFreed = freedDict
	vm.gcAllocCount = 0

	if vm.Trace {
		fmt.Fprintf(vm.Output, "[GC] swept: obj=%d list=%d dict=%d ptr=%d closure=%d\n", freedObj, freedList, freedDict, freedPtr, freedClosure)
	}

	return freedObj + freedList + freedDict + freedPtr + freedClosure
}

// gcMarkValue 保守式标记：将值视为潜在的对象/列表/字典 ID 尝试标记
func (vm *Interpreter) gcMarkValue(v int64) {
	if v <= 0 {
		return
	}
	uid := int32(v)
	// 尝试标记对象
	if int(uid) < len(vm.gcMarked) {
		o := &vm.objects[uid]
		if o.ClassID >= 0 && !vm.gcMarked[uid] {
			vm.gcMarked[uid] = true
			if vm.Trace {
				fmt.Fprintf(vm.Output, "[GC] mark obj %d (class=%d fields=%d)\n", uid, o.ClassID, len(o.Fields))
			}
			// 递归标记对象的字段
			for _, f := range o.Fields {
				vm.gcMarkValue(f)
			}
		}
	}
	// 尝试标记列表
	if int(uid) < len(vm.gcListMarked) {
		if vm.lists[uid] != nil && !vm.gcListMarked[uid] {
			vm.gcListMarked[uid] = true
			for _, e := range vm.lists[uid] {
				vm.gcMarkValue(e)
			}
		}
	}
	// 尝试标记字典
	if int(uid) < len(vm.gcDictMarked) {
		if vm.dicts[uid] != nil && !vm.gcDictMarked[uid] {
			vm.gcDictMarked[uid] = true
			for _, val := range vm.dicts[uid] {
				vm.gcMarkValue(val)
			}
		}
	}
	// 尝试标记指针
	if int(uid) < len(vm.gcPtrMarked) {
		if !vm.gcPtrMarked[uid] {
			vm.gcPtrMarked[uid] = true
			vm.gcMarkValue(vm.ptrs[uid])
		}
	}
	// 尝试标记闭包（id 合法且存活 → 递归标记其捕获的 Captures）
	if int(uid) < len(vm.gcClosureMarked) {
		cl := &vm.closures[uid]
		if cl.RefCount > 0 && !vm.gcClosureMarked[uid] {
			vm.gcClosureMarked[uid] = true
			if vm.Trace {
				fmt.Fprintf(vm.Output, "[GC] mark closure %d (captures=%d)\n", uid, len(cl.Captures))
			}
			for _, c := range cl.Captures {
				vm.gcMarkValue(c)
			}
		}
	}
}

// gcMaybeTrigger 在每次分配时调用，达到阈值则自动触发 GC
func (vm *Interpreter) gcMaybeTrigger() {
	if vm.gcDisabled || vm.gcThreshold <= 0 {
		return
	}
	vm.gcAllocCount++
	if vm.gcAllocCount >= vm.gcThreshold {
		vm.gcRun()
	}
}

func (vm *Interpreter) getAttr(objID int64, name string) int64 {
	o := vm.objGet(int32(objID))
	if o == nil {
		return 0
	}
	cm := vm.classes.Find(o.ClassID)
	if cm == nil {
		return 0
	}
	f, ok := cm.FieldTable[name]
	if !ok || f.IsStatic {
		return 0
	}
	if f.Slot >= 0 && f.Slot < len(o.Fields) {
		return o.Fields[f.Slot]
	}
	return 0
}
func (vm *Interpreter) setAttr(objID int64, name string, val int64) {
	o := vm.objGet(int32(objID))
	if o == nil {
		return
	}
	cm := vm.classes.Find(o.ClassID)
	if cm == nil {
		return
	}
	f, ok := cm.FieldTable[name]
	if !ok || f.IsStatic {
		return
	}
	if f.Slot >= 0 && f.Slot < len(o.Fields) {
		o.Fields[f.Slot] = val
	}
}
func (vm *Interpreter) getStatic(classID int32, name string) int64 {
	cm := vm.classes.Find(classID)
	if cm == nil {
		return 0
	}
	f, ok := cm.Fields[name]
	if !ok || !f.IsStatic {
		// 尝试继承链查找：直接 FieldTable 没收录 static 的情况，退而遍历父链
		cur := cm
		for cur != nil {
			if ff, kk := cur.Fields[name]; kk && ff.IsStatic {
				f = ff
				ok = true
				classID = cur.ID
				break
			}
			if cur.ParentID < 0 {
				break
			}
			cur = vm.classes.Find(cur.ParentID)
		}
		if !ok {
			return 0
		}
	}
	sv := vm.classes.StaticValues(classID)
	if f.Slot < len(*sv) {
		return (*sv)[f.Slot]
	}
	return 0
}
func (vm *Interpreter) setStatic(classID int32, name string, val int64) {
	cm := vm.classes.Find(classID)
	if cm == nil {
		return
	}
	f, ok := cm.Fields[name]
	if !ok || !f.IsStatic {
		cur := cm
		for cur != nil {
			if ff, kk := cur.Fields[name]; kk && ff.IsStatic {
				f = ff
				ok = true
				classID = cur.ID
				break
			}
			if cur.ParentID < 0 {
				break
			}
			cur = vm.classes.Find(cur.ParentID)
		}
		if !ok {
			return
		}
	}
	sv := vm.classes.StaticValues(classID)
	for f.Slot >= len(*sv) {
		*sv = append(*sv, 0)
	}
	(*sv)[f.Slot] = val
}

func (vm *Interpreter) instanceOf(objID int64, classID int32) int64 {
	o := vm.objGet(int32(objID))
	if o == nil {
		return 0
	}
	cur := vm.classes.Find(o.ClassID)
	target := vm.classes.Find(classID)
	if target == nil {
		return 0
	}
	for cur != nil {
		if cur.ID == target.ID {
			return 1
		}
		for _, iid := range cur.Interfaces {
			if iid == target.ID {
				return 1
			}
			// 接口继承链也遍历
			ifi := vm.classes.Find(iid)
			for ifi != nil {
				if ifi.ID == target.ID {
					return 1
				}
				if ifi.ParentID < 0 {
					break
				}
				ifi = vm.classes.Find(ifi.ParentID)
			}
		}
		if cur.ParentID < 0 {
			break
		}
		cur = vm.classes.Find(cur.ParentID)
	}
	return 0
}

// OpNewObj: 栈：[class_id, argc, arg1..argN] → [obj_id]
func (vm *Interpreter) opNewObj(code []byte, pc *int) (Status, error) {
	// 弹出 argc + args（先弹 argc，再弹 args）
	argcV, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	argc := int(argcV)
	if argc < 0 {
		argc = 0
	}
	args := make([]int64, argc)
	for i := argc - 1; i >= 0; i-- {
		if args[i], err = vm.pop(); err != nil {
			return StatusStackUnderflow, err
		}
	}
	classIDv, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	classID := int32(classIDv)
	objID := vm.objAlloc(classID)
	// 调用 init/构造
	cm := vm.classes.Find(classID)
	if cm != nil && cm.InitOffset >= 0 {
		// slot 0 = this，之后填参数
		vm.saveFrame()
		// 标记：该帧 Ret 时用 objID 替换 constructor retval 压栈（保证栈深度=1，且调用方拿到 objID）
		vm.frames[len(vm.frames)-1].pendingNewObj = objID
		vm.locals[0] = int64(objID)
		for i := 0; i < argc && i < int(cm.InitNumParams); i++ {
			vm.locals[1+uint8(i)] = args[i]
		}
		vm.callStack = append(vm.callStack, *pc)
		*pc = int(cm.InitOffset)
		// 不在这里 push(objID)：构造函数的 Ret 会通过 pendingNewObj 机制压入正确的 objID
		// 避免栈上残留 [objID, retval(0)] 两个值导致调用方误取 retval(0) 作 objID
	} else {
		// 无构造函数：直接 push objID
		vm.push(int64(objID))
	}
	return StatusOk, nil
}

// OpInvoke / InvokeSuper: 栈：[obj_id, method_str_idx, argc, arg1..argN] → [ret]
func (vm *Interpreter) opInvoke(code []byte, pc *int, isSuper bool) (Status, error) {
	argcV, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	argc := int(argcV)
	if argc < 0 {
		argc = 0
	}
	args := make([]int64, argc)
	for i := argc - 1; i >= 0; i-- {
		if args[i], err = vm.pop(); err != nil {
			return StatusStackUnderflow, err
		}
	}
	methodIdx, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	methodName := vm.strByIdx(methodIdx)
	objIDv, err := vm.pop()
	if err != nil {
		return StatusStackUnderflow, err
	}
	objID := int32(objIDv)
	o := vm.objGet(objID)
	var m ClassMethod
	var found bool
	var classID int32 = -1
	if o == nil {
		// null 对象 → 当作 class 静态方法调用（obj_id 实际是 class_id）
		classID = int32(objIDv)
		cm := vm.classes.Find(classID)
		if cm != nil {
			m, found = cm.VTable[methodName]
			// 但 vtable 里可能没有静态方法（我们只在 Methods 里写），所以额外查 Methods
			if !found {
				m, found = cm.Methods[methodName]
			}
		}
	} else {
		classID = o.ClassID
		cm := vm.classes.Find(classID)
		startClass := cm
		if isSuper && cm != nil && cm.ParentID >= 0 {
			startClass = vm.classes.Find(cm.ParentID)
		}
		if startClass != nil {
			// 先查 vtable（实例方法）
			m, found = startClass.VTable[methodName]
			// 再查 Methods（可能含静态但被我们未合并的）
			if !found {
				m, found = startClass.Methods[methodName]
			}
		}
	}
	if !found {
		if vm.Trace {
			fmt.Fprintf(os.Stderr, "[invoke MISS name=%q objID=%d o==nil=%v classID=%d]\n",
				methodName, objIDv, o == nil, classID)
		}
		vm.push(0)
		return StatusOk, nil
	}
	if m.CodeOffset < 0 {
		// 抽象方法：错误
		vm.push(0)
		return StatusOk, nil
	}
	// 调用方法
	vm.saveFrame()
	if !m.IsStatic {
		vm.locals[0] = int64(objID) // this
		for i := 0; i < argc && i < int(m.NumParams); i++ {
			vm.locals[1+uint8(i)] = args[i]
		}
	} else {
		for i := 0; i < argc && i < int(m.NumParams); i++ {
			vm.locals[uint8(i)] = args[i]
		}
	}
	vm.callStack = append(vm.callStack, *pc)
	*pc = int(m.CodeOffset)
	return StatusOk, nil
}

func readI64(data []byte, off int) (int64, int, error) {
	if off+8 > len(data) {
		return 0, off, fmt.Errorf("truncated i64")
	}
	v := int64(binary.LittleEndian.Uint64(data[off : off+8]))
	return v, off + 8, nil
}

// RunFile 简化：从 .mt 文件读 → 编译 → 执行
// （此函数在 vm 包内不方便引入 lexer/parser/compiler 循环依赖，
// 实际主入口在 cmd 中完成。这里保留：Run(program)）

// DumpStackString 调试：栈内容 → 字符串（调试辅助）
func (vm *Interpreter) DumpStackString() string {
	var sb strings.Builder
	sb.WriteString("[")
	for i, v := range vm.stack {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(strconv.FormatInt(v, 10))
	}
	sb.WriteString("]")
	return sb.String()
}

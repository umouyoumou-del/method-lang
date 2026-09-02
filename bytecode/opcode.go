// Package bytecode —— Method Language Runtime (MLR) 字节码定义
//
// .mlr 文件格式（小端）:
//
//	偏移  长度  字段
//	0     4     Magic: "MLR\0"
//	4     2     Version (uint16)
//	6     4     Code length (uint32)
//	10    N     Bytecode 指令流
//
// 指令为栈式：操作数从虚拟栈取，结果压回。每条指令 1 字节操作码。
package bytecode

import "encoding/binary"

// 文件魔数与字节码版本
const (
	Magic           uint32 = 0x00524C4D // 'M','L','R','\0' (LE)
	BytecodeVersion uint16 = 3
)

// 伪指令段 tag
const (
	PseudoTagEnd       uint16 = 0
	PseudoTagClassMeta uint16 = 1
)

// Op —— 操作码（栈式虚拟机）
type Op uint8

const (
	OpNop        Op = 0x00
	OpPushI64    Op = 0x01 // i64 imm
	OpPop        Op = 0x02
	OpDup        Op = 0x03
	OpLoadLocal  Op = 0x04 // u8 slot
	OpStoreLocal Op = 0x05 // u8 slot

	OpAddLocalLocal Op = 0x06 // u8 a, u8 b : locals[a] += locals[b]
	OpIncLocalImm8  Op = 0x07 // u8 slot, i8 imm
	OpJccLocalLocal Op = 0x08 // u8 a, u8 b, u8 mode(0=lt,1=le,2=gt,3=ge,4=eq,5=ne), i32 off
	OpSwap          Op = 0x09 // swap top two stack items

	OpAddI64 Op = 0x10
	OpSubI64 Op = 0x11
	OpMulI64 Op = 0x12
	OpDivI64 Op = 0x13
	OpModI64 Op = 0x14
	OpNegI64 Op = 0x15

	OpPrintI64  Op = 0x20
	OpPrintChar Op = 0x21
	OpPrintStr  Op = 0x22 // [str_idx] → []；打印字符串内容

	OpJmp Op = 0x30 // i32 offset
	OpJz  Op = 0x31
	OpJnz Op = 0x32

	OpCmpEq Op = 0x40
	OpCmpLt Op = 0x41
	OpCmpGt Op = 0x42
	OpCmpLe Op = 0x43
	OpCmpGe Op = 0x44
	OpCmpNe Op = 0x45

	OpCall Op = 0x50 // i32 offset
	OpRet  Op = 0x51

	// 字符串表操作（索引 i64 引用字符串对象）
	OpStrNew        Op = 0x60
	OpStrAppendC    Op = 0x61 // [str_idx, char] → [str_idx]
	OpStrLen        Op = 0x62
	OpStrGetC       Op = 0x63
	OpStrDelete     Op = 0x64
	OpStrFind       Op = 0x65 // [str_idx, sub_str_idx] → [pos or -1]
	OpStrSlice      Op = 0x66 // [str_idx, start, end] → [new_str_idx]
	OpStrEqual      Op = 0x67 // [a_str_idx, b_str_idx] → [bool]
	OpStrNewFromIdx Op = 0x68 // [compiled_string_idx(i64)] → [str_idx]（直接用运行时字符串表条目）
	OpStrTrim       Op = 0x69 // [str_idx] → [new_str_idx]  去首尾空白
	OpStrReplace    Op = 0x6A // [str_idx, old_str_idx, new_str_idx] → [new_str_idx]
	OpStrAppendStr  Op = 0x6B // [dst_str_idx, src_str_idx] → [new_str_idx]（拼接两个字符串）

	// HTTP 客户端操作（简化一步式：OpHttpRequest 底层用 Go net/http + cookie jar）
	OpHttpRequest    Op = 0x70 // [url_str_idx, method_str_idx, body_str_idx] → [status, body_str_idx]
	OpHttpSetUA      Op = 0x71 // [ua_str_idx] → []（设置全局 User-Agent）
	OpHttpAddHdr     Op = 0x72 // [key_str_idx, val_str_idx] → []（添加全局默认 header）
	OpHttpGetCookie  Op = 0x73 // [name_str_idx] → [value_str_idx]（从 cookie jar 读 cookie）
	OpHttpClear      Op = 0x74 // [] → []（清空 cookie jar）
	OpSystemExec     Op = 0x79
	OpSystemReadFile Op = 0x7A // [path_str_idx] → [ok(0/1), content_str_idx]（读文本文件）

	// 列表容器
	OpListNew      Op = 0xA0 // [] → [list_id]
	OpListPush     Op = 0xA1 // [list_id, val] → []
	OpListGet      Op = 0xA2 // [list_id, idx] → [val]
	OpListSet      Op = 0xA3 // [list_id, idx, val] → []
	OpListPop      Op = 0xA4 // [list_id] → [val]  弹出末尾
	OpListLen      Op = 0xA5 // [list_id] → [len]
	OpListDeleteAt Op = 0xA6 // [list_id, idx] → []
	OpListRelease  Op = 0xA7 // [list_id] → []

	// 字典容器（key 为字符串 str_idx）
	OpDictNew     Op = 0xB0 // [] → [dict_id]
	OpDictPut     Op = 0xB1 // [dict_id, key_str_idx, val] → []
	OpDictGet     Op = 0xB2 // [dict_id, key_str_idx] → [val]（key 不存在返回 0）
	OpDictHas     Op = 0xB3 // [dict_id, key_str_idx] → [bool]
	OpDictDelete  Op = 0xB4 // [dict_id, key_str_idx] → []
	OpDictLen     Op = 0xB5 // [dict_id] → [len]
	OpDictRelease Op = 0xB6 // [dict_id] → []

	// 类型转换 / 时间
	OpAtoi  Op = 0xC0 // [str_idx] → [i64]（失败返回 0）
	OpItoa  Op = 0xC1 // [i64] → [str_idx]
	OpSleep Op = 0xC2 // [ms] → []
	OpNow   Op = 0xC3 // [] → [unix_ms]

	// OOP 操作
	OpNewObj      Op = 0x81 // [class_id, argc, arg1..argN] → [obj_id]
	OpGetAttr     Op = 0x82 // [obj_id, attr_str_idx] → [value]
	OpSetAttr     Op = 0x83 // [obj_id, attr_str_idx, value] → [value]
	OpInvoke      Op = 0x84 // [obj_id, method_str_idx, argc, arg1..argN] → [ret]
	OpInstanceOf  Op = 0x85
	OpGetStatic   Op = 0x86
	OpSetStatic   Op = 0x87
	OpInvokeSuper Op = 0x88
	OpObjRelease  Op = 0x89

	// 高并发操作（Go 风格线程 + 通道）
	OpGo      Op = 0x90 // [arg1..argN, argc, method_str_idx] → [0]；新线程异步执行顶层方法
	OpChanNew Op = 0x91 // [capacity] → [chan_id]
	OpChanPut Op = 0x92 // [chan_id, value] → []；满则阻塞
	OpChanGet Op = 0x93 // [chan_id] → [value]；空则阻塞

	// 异常处理（handler 栈模型）
	OpPushHandler Op = 0x95 // i32 handler offset；记录 sp/帧快照，压入 handler 栈
	OpPopHandler  Op = 0x96 // 弹出 handler（try 体正常结束）
	OpRaise       Op = 0x97 // [msg_str_idx] → 抛出异常；无 handler 则硬错误

	OpHalt Op = 0xFF
)

// OpName 返回操作码名称
func OpName(op Op) string {
	switch op {
	case OpNop:
		return "Nop"
	case OpPushI64:
		return "PushI64"
	case OpPop:
		return "Pop"
	case OpDup:
		return "Dup"
	case OpSwap:
		return "Swap"
	case OpLoadLocal:
		return "LoadLocal"
	case OpStoreLocal:
		return "StoreLocal"
	case OpAddLocalLocal:
		return "AddLocalLocal"
	case OpIncLocalImm8:
		return "IncLocalImm8"
	case OpJccLocalLocal:
		return "JccLocalLocal"
	case OpAddI64:
		return "AddI64"
	case OpSubI64:
		return "SubI64"
	case OpMulI64:
		return "MulI64"
	case OpDivI64:
		return "DivI64"
	case OpModI64:
		return "ModI64"
	case OpNegI64:
		return "NegI64"
	case OpPrintI64:
		return "PrintI64"
	case OpPrintChar:
		return "PrintChar"
	case OpPrintStr:
		return "PrintStr"
	case OpJmp:
		return "Jmp"
	case OpJz:
		return "Jz"
	case OpJnz:
		return "Jnz"
	case OpCmpEq:
		return "CmpEq"
	case OpCmpLt:
		return "CmpLt"
	case OpCmpGt:
		return "CmpGt"
	case OpCmpLe:
		return "CmpLe"
	case OpCmpGe:
		return "CmpGe"
	case OpCmpNe:
		return "CmpNe"
	case OpCall:
		return "Call"
	case OpRet:
		return "Ret"
	case OpStrNew:
		return "StrNew"
	case OpStrAppendC:
		return "StrAppendC"
	case OpStrLen:
		return "StrLen"
	case OpStrGetC:
		return "StrGetC"
	case OpStrDelete:
		return "StrDelete"
	case OpStrFind:
		return "StrFind"
	case OpStrSlice:
		return "StrSlice"
	case OpStrEqual:
		return "StrEqual"
	case OpStrNewFromIdx:
		return "StrNewFromIdx"
	case OpStrTrim:
		return "StrTrim"
	case OpStrReplace:
		return "StrReplace"
	case OpStrAppendStr:
		return "StrAppendStr"
	case OpHttpRequest:
		return "HttpRequest"
	case OpHttpSetUA:
		return "HttpSetUA"
	case OpHttpAddHdr:
		return "HttpAddHdr"
	case OpHttpGetCookie:
		return "HttpGetCookie"
	case OpHttpClear:
		return "HttpClear"
	case OpSystemExec:
		return "SystemExec"
	case OpSystemReadFile:
		return "SystemReadFile"
	case OpListNew:
		return "ListNew"
	case OpListPush:
		return "ListPush"
	case OpListGet:
		return "ListGet"
	case OpListSet:
		return "ListSet"
	case OpListPop:
		return "ListPop"
	case OpListLen:
		return "ListLen"
	case OpListDeleteAt:
		return "ListDeleteAt"
	case OpListRelease:
		return "ListRelease"
	case OpDictNew:
		return "DictNew"
	case OpDictPut:
		return "DictPut"
	case OpDictGet:
		return "DictGet"
	case OpDictHas:
		return "DictHas"
	case OpDictDelete:
		return "DictDelete"
	case OpDictLen:
		return "DictLen"
	case OpDictRelease:
		return "DictRelease"
	case OpAtoi:
		return "Atoi"
	case OpItoa:
		return "Itoa"
	case OpSleep:
		return "Sleep"
	case OpNow:
		return "Now"
	case OpNewObj:
		return "NewObj"
	case OpGetAttr:
		return "GetAttr"
	case OpSetAttr:
		return "SetAttr"
	case OpInvoke:
		return "Invoke"
	case OpInstanceOf:
		return "InstanceOf"
	case OpGetStatic:
		return "GetStatic"
	case OpSetStatic:
		return "SetStatic"
	case OpInvokeSuper:
		return "InvokeSuper"
	case OpObjRelease:
		return "ObjRelease"
	case OpGo:
		return "Go"
	case OpChanNew:
		return "ChanNew"
	case OpChanPut:
		return "ChanPut"
	case OpChanGet:
		return "ChanGet"
	case OpPushHandler:
		return "PushHandler"
	case OpPopHandler:
		return "PopHandler"
	case OpRaise:
		return "Raise"
	case OpHalt:
		return "Halt"
	}
	return "?"
}

// InstructionSize 返回某操作码占用的字节数（含操作码+操作数）
func InstructionSize(op Op) int {
	switch op {
	case OpNop, OpPop, OpDup, OpSwap:
		return 1
	case OpPushI64:
		return 1 + 8
	case OpLoadLocal, OpStoreLocal:
		return 1 + 1
	case OpAddLocalLocal:
		return 1 + 2
	case OpIncLocalImm8:
		return 1 + 2
	case OpJccLocalLocal:
		return 1 + 3 + 4
	case OpAddI64, OpSubI64, OpMulI64, OpDivI64, OpModI64, OpNegI64:
		return 1
	case OpPrintI64, OpPrintChar:
		return 1
	case OpPrintStr:
		return 1
	case OpJmp, OpJz, OpJnz:
		return 1 + 4
	case OpCmpEq, OpCmpLt, OpCmpGt, OpCmpLe, OpCmpGe, OpCmpNe:
		return 1
	case OpCall:
		return 1 + 4
	case OpRet:
		return 1
	case OpStrNew, OpStrAppendC, OpStrLen, OpStrGetC, OpStrDelete,
		OpStrFind, OpStrSlice, OpStrEqual, OpStrNewFromIdx, OpStrTrim, OpStrReplace, OpStrAppendStr,
		OpHttpRequest, OpHttpSetUA, OpHttpAddHdr, OpHttpGetCookie, OpHttpClear, OpSystemExec,
		OpSystemReadFile,
		OpListNew, OpListPush, OpListGet, OpListSet, OpListPop, OpListLen, OpListDeleteAt, OpListRelease,
		OpDictNew, OpDictPut, OpDictGet, OpDictHas, OpDictDelete, OpDictLen, OpDictRelease,
		OpAtoi, OpItoa, OpSleep, OpNow:
		return 1
	case OpNewObj, OpGetAttr, OpSetAttr, OpInvoke, OpInstanceOf,
		OpGetStatic, OpSetStatic, OpInvokeSuper, OpObjRelease:
		return 1
	case OpGo, OpChanNew, OpChanPut, OpChanGet:
		return 1
	case OpPushHandler:
		return 1 + 4
	case OpPopHandler, OpRaise:
		return 1
	case OpHalt:
		return 1
	}
	return 0
}

// Export —— 函数导出表项
type Export struct {
	Name       string
	CodeOffset int32
	NumLocals  uint8
	NumParams  uint8
}

// Program —— 已加载的 .mlr 程序
type Program struct {
	Valid            bool
	Version          uint16
	Code             []byte
	Pseudo           []byte // OOP 伪指令段
	MethodBlockStart uint32
	Exports          []Export
	StringTable      []string // 字符串表（编译期生成，索引0=空串）
}

// Serialize 将 Program 序列化为 .mlr 文件字节
func (p *Program) Serialize() []byte {
	codeLen := len(p.Code)
	pseudoLen := len(p.Pseudo)
	strTable := serializeStringTable(p.StringTable)
	// Magic(4) + Version(2) + CodeLen(4) + Code + PseudoLen(4) + Pseudo + StrTabLen(4) + StrTab
	// + MethodBlockStart(4) + Exports（v3 起）
	total := 4 + 2 + 4 + codeLen + 4 + pseudoLen + 4 + len(strTable) + 4 + 4
	for _, e := range p.Exports {
		total += 4 + len(e.Name) + 4 + 1 + 1
	}
	out := make([]byte, 0, total)
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], Magic)
	out = append(out, buf[:]...)
	binary.LittleEndian.PutUint16(buf[:2], BytecodeVersion)
	out = append(out, buf[:2]...)
	binary.LittleEndian.PutUint32(buf[:], uint32(codeLen))
	out = append(out, buf[:]...)
	out = append(out, p.Code...)
	binary.LittleEndian.PutUint32(buf[:], uint32(pseudoLen))
	out = append(out, buf[:]...)
	out = append(out, p.Pseudo...)
	binary.LittleEndian.PutUint32(buf[:], uint32(len(strTable)))
	out = append(out, buf[:]...)
	out = append(out, strTable...)
	binary.LittleEndian.PutUint32(buf[:], p.MethodBlockStart)
	out = append(out, buf[:]...)
	binary.LittleEndian.PutUint32(buf[:], uint32(len(p.Exports)))
	out = append(out, buf[:]...)
	for _, e := range p.Exports {
		b := []byte(e.Name)
		binary.LittleEndian.PutUint32(buf[:], uint32(len(b)))
		out = append(out, buf[:]...)
		out = append(out, b...)
		binary.LittleEndian.PutUint32(buf[:], uint32(e.CodeOffset))
		out = append(out, buf[:]...)
		out = append(out, e.NumLocals, e.NumParams)
	}
	return out
}

// Deserialize 从字节反序列化为 Program
func Deserialize(data []byte) (*Program, error) {
	if len(data) < 10 {
		return nil, errCorrupted
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != Magic {
		return nil, errCorrupted
	}
	ver := binary.LittleEndian.Uint16(data[4:6])
	codeLen := binary.LittleEndian.Uint32(data[6:10])
	off := 10
	if uint32(len(data)-off) < codeLen+8 {
		return nil, errTruncated
	}
	code := make([]byte, codeLen)
	copy(code, data[off:off+int(codeLen)])
	off += int(codeLen)
	pseudoLen := binary.LittleEndian.Uint32(data[off : off+4])
	off += 4
	pseudo := make([]byte, pseudoLen)
	copy(pseudo, data[off:off+int(pseudoLen)])
	off += int(pseudoLen)
	var strTab []string
	if off+4 <= len(data) {
		stLen := binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
		if uint32(len(data)-off) >= stLen {
			strTab = deserializeStringTable(data[off : off+int(stLen)])
		}
		off += int(stLen) // 跳过字符串表数据区（否则后续段错位）
	}
	prog := &Program{
		Valid:       true,
		Version:     ver,
		Code:        code,
		Pseudo:      pseudo,
		StringTable: strTab,
	}
	// v3 尾部：MethodBlockStart + Exports（可选，旧版本文件缺失时保持零值）
	if off+4 <= len(data) {
		prog.MethodBlockStart = binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
	}
	if off+4 <= len(data) {
		nExp := int(binary.LittleEndian.Uint32(data[off : off+4]))
		off += 4
		for i := 0; i < nExp; i++ {
			if off+4 > len(data) {
				break
			}
			nLen := int(binary.LittleEndian.Uint32(data[off : off+4]))
			off += 4
			if off+nLen+6 > len(data) {
				break
			}
			e := Export{
				Name:       string(data[off : off+nLen]),
				CodeOffset: int32(binary.LittleEndian.Uint32(data[off+nLen : off+nLen+4])),
				NumLocals:  data[off+nLen+4],
				NumParams:  data[off+nLen+5],
			}
			off += nLen + 6
			prog.Exports = append(prog.Exports, e)
		}
	}
	return prog, nil
}

type errorString string

func (e errorString) Error() string { return string(e) }

var (
	errCorrupted errorString = "mlr: corrupted file (bad magic)"
	errTruncated errorString = "mlr: file truncated"
)

// serializeStringTable: [count][len1][str1][len2][str2]...
func serializeStringTable(tab []string) []byte {
	var out []byte
	var buf [4]byte
	binary.LittleEndian.PutUint32(buf[:], uint32(len(tab)))
	out = append(out, buf[:]...)
	for _, s := range tab {
		b := []byte(s)
		binary.LittleEndian.PutUint32(buf[:], uint32(len(b)))
		out = append(out, buf[:]...)
		out = append(out, b...)
	}
	return out
}

func deserializeStringTable(data []byte) []string {
	if len(data) < 4 {
		return nil
	}
	count := binary.LittleEndian.Uint32(data[0:4])
	off := 4
	tab := make([]string, 0, count)
	for i := uint32(0); i < count; i++ {
		if off+4 > len(data) {
			break
		}
		sLen := binary.LittleEndian.Uint32(data[off : off+4])
		off += 4
		if uint32(len(data)-off) < sLen {
			break
		}
		tab = append(tab, string(data[off:off+int(sLen)]))
		off += int(sLen)
	}
	return tab
}

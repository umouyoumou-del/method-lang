// Package ast —— Method 语言抽象语法树（AST）节点定义。
package ast

// NodeKind —— AST 节点种类
type NodeKind int

const (
	NProgram NodeKind = iota
	NImport
	NClass
	NMethod
	NField
	NParam
	NBlock
	NList
	NIf
	NWhile
	NForIn
	NForC
	NTry
	NReturn
	NBreak
	NContinue
	NPass
	NRaise
	NAssert
	NWith
	NGlobal
	NNonlocal
	NExprStmt
	NAssign
	NAugAssign
	NLambda
	NYield
	NUnary
	NCompare
	NCall
	NIndex
	NMember
	NName
	NInt
	NFloat
	NString
	NBool
	NNull
	NThis
	NListExpr
	NNew
	NInstanceOf
	NSuper
	NInterface
	NBinary // 二元运算
	NGo     // go 语句：children[0]=方法名, children[1]=参数列表
	NFinally
)

// NodeKindName 返回节点种类的可读名称
func NodeKindName(k NodeKind) string {
	names := [...]string{
		"Program", "Import", "Class", "Method", "Field", "Param",
		"Block", "List", "If", "While", "ForIn", "ForC", "Try",
		"Return", "Break", "Continue", "Pass", "Raise", "Assert",
		"With", "Global", "Nonlocal", "ExprStmt", "Assign", "AugAssign",
		"Lambda", "Yield", "Unary", "Compare", "Call",
		"Index", "Member", "Name", "Int", "Float", "String", "Bool",
		"Null", "This", "ListExpr", "New", "InstanceOf", "Super", "Interface",
		"Binary", "Go", "Finally",
	}
	if int(k) >= 0 && int(k) < len(names) {
		return names[k]
	}
	return "?"
}

// Node —— AST 节点
type Node struct {
	Kind     NodeKind
	Text     string  // 名字 / 运算符 / 字符串值
	IVal     int64   // N_INT / N_BOOL
	FVal     float64 // N_FLOAT
	Line     int
	IsPublic int // 修饰符：低2位access(0=pkg,1=pub,2=priv,3=prot), bit2=static, bit3=abstract, bit4=async
	Children []*Node
}

// New 创建一个节点
func New(kind NodeKind, text string, line int) *Node {
	return &Node{Kind: kind, Text: text, Line: line, Children: nil}
}

func NewInt(v int64, line int) *Node {
	n := New(NInt, "", line)
	n.IVal = v
	return n
}

func NewFloat(v float64, line int) *Node {
	n := New(NFloat, "", line)
	n.FVal = v
	return n
}

func NewBool(v bool, line int) *Node {
	n := New(NBool, "", line)
	if v {
		n.IVal = 1
	}
	return n
}

// Add 添加子节点，nil 子节点被忽略
func (n *Node) Add(child *Node) {
	if child == nil {
		return
	}
	n.Children = append(n.Children, child)
}

// MoveChildren 把 src 的子节点搬过来并清空 src
func (n *Node) MoveChildren(src *Node) {
	if src == nil {
		return
	}
	n.Children = append(n.Children, src.Children...)
	src.Children = nil
}

// StringUnescape 解码字符串字面量：去除首尾引号，处理转义
func StringUnescape(raw string) string {
	if len(raw) < 2 {
		return ""
	}
	body := raw[1 : len(raw)-1]
	out := make([]byte, 0, len(body))
	for i := 0; i < len(body); i++ {
		if body[i] == '\\' && i+1 < len(body) {
			i++
			c := body[i]
			switch c {
			case 'n':
				out = append(out, '\n')
			case 't':
				out = append(out, '\t')
			case 'r':
				out = append(out, '\r')
			case '\\':
				out = append(out, '\\')
			case '"':
				out = append(out, '"')
			case '\'':
				out = append(out, '\'')
			case '0':
				out = append(out, 0)
			case 'f':
				out = append(out, '\f')
			case 'v':
				out = append(out, '\v')
			default:
				out = append(out, '\\', c)
			}
		} else {
			out = append(out, body[i])
		}
	}
	return string(out)
}

// Print 打印 AST 到 stdout，缩进形式
func (n *Node) Print(depth int) {
	if n == nil {
		printIndent(depth)
		println("(null)")
		return
	}
	printIndent(depth)
	print(NodeKindName(n.Kind))
	if n.Text != "" {
		print(" '", n.Text, "'")
	}
	switch n.Kind {
	case NInt:
		print(" = ", n.IVal)
	case NFloat:
		print(" = ", n.FVal)
	case NBool:
		if n.IVal != 0 {
			print(" = true")
		} else {
			print(" = false")
		}
	}
	if n.IsPublic&1 != 0 {
		print(" [public]")
	}
	if n.IsPublic&(1<<2) != 0 {
		print(" [static]")
	}
	if n.IsPublic&(1<<3) != 0 {
		print(" [abstract]")
	}
	if n.IsPublic&(1<<4) != 0 {
		print(" [async]")
	}
	println()
	for _, c := range n.Children {
		c.Print(depth + 1)
	}
}

func printIndent(d int) {
	for i := 0; i < d*2; i++ {
		print(" ")
	}
}

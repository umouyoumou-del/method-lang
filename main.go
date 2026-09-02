// main —— methodc.exe：Method 语言一站式工具
//
// 用法：
//
//	methodc <file.mt>                  # 编译+解释执行
//	methodc <file.mlr>                 # 加载 .mlr 字节码并执行
//	methodc <file.mt> --compile -o X.mlr   # 编译为 .mlr
//	methodc <file.mt> --ast            # 仅打印 AST
//	methodc -h / --help                # 帮助
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"method/ast"
	"method/bytecode"
	"method/compiler"
	"method/lexer"
	"method/parser"
	"method/vm"
)

func main() {
	if len(os.Args) < 2 {
		printBanner()
		printHelp()
		os.Exit(1)
	}

	// 自定义 flag 解析（为兼容 .NET 风格 /opt:val）
	args := os.Args[1:]
	var inputFile string
	var outputFile string
	mode := "interpret" // interpret | compile | ast
	var showHelp bool

	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") || strings.HasPrefix(a, "/") {
			opt := strings.TrimLeft(a, "-/")
			optLower := strings.ToLower(opt)
			switch {
			case optLower == "h" || optLower == "help" || optLower == "?":
				showHelp = true
			case optLower == "c" || optLower == "compile":
				mode = "compile"
			case optLower == "ast":
				mode = "ast"
			case optLower == "interpret":
				mode = "interpret"
			case optLower == "o" || optLower == "out":
				// 下一个参数
				if i+1 < len(args) {
					i++
					outputFile = args[i]
				}
			case strings.HasPrefix(optLower, "out:"):
				outputFile = opt[4:] // /out:xxx
			case strings.HasPrefix(optLower, "o:"):
				outputFile = opt[2:]
			case strings.HasPrefix(optLower, "t:"):
				// .NET 风格目标：/t:exe /t:library 等——简化：仅支持 /t:mlr(=compile)
				tv := strings.ToLower(opt[2:])
				if tv == "mlr" {
					mode = "compile"
				}
			default:
				fmt.Fprintf(os.Stderr, "unknown option: %s\n", a)
				os.Exit(2)
			}
		} else {
			inputFile = a
		}
	}

	if showHelp {
		printBanner()
		printHelp()
		return
	}
	if inputFile == "" {
		fmt.Fprintln(os.Stderr, "error: no input file")
		printHelp()
		os.Exit(2)
	}

	printBanner()

	ext := strings.ToLower(filepath.Ext(inputFile))

	switch mode {
	case "ast":
		runAST(inputFile)
	case "compile":
		runCompile(inputFile, outputFile)
	case "interpret":
		fallthrough
	default:
		if ext == ".mlr" {
			runMLRFile(inputFile)
		} else {
			runMTFile(inputFile)
		}
	}
}

func printBanner() {
	fmt.Fprintln(os.Stderr, "methodc — Method 语言编译器 & 虚拟机")
	fmt.Fprintln(os.Stderr, "  语言名: method  |  文件后缀: .mt  |  字节码: .mlr")
	fmt.Fprintln(os.Stderr, "----------------------------------------")
}

func printHelp() {
	fmt.Println("用法：")
	fmt.Println("  methodc <file.mt>                    编译 .mt 源码并解释执行")
	fmt.Println("  methodc <file.mlr>                   加载 .mlr 字节码并解释执行")
	fmt.Println("  methodc <file.mt> --compile -o X.mlr 编译为 .mlr 字节码文件")
	fmt.Println("  methodc <file.mt> --ast              解析 .mt 并打印 AST")
	fmt.Println("  methodc -h / --help                  显示本帮助")
	fmt.Println()
	fmt.Println("选项别名（.NET 风格）：")
	fmt.Println("  /t:mlr         → 等价 --compile")
	fmt.Println("  /out:<file>    → 等价 -o <file>")
}

// ========== 子命令实现 ==========

// readSrc 读取 .mt 源文件
func readSrc(path string) (string, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return "", err
	}
	return string(data), nil
}

// parseSrc 词法+语法 → AST
func parseSrc(src string, filename string) (*ast.Node, []string, error) {
	l := lexer.New(src)
	p := parser.New(l)
	tree := p.ParseProgram()
	if len(p.Errors()) > 0 {
		return nil, p.Errors(), fmt.Errorf("parse failed: %d errors", len(p.Errors()))
	}
	return tree, nil, nil
}

// runAST: --ast 模式
func runAST(path string) {
	src, err := readSrc(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(3)
	}
	tree, errs, err := parseSrc(src, path)
	if err != nil {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(4)
	}
	tree.Print(0)
}

// runCompile: --compile 模式
func runCompile(input, output string) {
	src, err := readSrc(input)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(3)
	}
	tree, errs, err := parseSrc(src, input)
	if err != nil {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(4)
	}
	comp := compiler.NewCompiler()
	prog := comp.Compile(tree)
	if len(comp.Errors()) > 0 {
		for _, e := range comp.Errors() {
			fmt.Fprintln(os.Stderr, "compiler error:", e)
		}
		os.Exit(5)
	}
	for _, w := range comp.Warnings() {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	// 确定输出文件名
	if output == "" {
		base := strings.TrimSuffix(input, filepath.Ext(input))
		output = base + ".mlr"
	}
	data := prog.Serialize()
	if err := os.WriteFile(output, data, 0644); err != nil {
		fmt.Fprintf(os.Stderr, "write %s error: %v\n", output, err)
		os.Exit(6)
	}
	fmt.Fprintf(os.Stderr, "compile OK → %s (%d bytes code)\n",
		output, len(prog.Code))
}

// runMTFile: 解释 .mt
func runMTFile(path string) {
	src, err := readSrc(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(3)
	}
	tree, errs, err := parseSrc(src, path)
	if err != nil {
		for _, e := range errs {
			fmt.Fprintln(os.Stderr, e)
		}
		os.Exit(4)
	}
	comp := compiler.NewCompiler()
	prog := comp.Compile(tree)
	if len(comp.Errors()) > 0 {
		for _, e := range comp.Errors() {
			fmt.Fprintln(os.Stderr, "compiler error:", e)
		}
		os.Exit(5)
	}
	for _, w := range comp.Warnings() {
		fmt.Fprintln(os.Stderr, "warning:", w)
	}
	runProgram(prog)
}

// runMLRFile: 加载并运行 .mlr
func runMLRFile(path string) {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintf(os.Stderr, "read error: %v\n", err)
		os.Exit(3)
	}
	prog, err := bytecode.Deserialize(data)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load .mlr error: %v\n", err)
		os.Exit(7)
	}
	runProgram(prog)
}

func runProgram(prog *bytecode.Program) {
	interp := vm.NewInterpreter()
	st, err := interp.Run(prog)
	if st != vm.StatusOk {
		fmt.Fprintf(os.Stderr, "\nvm error: status=%s", st)
		if err != nil {
			fmt.Fprintf(os.Stderr, " — %v", err)
		}
		fmt.Fprintln(os.Stderr)
		os.Exit(10)
	}
}

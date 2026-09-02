// delete-mail —— delete.mt 的独立可执行包装（go:embed 内嵌 .mlr 字节码）
//
// 重新生成流程（修改 delete.mt 后）：
//   1. methodc lang\delete-mail\delete.mt --compile -o lang\delete-mail\delete.mlr
//   2. go build -ldflags "-s -w" -o lang\delete-mail\delete-mail.exe ./lang/delete-mail
//
// 运行：config.json 放在 exe 同目录，或保持 lang/delete-mail 相对结构
package main

import (
	_ "embed"
	"fmt"
	"os"

	"method/bytecode"
	"method/vm"
)

//go:embed delete.mlr
var mlrData []byte

func main() {
	prog, err := bytecode.Deserialize(mlrData)
	if err != nil {
		fmt.Fprintf(os.Stderr, "load embedded bytecode error: %v\n", err)
		os.Exit(7)
	}
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

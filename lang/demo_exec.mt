// demo_exec.mt —— Method 语言 system.exec 调用演示（跨平台）

// 打印当前 OS 信息（通过 Go runtime 内置检测不可得，用 system.exec 调命令）
class System {
    static method println(n) {
        system.print(n);
        system.print_char(10);
        return 0;
    }
    static method exec(cmd) {
        return system.exec(cmd);
    }
}

// Windows 上运行 ver；Linux/macOS 上运行 uname -a
System.println(System.exec("ver"));

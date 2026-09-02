// 高并发特性验证：go 线程 + 缓冲通道（Go 风格）
//
// 期望输出（并发交错模式每次不同，但以下内容全部出现）：
//   -999                  (main: start 占位)
//   1 done (sum=10)       (worker1)
//   2 done (sum=55)       (worker2)
//   main: got 42          (channel 同步传递)
//   -888                  (main: end 占位)
// 主程序退出前等待全部子线程结束（WaitGroup）。

// 生产者：通过通道发送值（通道容量 1，发送后子线程结束）
method producer(ch) {
    chan_put(ch, 42);
    return 0;
}

// 独立计算线程：求 1..n 的和（重计算，给调度留出交错空间）
method counter(id, n) {
    var sum = 0;
    var i = 0;
    while (i < n) {
        i = i + 1;
        sum = sum + i;
    }
    system.print(id);
    system.print_str(" done (sum=");
    system.print(sum);
    system.print_str(")");
    system.println(0);
    return 0;
}

ch = chan_new(1);
go counter(1, 4);      // worker1: sum=10
go counter(2, 10);     // worker2: sum=55
system.println(-999);  // main: start
go producer(ch);
v = chan_get(ch);      // 阻塞等待子线程发送 42
system.print_str("main: got ");
system.println(v);
system.println(-888);  // main: end

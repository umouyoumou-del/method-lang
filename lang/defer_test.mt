// ==================== defer 延迟调用测试 ====================
// Go 语义：参数立即求值；函数体延迟到所属帧返回/异常展开时按 LIFO 执行。
// 支持：defer methodName(args) 与 defer lambda(params){...}(args)

method expect_i(actual, expected, name) {
    if (actual == expected) {
        system.print_str("  [PASS] ");
        system.print_str(name);
        system.print_char(10);
        return 1;
    }
    system.print_str("  [FAIL] ");
    system.print_str(name);
    system.print_str("  expected=");
    system.print(expected);
    system.print_str(" actual=");
    system.print(actual);
    system.print_char(10);
    return 0;
}

// 用类静态字段保存当前待检查的顺序表（方法体内可通过 Sink.cur 读取全局态）
class Sink {
    static var cur;
}

var passed = 0;

// rec 追加 v 到当前顺序表
method rec(v) {
    list.push(Sink.cur, v);
    return 0;
}

// ---- 1. LIFO：3 个 defer 逆序执行 ----
system.print_str("[1] defer LIFO\n");
Sink.cur = list.new();
method t1() {
    defer rec(1);
    defer rec(2);
    defer rec(3);
    return 0;
}
var r1 = t1();
passed = passed + expect_i(r1, 0, "t1 返回正常");
passed = passed + expect_i(list.get(Sink.cur, 0), 3, "order1[0]=3");
passed = passed + expect_i(list.get(Sink.cur, 1), 2, "order1[1]=2");
passed = passed + expect_i(list.get(Sink.cur, 2), 1, "order1[2]=1");

// ---- 2. return 后执行：先 body 后 deferred ----
system.print_str("[2] body 先于 defer\n");
Sink.cur = list.new();
method t2() {
    rec(10);            // body 副作用
    defer rec(20);      // 延迟执行
    rec(11);            // 更多 body
    return 0;
}
var r2 = t2();
passed = passed + expect_i(list.get(Sink.cur, 0), 10, "order2[0]=10 (body)");
passed = passed + expect_i(list.get(Sink.cur, 1), 11, "order2[1]=11 (body)");
passed = passed + expect_i(list.get(Sink.cur, 2), 20, "order2[2]=20 (deferred)");

// ---- 3. 异常路径：方法内 raise，defer 仍执行 ----
system.print_str("[3] 异常时 defer 执行\n");
Sink.cur = list.new();
method t3() {
    defer rec(33);
    raise "t3 boom";
}
method t3call() {
    try {
        t3();
    } except {
        return 0;
    }
    return 1;
}
var r3 = t3call();
passed = passed + expect_i(r3, 0, "t3 异常被捕获");
passed = passed + expect_i(list.get(Sink.cur, 0), 33, "order3[0]=33 (defer in exception)");

// ---- 4. 参数立即求值：循环里 defer 快照 i ----
system.print_str("[4] 参数立即求值\n");
Sink.cur = list.new();
method t4() {
    var i = 0;
    while (i < 3) {
        defer rec(i); // i 立即求值：0,1,2
        i = i + 1;
    }
    return 0;
}
var r4 = t4();
passed = passed + expect_i(r4, 0, "t4 返回正常");
passed = passed + expect_i(list.get(Sink.cur, 0), 2, "order4[0]=2");
passed = passed + expect_i(list.get(Sink.cur, 1), 1, "order4[1]=1");
passed = passed + expect_i(list.get(Sink.cur, 2), 0, "order4[2]=0");

// ---- 5. defer 在 try-finally 块内（时机差异）----
system.print_str("[5] defer 在 try-finally\n");
Sink.cur = list.new();
method t5() {
    try {
        defer rec(51);
        defer rec(52);
        raise "t5 boom";
    } except {
        // 在本方法内吞掉异常：defer 仍挂起
    } finally {
        rec(59);
    }
    return 0;
}
var r5 = t5();
passed = passed + expect_i(r5, 0, "t5 返回正常");
passed = passed + expect_i(list.get(Sink.cur, 0), 59, "order5[0]=59 (finally)");
passed = passed + expect_i(list.get(Sink.cur, 1), 52, "order5[1]=52 (defer LIFO)");
passed = passed + expect_i(list.get(Sink.cur, 2), 51, "order5[2]=51 (defer LIFO)");

// ---- 6. defer 与 GC：defer 记录中的闭包在帧存活期间不被回收 ----
system.print_str("[6] defer 与 GC\n");
Sink.cur = list.new();
method t6() {
    var i = 0;
    while (i < 5) {
        defer lambda(v) {
            rec(v); // v 作为参数快照
        }(i);
        i = i + 1;
    }
    var freed = system.gc(); // defer 记录持 closure_id，不可被回收
    return 0;
}
var r6 = t6();
passed = passed + expect_i(r6, 0, "t6 返回正常");
passed = passed + expect_i(list.get(Sink.cur, 0), 4, "order6[0]=4");
passed = passed + expect_i(list.get(Sink.cur, 1), 3, "order6[1]=3");
passed = passed + expect_i(list.get(Sink.cur, 2), 2, "order6[2]=2");
passed = passed + expect_i(list.get(Sink.cur, 3), 1, "order6[3]=1");
passed = passed + expect_i(list.get(Sink.cur, 4), 0, "order6[4]=0");

// ---- 7. defer 链异常：defer 内 raise 向外传播被捕获 ----
system.print_str("[7] defer 内 raise 向外传播\n");
Sink.cur = list.new();
method t7() {
    defer lambda() {
        rec(71);
        raise "defer boom";
    }();
    return 0;
}
method t7call() {
    try {
        t7();
    } except {
        return 0;
    }
    return 1;
}
var r7 = t7call();
passed = passed + expect_i(r7, 0, "defer 内 raise 被外层捕获");
passed = passed + expect_i(list.get(Sink.cur, 0), 71, "order7[0]=71 (defer body ran)");

system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/25\n");
if (passed == 25) {
    system.print_str("  ALL DEFER TESTS PASSED\n");
    return 0;
}
system.print_str("  SOME DEFER TESTS FAILED\n");
return 1;

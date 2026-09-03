// ==================== GC 垃圾回收测试 ====================
// 测试手动 GC + 自动触发 + 对象/列表/字典回收

class Node {
    var next;
    method init() { this.next = 0; }
}

class Container {
    var data;
    method init() { this.data = list.new(); }
    method add(v) { list.push(this.data, v); }
}

// 返回 1=通过 0=失败（int 比较）
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

var passed = 0;

// ---- 1. 手动 GC 基本功能 ----
system.print_str("[1] 手动 GC 基本功能\n");
var i = 0;
while (i < 50) {
    var tmp = new Node();
    i = i + 1;
}
var freed = system.gc();
system.print_str("  freed after 50 temp objects: ");
system.print(freed);
system.print_char(10);
// 49 freed (last tmp still in local), 50th is reachable
passed = passed + expect_i(freed >= 49, 1, "GC 回收了临时对象 (>=49)");

// ---- 2. 保留引用的对象不会被回收 ----
system.print_str("[2] 保留引用的对象不会被回收\n");
var keep = new Node();
freed = system.gc();
system.print_str("  freed when 1 live object: ");
system.print(freed);
system.print_char(10);
passed = passed + expect_i(keep.next, 0, "存活对象可访问");

// ---- 3. 循环引用 ----
system.print_str("[3] 循环引用\n");
var a = new Node();
var b = new Node();
a.next = b;
b.next = a;
a = 0;
b = 0;
freed = system.gc();
system.print_str("  freed after cycle: ");
system.print(freed);
system.print_char(10);
passed = passed + expect_i(freed >= 2, 1, "GC 回收了循环引用对象");

// ---- 4. 列表回收 ----
system.print_str("[4] 列表回收\n");
i = 0;
while (i < 30) {
    var tmp_list = list.new();
    list.push(tmp_list, 0);
    i = i + 1;
}
freed = system.gc();
system.print_str("  freed after 30 temp lists: ");
system.print(freed);
system.print_char(10);
passed = passed + expect_i(freed >= 25, 1, "GC 回收了临时列表 (>=25)");

// ---- 5. 字典回收 ----
system.print_str("[5] 字典回收\n");
i = 0;
while (i < 20) {
    var tmp_dict = dict.new();
    i = i + 1;
}
freed = system.gc();
system.print_str("  freed after 20 temp dicts: ");
system.print(freed);
system.print_char(10);
passed = passed + expect_i(freed >= 18, 1, "GC 回收了临时字典 (>=18)");

// ---- 6. 容器内对象的可达性 ----
system.print_str("[6] 容器内对象可达性\n");
var box = new Container();
box.add(new Node());
box.add(new Node());
freed = system.gc();
system.print_str("  freed with container holding 2 nodes: ");
system.print(freed);
system.print_char(10);
var v0 = list.get(box.data, 0);
passed = passed + expect_i(v0 > 0, 1, "容器内对象 ID 仍然有效");

// ---- 7. 自动 GC 触发 ----
system.print_str("[7] 自动 GC 触发\n");
system.print_str("  分配 500 个对象（自动触发 GC）...\n");
i = 0;
while (i < 500) {
    var tmp = new Node();
    i = i + 1;
}
system.print_str("  完成，无崩溃\n");
passed = passed + expect_i(1, 1, "大量分配后 VM 稳定运行");

// ---- 8. GC 不影响存活对象 ----
system.print_str("[8] GC 不影响存活对象\n");
var longlived = new Container();
i = 0;
while (i < 100) {
    longlived.add(new Node());
    i = i + 1;
}
system.gc();
var ok = 1;
i = 0;
while (i < 100) {
    var oid = list.get(longlived.data, i);
    if (oid <= 0) { ok = 0; }
    i = i + 1;
}
passed = passed + expect_i(ok, 1, "GC 后 100 个容器内对象全部可达");

// ---- 9. GC 后继续分配正常 ----
system.print_str("[9] GC 后继续分配正常\n");
freed = system.gc();
var after = new Node();
system.gc();
passed = passed + expect_i(after.next, 0, "GC 后新对象正常存活");

// ---- 汇总 ----
system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/9\n");
if (passed == 9) {
    system.print_str("  ALL GC TESTS PASSED\n");
}
if (passed != 9) {
    system.print_str("  SOME GC TESTS FAILED\n");
    return 1;
}
return 0;

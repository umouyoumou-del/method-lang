// ==================== 闭包功能测试 ====================
// lambda block 形式：lambda(params) { ... } 作为闭包表达式
// 捕获语义：按值快照外层 local 的当前值（与 &x ref-cell 模型一致）

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

// ---- 1. 命名闭包与立即调用 ----
system.print_str("[1] 命名闭包与立即调用\n");
var add = lambda(a, b) {
    return a + b;
};
passed = passed + expect_i(add(3, 4), 7, "add(3,4)=7");
passed = passed + expect_i(lambda(x) { return x * x; }(5), 25, "lambda(x){x*x;}(5)=25");

// ---- 2. 无参闭包 ----
system.print_str("[2] 无参闭包\n");
var greet = lambda {
    return 42;
};
passed = passed + expect_i(greet(), 42, "greet()=42");

// ---- 3. 捕获外层 local（按值快照） ----
system.print_str("[3] 捕获外层 local\n");
method make_adder(n) {
    return lambda(x) {
        return x + n;
    };
}
var add10 = make_adder(10);
var add20 = make_adder(20);
passed = passed + expect_i(add10(5), 15, "add10(5)=15 (capture n=10)");
passed = passed + expect_i(add20(5), 25, "add20(5)=25 (capture n=20)");

// ---- 4. 捕获后外层变量不变（值快照语义） ----
system.print_str("[4] 值快照语义\n");
var base = 100;
var f = lambda(x) { return x + base; };
passed = passed + expect_i(f(1), 101, "f(1)=101 (base=100)");
base = 999;
passed = passed + expect_i(f(1), 101, "f(1)=101 仍为旧值（值快照）");

// ---- 5. 闭包作为方法参数 ----
system.print_str("[5] 闭包作为方法参数\n");
method apply(f, v) {
    return f(v);
}
passed = passed + expect_i(apply(lambda(x) { return x * 3; }, 7), 21, "apply(f,7)=21");

// ---- 6. 闭包修改自身 capture（StoreCapture） ----
system.print_str("[6] 闭包修改 capture\n");
method make_counter() {
    var c = 0;
    var inc = lambda {
        c = c + 1;
        return c;
    };
    return inc;
}
var cnt = make_counter();
cnt();
cnt();
cnt();
passed = passed + expect_i(cnt(), 4, "第 4 次调用 cnt()=4");

// ---- 7. 嵌套闭包（捕获外层闭包的 capture） ----
system.print_str("[7] 嵌套闭包\n");
method outer() {
    var a = 1;
    var mid = lambda {
        var b = 2;
        var inner = lambda {
            return a + b;
        };
        return inner;
    };
    return mid;
}
// outer() → mid, mid() → inner, inner() → 3
var mid_fn = outer();
var inner_fn = mid_fn();
passed = passed + expect_i(inner_fn(), 3, "inner_fn()=1+2=3");

// ---- 8. 闭包捕获 obj_id 访问字段 ----
system.print_str("[8] 闭包捕获对象引用\n");
class Box {
    var v;
    method get() {
        return this.v;
    }
}
var box = new Box();
box.v = 99;
var getbox = lambda {
    return box.get();
};
passed = passed + expect_i(getbox(), 99, "getbox()=99 via captured obj_id");

// ---- 9. 闭包作为 list 元素 ----
system.print_str("[9] 闭包作为 list 元素\n");
var fs = list.new();
list.push(fs, lambda(x) { return x + 1; });
list.push(fs, lambda(x) { return x + 2; });
var f0 = list.get(fs, 0);
var f1 = list.get(fs, 1);
passed = passed + expect_i(f0(10), 11, "fs[0](10)=11");
passed = passed + expect_i(f1(10), 12, "fs[1](10)=12");

// ---- 10. 闭包与 GC（大量分配不崩溃） ----
system.print_str("[10] 闭包与 GC\n");
var i = 0;
while (i < 100) {
    var tmp = lambda(x) { return x + i; };
    i = i + 1;
}
var freed = gc();
system.print_str("  freed after 100 temp closures: ");
system.print(freed);
system.print_char(10);
var gc_ok = 1;
if (freed < 0) {
    gc_ok = 0;
}
passed = passed + expect_i(gc_ok, 1, "GC 运行无崩溃");

// ---- 11. 旧单表达式 lambda 仍可用（系统调用占位） ----
system.print_str("[11] 旧语法兼容\n");
// 旧 lambda 单表达式形式不作为值使用，仅作占位
// 这里仅验证 block 形式正常工作
passed = passed + expect_i(lambda() { return 7; }(), 7, "lambda(){return 7;}()=7");

// ---- 12. 闭包内调用其他闭包 ----
system.print_str("[12] 闭包链式调用\n");
var doubler = lambda(x) { return x * 2; };
var quad = lambda(x) {
    return doubler(doubler(x));
};
passed = passed + expect_i(quad(5), 20, "quad(5)=20");

system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/16\n");
if (passed == 16) {
    system.print_str("  ALL CLOSURE TESTS PASSED\n");
}

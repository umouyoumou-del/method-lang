// ==================== slice / range 测试 ====================
// Go 风格迭代：
//   for v in range iter
//   for k, v in range iter
// list / slice（list 别名）/ dict / str 四种容器。
// slice API 别名：slice.new= list.new, slice.append=list.push, slice.len/cap=list.len, ...

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

// ---- 1. 单值迭代 list ----
system.print_str("[1] for v in range list\n");
var l1 = list.new();
list.push(l1, 10);
list.push(l1, 20);
list.push(l1, 30);
var sum1 = 0;
var cnt1 = 0;
for v in range l1 {
    sum1 = sum1 + v;
    cnt1 = cnt1 + 1;
}
passed = passed + expect_i(sum1, 60, "l1 值总和 60");
passed = passed + expect_i(cnt1, 3, "l1 迭代 3 次");

// ---- 2. 双变量迭代 list：k=index, v=value ----
system.print_str("[2] for k, v in range list\n");
var l2 = list.new();
var i2 = 0;
while (i2 < 5) {
    list.push(l2, i2 * 3);
    i2 = i2 + 1;
}
var ok2 = 1;
var cnt2 = 0;
for k, v in range l2 {
    if (v != k * 3) {
        ok2 = 0;
    }
    cnt2 = cnt2 + 1;
}
passed = passed + expect_i(ok2, 1, "l2 k=index v=value 一致");
passed = passed + expect_i(cnt2, 5, "l2 迭代 5 次");

// ---- 3. 双变量迭代 dict：k=str_idx key, v=value ----
system.print_str("[3] for k, v in range dict\n");
var d3 = dict.new();
dict.put(d3, "a", 1);
dict.put(d3, "b", 2);
dict.put(d3, "c", 3);
var n3 = 0;
var sum3 = 0;
var ok3 = 1;
for k, v in range d3 {
    n3 = n3 + 1;
    sum3 = sum3 + v;
    var is_a = str.equal(k, "a");
    var is_b = str.equal(k, "b");
    var is_c = str.equal(k, "c");
    if (not (is_a or is_b or is_c)) {
        ok3 = 0;
    }
}
passed = passed + expect_i(n3, 3, "d3 迭代 3 键");
passed = passed + expect_i(sum3, 6, "d3 值总和 6");
passed = passed + expect_i(ok3, 1, "d3 全部 key 属于 a/b/c");

// ---- 4. 单值迭代 str：c = rune codepoint ----
system.print_str("[4] for c in range str\n");
var s4 = "Hi你";
var cnt4 = 0;
for c in range s4 {
    cnt4 = cnt4 + 1;
}
passed = passed + expect_i(cnt4, 3, "s4 rune 数=3（多字节安全）");

// ---- 5. 双变量迭代 str：k=rune idx, v=rune ----
system.print_str("[5] for k, v in range str\n");
var s5 = "ABC";
var ok5 = 1;
for k, v in range s5 {
    if (k == 0 and v != 65) { ok5 = 0; }
    if (k == 1 and v != 66) { ok5 = 0; }
    if (k == 2 and v != 67) { ok5 = 0; }
}
passed = passed + expect_i(ok5, 1, "s5 索引与 codepoint 正确");

// ---- 6. 空容器：循环体不执行 ----
system.print_str("[6] 空容器迭代\n");
var l6 = list.new();
var d6 = dict.new();
var s6 = "";
var cnt6 = 0;
for v in range l6 {
    cnt6 = cnt6 + 1;
}
for v in range d6 {
    cnt6 = cnt6 + 1;
}
for c in range s6 {
    cnt6 = cnt6 + 1;
}
passed = passed + expect_i(cnt6, 0, "空 list/dict/str 循环体不执行");

// ---- 7. range + break/continue ----
system.print_str("[7] range + break/continue\n");
var l7 = list.new();
var j7 = 1;
while (j7 <= 5) {
    list.push(l7, j7);
    j7 = j7 + 1;
}
var sum7 = 0;
for v in range l7 {
    if (v == 3) {
        continue;
    }
    if (v == 5) {
        break;
    }
    sum7 = sum7 + v;
}
passed = passed + expect_i(sum7, 7, "跳过3并在5中断，sum=1+2+4=7");

// ---- 8. slice API 别名 ----
system.print_str("[8] slice API 别名\n");
var s = slice.new();
slice.append(s, 5);
slice.append(s, 6);
slice.append(s, 7);
var cnt8 = 0;
var sum8 = 0;
for k, v in range s {
    if (v != k + 5) {
        cnt8 = -1000;
    }
    sum8 = sum8 + v;
    cnt8 = cnt8 + 1;
}
passed = passed + expect_i(cnt8, 3, "slice 迭代 3 次");
passed = passed + expect_i(sum8, 18, "slice 值总和 18");
passed = passed + expect_i(slice.len(s), 3, "slice.len(s)=3");
passed = passed + expect_i(slice.cap(s), 3, "slice.cap(s)=3 (简化同 len)");

// ---- 9. range 内使用 lambda 闭包 ----
system.print_str("[9] range 内用 lambda\n");
var l9 = list.new();
list.push(l9, 1);
list.push(l9, 2);
list.push(l9, 3);
var mul2 = lambda(x) {
    return x * 2;
};
var sum9 = 0;
for v in range l9 {
    sum9 = sum9 + mul2(v);
}
passed = passed + expect_i(sum9, 12, "sum9 = 2+4+6 = 12");

// ---- 10. range 内用多返回值接收 ----
system.print_str("[10] range 内多返回值\n");
method dv(x) (int, int) {
    return x / 2, x % 2;
}
var l10 = list.new();
list.push(l10, 5);
list.push(l10, 7);
list.push(l10, 9);
var sum10 = 0;
for v in range l10 {
    var q, r = dv(v);
    sum10 = sum10 + q + r;
}
passed = passed + expect_i(sum10, 12, "sum10 = Σ(q+r) = 12");

system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/17\n");
if (passed == 17) {
    system.print_str("  ALL RANGE TESTS PASSED\n");
    return 0;
}
system.print_str("  SOME RANGE TESTS FAILED\n");
return 1;

// ==================== 多返回值 + error 模式测试 ====================
// 语法：
//   method f(a, b) (int, int) { return a/b, a%b; }
//   var q, r = f(17, 5);      // NMultiAssign
//   var only = f(10, 3);      // 单接收 → 取第一个返回值
//   f(10, 3);                 // 语句级丢弃
// 闭包同样可返回多值。

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

// ---- 1. 多返回值方法 divmod ----
system.print_str("[1] divmod 双值返回\n");
method divmod(a, b) (int, int) {
    return a / b, a % b;
}
var q, r = divmod(17, 5);
passed = passed + expect_i(q, 3, "q = 17/5 = 3");
passed = passed + expect_i(r, 2, "r = 17%5 = 2");

// ---- 2. error 模式：parse_int 返回 (value, err_str) ----
system.print_str("[2] error 模式 parse_int\n");
method parse_int(s) (int, str) {
    var v = atoi(s);
    if (s == "") {
        return 0, "empty input";
    }
    var back = itoa(v);
    if (back == s) {
        return v, "";
    }
    // 语言不支持 str + str；直接把原输入作为错误信息返回（保证非空错误串）
    return 0, s;
}
var v1, e1 = parse_int("42");
passed = passed + expect_i(v1, 42, "parse_int(42) value");
passed = passed + expect_i(e1 == 0, 1, "parse_int(42) err empty (str idx 0)");
var v2, e2 = parse_int("abc");
passed = passed + expect_i(v2, 0, "parse_int(abc) value=0");
// e2 是非空错误串：str.len(e2) > 0
var errLen = str.len(e2);
passed = passed + expect_i(errLen > 0, 1, "parse_int(abc) err non-empty");

// ---- 3. 多返回值单接收：var only = divmod(10, 3) → only = q ----
system.print_str("[3] 单接收取第一返回值\n");
var only = divmod(10, 3);
passed = passed + expect_i(only, 3, "only = divmod(10,3)[0] = 3");

// ---- 4. 语句级丢弃 ----
system.print_str("[4] 语句级丢弃多返回值\n");
divmod(10, 3);
passed = passed + expect_i(1, 1, "divmod(10,3); 无副作用且无异常");

// ---- 5. 链式接收个数不足：a, b, c = f() 且 f 返 2 → c 补 0 ----
system.print_str("[5] 目标多于返回值补 0\n");
var a, b, c = divmod(11, 4);
passed = passed + expect_i(a, 2, "a = 11/4 = 2");
passed = passed + expect_i(b, 3, "b = 11%4 = 3");
passed = passed + expect_i(c, 0, "c 补 0");

// ---- 6. 闭包返回多值 ----
system.print_str("[6] 闭包多返回值\n");
var f = lambda(x) {
    return x, x * x;
};
var s1, s2 = f(7);
passed = passed + expect_i(s1, 7, "closure s1 = x");
passed = passed + expect_i(s2, 49, "closure s2 = x*x");

// ---- 7. 现有单返回 API 不破坏（http.request/read_file 仍返 list）----
system.print_str("[7] 单返回值兼容\n");
method triple(n) {
    return n * 3;
}
passed = passed + expect_i(triple(5), 15, "triple(5)=15 (单返回)");
// 普通赋值仍正常
var x = triple(2) + 1;
passed = passed + expect_i(x, 7, "x = triple(2)+1 = 7");

system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/15\n");
if (passed == 15) {
    system.print_str("  ALL MULTIRET TESTS PASSED\n");
}
if (passed != 15) {
    system.print_str("  SOME MULTIRET TESTS FAILED\n");
    return 1;
}
return 0;

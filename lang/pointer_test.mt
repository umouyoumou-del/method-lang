// ==================== 指针功能测试 ====================
// &x 取地址（创建引用单元格）、*p 解引用读写

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

// ---- 1. 基本取地址与解引用 ----
system.print_str("[1] 基本取地址与解引用\n");
var x = 42;
var p = &x;
passed = passed + expect_i(*p, 42, "*p 读取 x 的值");

// ---- 2. 通过指针修改变量 ----
system.print_str("[2] 通过指针修改\n");
*p = 99;
passed = passed + expect_i(*p, 99, "*p 修改后读取");
// 注意：&x 复制了 x 的值到指针表，所以 x 本身不变
passed = passed + expect_i(x, 42, "x 原值不变（ref-cell 语义）");

// ---- 3. 指针作为函数参数（跨函数引用）----
system.print_str("[3] 指针作为函数参数\n");

method increment(p) {
    *p = *p + 1;
}

var val = 10;
var ptr = &val;
increment(ptr);
passed = passed + expect_i(*ptr, 11, "跨函数 *p 递增");

method set_to_100(p) {
    *p = 100;
}

set_to_100(ptr);
passed = passed + expect_i(*ptr, 100, "跨函数 *p 赋值");

// ---- 4. 多指针指向不同变量 ----
system.print_str("[4] 多指针\n");
var a = 1;
var b = 2;
var pa = &a;
var pb = &b;
passed = passed + expect_i(*pa, 1, "*pa = 1");
passed = passed + expect_i(*pb, 2, "*pb = 2");
*pa = 100;
*pb = 200;
passed = passed + expect_i(*pa, 100, "*pa 修改后");
passed = passed + expect_i(*pb, 200, "*pb 修改后");

// ---- 5. 指针交换 ----
system.print_str("[5] 指针交换\n");
var sa = 111;
var sb = 222;
var pa2 = &sa;
var pb2 = &sb;
var tmp = *pa2;
*pa2 = *pb2;
*pb2 = tmp;
passed = passed + expect_i(*pa2, 222, "swap 后 *pa2");
passed = passed + expect_i(*pb2, 111, "swap 后 *pb2");

// ---- 6. 指针与对象 ID（指针存储对象引用）----
system.print_str("[6] 指针存储对象引用\n");
class Box {
    var val;
    method init(v) { this.val = v; }
}
var box = new Box(55);
var pbox = &box;  // 复制 box 的对象 ID 到指针表
var retrieved = *pbox;
// retrieved 应该是对象 ID (>0)
passed = passed + expect_i(retrieved > 0, 1, "指针存储的对象 ID 有效");
// 通过指针取到的 ID 访问对象
// （需要通过普通变量传递，因为 *p 取出的是 int64 对象 ID）
box = *pbox;  // 恢复 box 的值
passed = passed + expect_i(box.val, 55, "通过指针恢复的对象字段正确");

// ---- 7. 指针与 GC ----
system.print_str("[7] 指针与 GC\n");
var i = 0;
while (i < 30) {
    var tmp_p = &i;
    i = i + 1;
}
var freed = system.gc();
system.print_str("  freed after 30 temp ptrs: ");
system.print(freed);
system.print_char(10);
passed = passed + expect_i(freed >= 20, 1, "GC 回收了临时指针 (>=20)");

// ---- 8. 指针链 ----
system.print_str("[8] 指针链\n");
var base = 7;
var p1 = &base;    // p1 指向 base 的值 (7)
var p2 = &p1;      // p2 指向 p1 的值 (指针 ID)
// *p2 = p1 的值（指针 ID），*p1 = base 的值
var deref2 = *p2;  // p1 的指针 ID
// 不能直接 **p2（语法不支持嵌套解引用作为单表达式），但可以分步
passed = passed + expect_i(*p1, 7, "*p1 读取 base 的值");

// ---- 汇总 ----
system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("/15\n");
if (passed == 15) {
    system.print_str("  ALL POINTER TESTS PASSED\n");
}
if (passed != 15) {
    system.print_str("  SOME POINTER TESTS FAILED\n");
    return 1;
}
return 0;

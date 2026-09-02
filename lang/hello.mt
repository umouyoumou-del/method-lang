// hello.mt —— Method 语言最小示例
// 用法：methodc.exe lang\hello.mt

system.print_char(72); system.print_char(101); system.print_char(108);
system.print_char(108); system.print_char(111); system.print_char(44);
system.print_char(32); system.print_char(87); system.print_char(111);
system.print_char(114); system.print_char(108); system.print_char(100);
system.print_char(33); system.print_char(10);

// 数学基础：1+2*3=7
a = 1 + 2 * 3;
system.println(a);

// 阶乘：5! = 120
method fact(n) {
    if (n < 2) { return 1; }
    return n * fact(n - 1);
}
system.println(fact(5));

// 斐波那契：fib(10)=55
method fib(n) {
    if (n < 2) { return n; }
    return fib(n-1) + fib(n-2);
}
system.println(fib(10));

// while 循环：求和 1..10 = 55
s = 0;
i = 1;
while (i <= 10) {
    s = s + i;
    i = i + 1;
}
system.println(s);

// if-else
m = 7;
if (m > 5) {
    system.println(1);
} else {
    system.println(0);
}

// 短路 and/or
x = 1 and 0;
y = 0 or 1;
system.println(x);  // 0
system.println(y);  // 1

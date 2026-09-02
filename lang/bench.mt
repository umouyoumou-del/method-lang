// bench.mt —— 微基准：方法调用 + 整数计算

class System {
    static method println(n) {
        system.print(n);
        system.print_char(10);
        return 0;
    }
    static method print(n) {
        system.print(n);
        return 0;
    }
    static method print_char(c) {
        system.print_char(c);
        return 0;
    }
}

// Ackermann(3,6) = 509
method ack(m, n) {
    if (m == 0) { return n + 1; }
    if (n == 0) { return ack(m - 1, 1); }
    return ack(m - 1, ack(m, n - 1));
}

System.print(97); // 'a'
System.print_char(99);
System.print_char(107);
System.print_char(40);
System.print(3);
System.print_char(44);
System.print(6);
System.print_char(41);
System.print_char(32);
System.print_char(61);
System.print_char(32);
System.println(ack(3,6));

// 循环求和 1..10000 = 50005000
s = 0;
i = 1;
N = 10000;
while (i <= N) {
    s = s + i;
    i = i + 1;
}
System.println(s);

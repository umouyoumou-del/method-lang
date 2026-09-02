// system.mt —— Method 语言基础系统操作库（OOP 封装版）
//   System  —— 静态方法容器（数学函数 + IO），通过 System.xxx() 调用
//   String  —— 真正的 OOP 字符串类，封装底层字符串表索引

class System {
    // —— IO ——
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

    // —— 数学 ——
    static method abs(n) {
        if (n < 0) { return -n; }
        return n;
    }
    static method max(a, b) {
        if (a > b) { return a; }
        return b;
    }
    static method min(a, b) {
        if (a < b) { return a; }
        return b;
    }
    static method sign(n) {
        if (n > 0) { return 1; }
        if (n < 0) { return -1; }
        return 0;
    }
    static method fact(n) {
        r = 1;
        i = 1;
        while (i <= n) {
            r = r * i;
            i = i + 1;
        }
        return r;
    }
    static method gcd(a, b) {
        while (b != 0) {
            t = b;
            b = a % b;
            a = t;
        }
        return a;
    }
    // 执行外部命令
    static method exec(cmd) {
        return system.exec(cmd);
    }
}

class String {
    private _s : int = 0;

    // —— 构造函数：从字符串表索引构造 String 对象 ——
    method String(s) {
        this._s = s;
        return 0;
    }

    method length() {
        return str.len(this._s);
    }
    method charAt(i) {
        return str.get_c(this._s, i);
    }
    method isEmpty() {
        n = str.len(this._s);
        if (n == 0) { return 1; }
        return 0;
    }
    method equals(other) {
        la = str.len(this._s);
        lb = str.len(other._s);
        if (la != lb) { return 0; }
        i = 0;
        while (i < la) {
            ca = str.get_c(this._s, i);
            cb = str.get_c(other._s, i);
            if (ca != cb) { return 0; }
            i = i + 1;
        }
        return 1;
    }
    method indexOf(ch) {
        n = str.len(this._s);
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            if (c == ch) { return i; }
            i = i + 1;
        }
        return -1;
    }
    method substring(begin) {
        n = str.len(this._s);
        if (begin < 0) { begin = 0; }
        r = str.new();
        i = begin;
        while (i < n) {
            c = str.get_c(this._s, i);
            str.append_c(r, c);
            i = i + 1;
        }
        return new String(r);
    }
    method concat(other) {
        la = str.len(this._s);
        lb = str.len(other._s);
        r = str.new();
        i = 0;
        while (i < la) {
            c = str.get_c(this._s, i);
            str.append_c(r, c);
            i = i + 1;
        }
        i = 0;
        while (i < lb) {
            c = str.get_c(other._s, i);
            str.append_c(r, c);
            i = i + 1;
        }
        return new String(r);
    }
    method replace(old_ch, new_ch) {
        n = str.len(this._s);
        r = str.new();
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            if (c == old_ch) { c = new_ch; }
            str.append_c(r, c);
            i = i + 1;
        }
        return new String(r);
    }
    method toLowerCase() {
        n = str.len(this._s);
        r = str.new();
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            if (c >= 65) {
                if (c <= 90) { c = c + 32; }
            }
            str.append_c(r, c);
            i = i + 1;
        }
        return new String(r);
    }
    method toUpperCase() {
        n = str.len(this._s);
        r = str.new();
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            if (c >= 97) {
                if (c <= 122) { c = c - 32; }
            }
            str.append_c(r, c);
            i = i + 1;
        }
        return new String(r);
    }
    method print() {
        n = str.len(this._s);
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            system.print_char(c);
            i = i + 1;
        }
        return 0;
    }
    method println() {
        n = str.len(this._s);
        i = 0;
        while (i < n) {
            c = str.get_c(this._s, i);
            system.print_char(c);
            i = i + 1;
        }
        system.print_char(10);
        return 0;
    }
    // 整数转字符串
    static method valueOfInt(n) {
        r = str.new();
        if (n == 0) {
            str.append_c(r, 48);
            return new String(r);
        }
        if (n < 0) {
            str.append_c(r, 45);
            n = -n;
        }
        temp = n;
        digits = 0;
        while (temp > 0) {
            digits = digits + 1;
            temp = temp / 10;
        }
        while (digits > 0) {
            pow = 1;
            k = 1;
            while (k < digits) {
                pow = pow * 10;
                k = k + 1;
            }
            digit = (n / pow) % 10;
            str.append_c(r, digit + 48);
            n = n % pow;
            digits = digits - 1;
        }
        return new String(r);
    }
    static method valueOfChar(c) {
        r = str.new();
        str.append_c(r, c);
        return new String(r);
    }
}

// =====  自检 main：直接运行 system.mt 即可验证各类方法  =====
System.println(System.fact(5));       // 120
System.println(System.gcd(48, 36));   // 12
System.println(System.max(7, 3));     // 7
System.println(System.min(7, 3));     // 3
System.println(System.abs(-42));      // 42
System.println(System.sign(-5));      // -1
System.println(System.sign(0));       // 0

// String 类
s = new String("Hello");
System.println(s.length());           // 5
System.println(s.charAt(0));          // 72 ('H')
System.println(s.charAt(4));          // 111 ('o')
System.println(s.isEmpty());          // 0 (false)
empty = new String("");
System.println(empty.isEmpty());      // 1 (true)
System.println(s.equals(new String("Hello")));  // 1
System.println(s.equals(new String("World")));  // 0
System.println(s.indexOf(108));       // 2 ('l' 第一次出现)
System.println(s.indexOf(122));       // -1 ('z' 不存在)

s.substring(1).println();             // ello
s.concat(new String(" World")).println();   // Hello World
s.replace(108, 76).println();         // 'l'→'L' → HeLLo
s.toUpperCase().println();            // HELLO
s.toLowerCase().println();            // hello
String.valueOfInt(12345).println();   // 12345
String.valueOfInt(-42).println();     // -42
String.valueOfChar(65).println();     // A

// try/except/finally 语义验收
system.print_str("== T1 基本捕获 ==\n");
try {
    system.print_str("  in try\n");
    raise "boom";
    system.print_str("  unreachable\n");
} except as e {
    system.print_str("  caught: ");
    system.print_str(e);
    system.print_char(10);
}

system.print_str("== T2 as 绑定消息 ==\n");
try {
    raise "custom message 42";
} except as e {
    if (str.equal(e, "custom message 42") == 1) {
        system.print_str("  msg OK\n");
    } else {
        system.print_str("  msg WRONG\n");
    }
}

system.print_str("== T3 正常路径 finally ==\n");
var fin_ran = 0;
try {
    system.print_str("  body\n");
} except {
    fin_ran = 99;
} finally {
    fin_ran = 1;
}
if (fin_ran == 1) { system.print_str("  finally OK\n"); }
else { system.print_str("  finally WRONG\n"); }

system.print_str("== T4 finally 异常路径 + 重抛 ==\n");
var fin2 = 0;
var outer_caught = 0;
try {
    try {
        raise "inner";
    } finally {
        fin2 = 1;
    }
} except as e {
    outer_caught = 1;
    if (str.equal(e, "inner") == 1) {
        system.print_str("  rethrow msg OK\n");
    }
}
if (fin2 == 1) { if (outer_caught == 1) { system.print_str("  finally+rethrow OK\n"); } }
else { system.print_str("  finally WRONG\n"); }

system.print_str("== T5 除零可捕获 ==\n");
var z = 0;
try {
    var x = 10 / z;
    system.print_str("  unreachable\n");
} except as e {
    system.print_str("  caught: ");
    system.print_str(e);
    system.print_char(10);
}

system.print_str("== T6 方法内 try + return 清理 ==\n");
safe_div(10, 2);
safe_div(10, 0);
bad_then_good();

system.print_str("== T7 嵌套 try ==\n");
try {
    try {
        raise "deep";
    } except as e {
        raise str.concat("wrapped: ", e);
    }
} except as e2 {
    system.print_str("  outer: ");
    system.print_str(e2);
    system.print_char(10);
}

system.print_str("== T8 try 循环计数 ==\n");
var n = 0;
var i = 0;
var running = 1;
while (running == 1) {
    if (i >= 3) { running = 0; }
    if (running == 1) {
        try {
            if (i == 1) { raise "skip one"; }
            n = n + 1;
        } except {
            system.print_str("  iter ");
            system.print(i);
            system.print_str(" raised\n");
        }
        i = i + 1;
    }
}
if (n == 2) { system.print_str("  loop OK\n"); }
else { system.print_str("  loop WRONG\n"); }

system.print_str("ALL DONE\n");

// ---- 方法 ----
method safe_div(a, b) {
    var r = 0;
    try {
        r = a / b;
        system.print_str("  div=");
        system.print(r);
        system.print_char(10);
    } except as e {
        system.print_str("  div caught: ");
        system.print_str(e);
        system.print_char(10);
    }
    return r;
}

method thrower() {
    raise "from method";
    return 0;
}

method bad_then_good() {
    try {
        thrower();
    } except as e {
        system.print_str("  cross-frame: ");
        system.print_str(e);
        system.print_char(10);
    }
    // 关键：try 结束后后续调用不能被残留 handler 污染
    var ok = 1;
    try {
        ok = 1;
    } except {
        ok = 0;
    }
    if (ok == 1) { system.print_str("  post-try clean\n"); }
}

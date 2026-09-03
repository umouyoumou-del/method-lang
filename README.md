# method-lang

**method** 是一门栈式字节码虚拟机语言，编译器与虚拟机用 Go 实现。

- 语言名：`method`　|　源码后缀：`.mt`　|　字节码：`.mlr`
- 编译流程：`lexer → parser → AST → compiler → 字节码 → VM 解释执行`
- 支持 OOP（class / static method / this / new）、递归、while 循环、内建 list / dict / string / HTTP

## 快速开始

```bash
# 编译并运行 .mt 源码
go run . lang/hello.mt

# 编译为 .mlr 字节码文件
go build -o methodc.exe .
methodc lang/hello.mt --compile -o hello.mlr

# 加载并执行字节码
methodc hello.mlr

# 仅打印 AST
methodc lang/hello.mt --ast
```

选项别名（.NET 风格）：`/t:mlr` ≡ `--compile`，`/out:<file>` ≡ `-o <file>`

## 项目结构

```
method/
├── lexer/         — 词法分析
├── parser/        — 语法分析 → AST
├── ast/           — AST 节点定义
├── compiler/      — AST → 字节码编译器（含内置函数降级）
├── bytecode/      — opcode / 指令编码 / .mlr 序列化
├── vm/            — 栈式字节码虚拟机（含 HTTP、Cookie Jar、容器、字符串表）
├── lang/          — method 语言示例与库
│   ├── hello.mt        — 最小示例（递归 / while / and / or）
│   ├── system.mt       — System / String OOP 封装库
│   ├── bench.mt        — 性能基准
│   ├── concurrent.mt   — 并发演示
│   ├── demo_exec.mt    — system.exec 外部命令演示
│   ├── HtmlParser.mt   — HTML 解析库（DOM 树）
│   └── Wikidot.mt      — Wikidot-Golang 完整重写
├── go.mod
└── main.go        — methodc 入口
```

## 语言特性

### 变量与类型

```
var site : int;          // 显式类型声明（OOP 成员）
x = 1 + 2 * 3;           // 推断赋值（全局/局部）
```

一切皆 64 位整数——字符串是字符串表索引（str_idx），list / dict / 对象都是句柄 id。

### 函数与 OOP

```
// 顶层函数
method fact(n) {
    if (n < 2) { return 1; }
    return n * fact(n - 1);
}

// 类 + 成员变量 + 方法
class WikidotClient {
    var site : int;
    var token : int;

    method init(site_name) {     // 构造器
        this.site = site_name;
        this.token = "";
    }
    method get_page_source(fullname) { ... }
}

// 静态方法
class System {
    static method println(n) {
        system.print(n);
        system.print_char(10);
    }
}

var client = new WikidotClient("backrooms-wiki-cn");
```

### 控制流

```
if (cond) { ... } else { ... }
while (cond) { ... }
return expr;
```

逻辑运算用关键字 `and` / `or`：

```
x = 1 and 0;   // 0
y = 0 or 1;    // 1
```

### Go 风格特性（v2）

**闭包 / 匿名函数**（复用 `lambda`，无 `func` 关键字）：

```
var add = lambda(a, b) { return a + b; };   // block 形式
var sq  = lambda(x) { return x * x; };
add(3, 4);          // 7
sq(5);              // 25
```

捕获语义为**按值快照**：闭包创建时把外层变量当前值复制进闭包；需要可变共享时使用指针（`&x` / `*p`）。

**多返回值 + error 模式**：

```
method divmod(a, b) (int, int) { return a / b, a % b; }
var q, r = divmod(17, 5);       // q=3, r=2
var only = divmod(10, 3);       // 单接收取第一个：3
divmod(10, 3);                  // 语句级丢弃

method parse_int(s) (int, str) { ... return v, ""; ... return 0, s; }
var v, err = parse_int("42");
```

**defer 延迟调用**（参数立即求值，函数体在帧返回/异常展开时按 LIFO 执行）：

```
defer methodName(args);
defer lambda(v) { ... }(v);
```

**slice / range**：slice 复用 list（动态数组），带 Go 风格 API 别名：

```
var s = slice.new();
slice.append(s, 1);
for k, v in range s { ... }         // list: k=index, v=value
for k, v in range dict_var { ... }  // dict: k=str_idx key, v=value（迭代顺序不确定，与 Go 一致）
for c in range "Hi你" { ... }        // str: 按 rune 迭代
```

> 注：dict 迭代顺序与 Go 一样是非确定的；list/dict/str 之外的对象默认按 list 模板迭代。

## 内置函数

| 类别 | 函数 |
|------|------|
| **system** | `print`, `print_str`, `print_char`, `println`, `exec` |
| **str** | `new`, `new_from_idx`, `concat`, `len`, `get_c`, `append_c`, `delete`, `find`, `slice`, `equal`, `trim`, `replace` |
| **list** | `new`, `push`, `get`, `set`, `pop`, `len`, `delete_at` |
| **dict** | `new`, `put`, `get`, `has`, `delete`, `len` |
| **http** | `request(url, method, body)`, `set_ua`, `add_header`, `get_cookie`, `clear` |
| **转换/时间** | `atoi`, `itoa`, `sleep(ms)`, `now()` |

要点：

- 字符串函数统一使用 **rune 索引**，正确处理中文等多字节字符
- `http.request` 返回 `list[body, status]`，自动维护 Cookie Jar（`cookiejar.New` + 30s 超时），自动设置同源 Referer
- `list.push` / `list.set` / `dict.put` 等指令执行后把容器 id 压回栈，支持链式调用

## lang/ 库与示例

### HtmlParser.mt — HTML 解析库

递归下降 tokenizer + 栈式 DOM 树构建，无需正则。

```
class HtmlNode:
    get_attr(name)          — 属性值
    has_class(class_name)   — class 匹配（单词级）
    get_id()                — id 属性
    find_by_tag / find_all_by_tag    — 按标签名查找后代
    find_by_class / find_all_by_class — 按 class 查找后代
    get_text / get_text_trimmed      — 后代文本拼接

class HtmlParser:
    parse(html) — 解析 HTML，返回根 HtmlNode
```

支持：嵌套标签、自闭合 `<img/>`、void 元素（`br`/`img`/`input` 等 13 种）、`<!-- 注释 -->`、`<!DOCTYPE>`、单/双引号及无引号属性。

### Wikidot.mt — Wikidot-Golang 完整重写

用 method 语言重实现 [Wikidot-Golang](https://github.com/umouyoumou-del/Wikidot-Golang) 的全部 API（内嵌 JsonParser + HtmlParser）：

| 模块 | API |
|------|-----|
| Core | `login`, `ensure_token`, `ensure_www_token`, `call_module`, `call_action`, `call_www_action` |
| Page | `get_page_id`, `get_page_source`, `get_page_html`, `get_page_tags`, `set_page_tags`, `list_pages`, `get_page_history`, `get_page_revision_source` |
| Edit | `acquire_edit_lock`, `release_edit_lock`, `create_page`, `edit_page` |
| Rename/Delete | `rename_page`, `delete_page` |
| Forum | `get_forum_categories`, `create_forum_thread`, `create_forum_post`, `get_forum_thread`, `get_forum_thread_posts` |
| Mail | `lookup_user_id`, `send_mail`, `get_inbox_messages`, `delete_mail` |
| User | `get_user_id` |

实测（backrooms-wiki-cn）：

```
[1] Token 获取          wikidot_token7 len: 32      OK
[2] GetPageID('start')  Page ID: 1312481462          OK
[3] GetPageSource       Source len: 12495            OK
[5] ListPages           HTML len: 219287             OK
[6] GetForumCategories  HTML len: 16875              OK
[7] GetPageHistory      History HTML len: 37007      OK
```

## 架构设计要点

- **栈式 VM**：编译器按顺序压参（arg0…argN-1），栈顶是最后一个参数；VM 先弹栈顶
- **字符串表**：编译期 `InternString` 分配索引，运行时 strTable 与编译期共享前缀，`PushI64(idx) + StrNewFromIdx()` 零拷贝
- **多返回值**：`http.request` 返回两个值，编译器用 `ListNew + Swap + ListPush` 封装成 list
- **.mlr 序列化**：字节码 + 字符串表 + 常量池整体序列化，可脱离源码分发执行

## License

MIT

# method-lang

栈式字节码虚拟机 + 编译器，用 Go 实现。附带 method 语言重写的 Wikidot-Golang 客户端。

## 项目结构

```
method/
├── ast/           — AST 节点定义
├── bytecode/      — opcode 定义
├── compiler/      — method → 字节码编译器
├── lexer/         — 词法分析
├── parser/        — 语法分析
├── vm/            — 栈式字节码虚拟机
├── lang/          — method 语言程序
│   ├── main.mt    — Wikidot-Golang 完整重写
│   ├── hello.mt   — Hello World
│   ├── system.mt  — 系统功能演示
│   ├── bench.mt   — 性能基准
│   ├── concurrent.mt — 并发演示
│   └── demo_exec.mt  — 执行演示
├── go.mod
└── main.go        — 入口
```

## 快速开始

```bash
# 编译并运行 method 程序
go run . lang/main.mt
```

## method 语言特性

### 内置类型
- **int** — 64 位整数（也是 str_idx / list_id / dict_id 的载体）
- **string** — 通过 str 内置函数操作
- **list** — 动态数组
- **dict** — 哈希表

### 内置函数

| 类别 | 函数 |
|------|------|
| **system** | `print`, `print_str`, `print_char` |
| **str** | `new`, `concat`, `len`, `get_c`, `slice`, `find`, `equal`, `trim`, `replace`, `append_c` |
| **list** | `new`, `push`, `get`, `set`, `pop`, `len`, `delete_at` |
| **dict** | `new`, `put`, `get`, `has`, `delete`, `len` |
| **http** | `request(url, method, body)`, `set_ua`, `add_header`, `get_cookie`, `clear` |
| **conv** | `atoi`, `itoa` |
| **time** | `now`, `sleep` |

### OOP 语法

```
class WikidotClient {
    var site : int;
    var token : int;

    method init(site_name) {
        this.site = site_name;
        this.token = "";
    }

    method get_page_source(fullname) {
        // ...
    }
}

var client = new WikidotClient("backrooms-wiki-cn");
```

### 控制流

- `if (cond) { ... } else { ... }`
- `while (cond) { ... }`
- `return expr;`

### 语言限制

method 是极简语言，以下特性**不可用**：

| 不支持 | 替代方案 |
|--------|----------|
| `||` `&&` 逻辑运算符 | 嵌套 `if/else` |
| `break` / `continue` | 条件变量 `while (cont == 1)` |
| 三元运算符 `?:` | `if/else` |
| `import` / module 系统 | 所有代码内嵌单文件 |
| 浮点数 | 整数运算 |
| HTML DOM 解析 | `str.find` / `str.slice` 字符串扫描 |

## Wikidot-Golang 重写 (lang/main.mt)

用 method 语言完整重实现了 [Wikidot-Golang](https://github.com/umouyoumou-del/Wikidot-Golang) 的全部 API。

### 已实现 API

**Core**
- `login(username, password)` — POST www.wikidot.com Login2Action
- `ensure_token()` / `ensure_www_token()` — 站点 / www 子域 CSRF token
- `call_module` / `call_action` / `call_www_action` — AJAX Module Connector 封装

**Page**
- `get_page_id(fullname)` — 从 HTML 提取 pageId
- `get_page_source(fullname)` — viewsource/ViewSourceModule
- `get_page_html(fullname)` — GET 页面原始 HTML
- `get_page_tags(fullname)` — 从 HTML 提取 page-tags
- `set_page_tags(fullname, tags_str)` — WikiPageAction/saveTags
- `list_pages(category, tags, per_page)` — list/ListPagesModule
- `get_page_history(fullname)` — history/PageRevisionListModule
- `get_page_revision_source(revision_id)` — history/PageSourceModule

**Edit**
- `acquire_edit_lock(fullname, page_id)` — edit/PageEditModule
- `release_edit_lock(...)` — WikiPageAction/removePageEditLock
- `create_page(fullname, title, content, tags, comment)` — WikiPageAction/savePage
- `edit_page(fullname, title, content, tags, comment)` — WikiPageAction/savePage

**Rename / Delete**
- `rename_page(fullname, new_name)` — WikiPageAction/renamePage
- `delete_page(fullname)` — WikiPageAction/deletePage

**Forum**
- `get_forum_categories()` — forum/ForumStartModule
- `create_forum_thread(category_id, title, content)` — ForumAction/newThread
- `create_forum_post(thread_id, content)` — ForumAction/savePost
- `get_forum_thread(thread_id)` — forum/ForumViewThreadModule
- `get_forum_thread_posts(thread_id, page_no)` — forum/ForumViewThreadPostsModule

**Mail**
- `lookup_user_id(username)` — quickmodule UserLookupQModule
- `send_mail(username, subject, content)` — DashboardMessageAction/send
- `get_inbox_messages(page)` — dashboard/messages/DMInboxModule
- `delete_mail(message_id)` — DashboardMessageAction/delete

**User**
- `get_user_id(username)` — www.wikidot.com/user:info

### 实测验证 (backrooms-wiki-cn)

```
[1] Token 获取          wikidot_token7 len: 32      OK
[2] GetPageID('start')  Page ID: 1312481462          OK
[3] GetPageSource       Source len: 12495            OK
[4] GetPageTags         Tags count: 0                OK
[5] ListPages           HTML len: 219287             OK
[6] GetForumCategories  HTML len: 16875              OK
[7] GetPageHistory      History HTML len: 37007      OK
```

### 内嵌组件

- **JsonParser** — 递归下降 JSON 解析器（无 break，纯条件循环）
- **HtmlHelper** — HTML 字符串扫描辅助（提取 pageId / title / source / tags）

## 技术细节

### VM 栈约定

编译器按顺序压参数（arg0, arg1, ..., argN-1），栈顶是最后一个参数。VM 先弹栈顶再弹前面的。

### 容器指令链式返回

`list.push` / `list.set` / `dict.put` 等指令执行后把容器 id push 回栈，支持链式调用。编译器语句级调用后额外 Pop 掉多余的 id。

### HTTP Cookie Jar

VM 使用 `cookiejar.New` + `http.Client(Timeout=30s, InsecureSkipVerify)`。站点 HTTP 走这个 Client，平台 www.wikidot.com 登录走同一 Client（会话 cookie 跨子域共享）。

### Rune 索引统一

`str.find` / `str.get_c` / `str.slice` 统一使用 rune 索引（非字节索引），正确处理中文等多字节字符。

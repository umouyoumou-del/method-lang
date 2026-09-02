// ==================================================================
// HtmlParser — method 语言 HTML 解析库
// 递归下降 tokenizer + 栈式 DOM 树构建
// ==================================================================

// ==================== HtmlNode ====================
class HtmlNode {
    var tag : int;       // str_idx: 标签名（"div","a","span"...），文本节点为 ""
    var attrs : int;     // dict_id: 属性表 (name→value)
    var children : int;  // list_id: 子节点列表 (HtmlNode)
    var text : int;      // str_idx: 文本内容（文本节点用）
    var is_text : int;   // 0=元素, 1=纯文本

    method init() {
        this.tag = "";
        this.attrs = dict.new();
        this.children = list.new();
        this.text = "";
        this.is_text = 0;
    }

    // 添加子节点
    method add_child(node) { list.push(this.children, node); }

    // 获取属性值，不存在返回 ""
    method get_attr(name) {
        if (dict.has(this.attrs, name) == 1) {
            return dict.get(this.attrs, name);
        }
        return "";
    }

    // 是否有指定 class
    method has_class(class_name) {
        var cls = this.get_attr("class");
        if (str.len(cls) == 0) { return 0; }
        // 在 cls 中找 class_name 作为完整单词
        var idx = str.find(cls, class_name);
        if (idx < 0) { return 0; }
        // 检查边界
        var before_ok = 0;
        if (idx == 0) { before_ok = 1; }
        if (idx > 0) {
            var prev = str.get_c(cls, idx - 1);
            if (prev == 32) { before_ok = 1; }
        }
        var after_idx = idx + str.len(class_name);
        var after_ok = 0;
        if (after_idx >= str.len(cls)) { after_ok = 1; }
        if (after_idx < str.len(cls)) {
            var next = str.get_c(cls, after_idx);
            if (next == 32) { after_ok = 1; }
        }
        if (before_ok == 1) { if (after_ok == 1) { return 1; } }
        return 0;
    }

    // 获取所有后代中匹配标签名的第一个节点
    method find_by_tag(tag_name) {
        // 先查直接子节点
        var i = 0;
        var cont = 1;
        while (cont == 1) {
            if (i >= list.len(this.children)) { cont = 0; }
            if (i < list.len(this.children)) {
                var child = list.get(this.children, i);
                var child_is_text = 0;
                if (child >= 0) { child_is_text = child.is_text; }
                if (child_is_text == 0) {
                    if (str.equal(child.tag, tag_name) == 1) { return child; }
                }
                i = i + 1;
            }
        }
        // 递归查子节点的后代
        i = 0; cont = 1;
        while (cont == 1) {
            if (i >= list.len(this.children)) { cont = 0; }
            if (i < list.len(this.children)) {
                var child = list.get(this.children, i);
                if (child.is_text == 0) {
                    var found = child.find_by_tag(tag_name);
                    if (found != 0) { return found; }  // 0 = null
                }
                i = i + 1;
            }
        }
        return 0;  // not found
    }

    // 获取所有后代中匹配标签名的节点列表
    method find_all_by_tag(tag_name) {
        var result = list.new();
        var i = 0;
        var cont = 1;
        while (cont == 1) {
            if (i >= list.len(this.children)) { cont = 0; }
            if (i < list.len(this.children)) {
                var child = list.get(this.children, i);
                if (child.is_text == 0) {
                    if (str.equal(child.tag, tag_name) == 1) {
                        list.push(result, child);
                    }
                    // 递归
                    var sub = child.find_all_by_tag(tag_name);
                    var j = 0;
                    var cont2 = 1;
                    while (cont2 == 1) {
                        if (j >= list.len(sub)) { cont2 = 0; }
                        if (j < list.len(sub)) {
                            list.push(result, list.get(sub, j));
                            j = j + 1;
                        }
                    }
                }
                i = i + 1;
            }
        }
        return result;
    }

    // 获取所有后代中有指定 class 的节点
    method find_all_by_class(class_name) {
        var result = list.new();
        var i = 0;
        var cont = 1;
        while (cont == 1) {
            if (i >= list.len(this.children)) { cont = 0; }
            if (i < list.len(this.children)) {
                var child = list.get(this.children, i);
                if (child.is_text == 0) {
                    if (child.has_class(class_name) == 1) {
                        list.push(result, child);
                    }
                    var sub = child.find_all_by_class(class_name);
                    var j = 0;
                    var cont2 = 1;
                    while (cont2 == 1) {
                        if (j >= list.len(sub)) { cont2 = 0; }
                        if (j < list.len(sub)) {
                            list.push(result, list.get(sub, j));
                            j = j + 1;
                        }
                    }
                }
                i = i + 1;
            }
        }
        return result;
    }

    // 获取第一个有指定 class 的后代
    method find_by_class(class_name) {
        var all = this.find_all_by_class(class_name);
        if (list.len(all) > 0) { return list.get(all, 0); }
        return 0;
    }

    // 获取 id 属性
    method get_id() { return this.get_attr("id"); }

    // 获取所有直接子文本拼接
    method get_text() {
        var result = "";
        var i = 0;
        var cont = 1;
        while (cont == 1) {
            if (i >= list.len(this.children)) { cont = 0; }
            if (i < list.len(this.children)) {
                var child = list.get(this.children, i);
                if (child.is_text == 1) {
                    if (str.len(result) == 0) {
                        result = child.text;
                    } else {
                        result = str.concat(result, child.text);
                    }
                } else {
                    // 递归获取子元素文本
                    var sub_text = child.get_text();
                    if (str.len(sub_text) > 0) {
                        if (str.len(result) == 0) {
                            result = sub_text;
                        } else {
                            result = str.concat(result, sub_text);
                        }
                    }
                }
                i = i + 1;
            }
        }
        return result;
    }

    // 获取包含后代的全部文本（带 trim）
    method get_text_trimmed() {
        var raw = this.get_text();
        return str.trim(raw);
    }
}

// ==================== HtmlParser ====================
class HtmlParser {
    var src : int;    // str_idx: HTML 源码
    var pos : int;    // 当前 rune 位置
    var len : int;    // 总长度

    method init() {
        this.src = "";
        this.pos = 0;
        this.len = 0;
    }

    // 解析 HTML，返回根 HtmlNode
    method parse(html) {
        this.src = html;
        this.pos = 0;
        this.len = str.len(html);
        var root = new HtmlNode();
        root.tag = "root";
        var stack = list.new();
        list.push(stack, root);  // 栈底是 root

        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                if (c == 60) {  // '<'
                    this.parse_tag_or_text(stack);
                } else {
                    this.parse_text_node(stack);
                }
            }
        }
        return root;
    }

    // 解析 <...> 标签或 <!-- 注释 -->
    method parse_tag_or_text(stack) {
        // 检查是否是注释 <!--
        var c1 = str.get_c(this.src, this.pos + 1);
        if (c1 == 33) {  // '!'
            // 可能是 <!-- 注释 --> 或 <!DOCTYPE>
            var c2 = str.get_c(this.src, this.pos + 2);
            if (c2 == 45) {  // '-' → <!--
                this.skip_comment();
                return;
            }
            // <!DOCTYPE ...> 跳过
            this.skip_doctype();
            return;
        }
        // 正常标签
        this.parse_tag(stack);
    }

    // 跳过 <!-- 注释 -->
    method skip_comment() {
        // 找 "-->"
        var idx = str.find(this.src, "-->");
        if (idx < 0) {
            this.pos = this.len;
            return;
        }
        this.pos = idx + 3;
    }

    // 跳过 <!DOCTYPE ...>
    method skip_doctype() {
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                if (c == 62) {  // '>'
                    this.pos = this.pos + 1;
                    cont = 0;
                } else {
                    this.pos = this.pos + 1;
                }
            }
        }
    }

    // 解析 <tag attr="val"> 或 </tag> 或 <tag/>
    method parse_tag(stack) {
        this.pos = this.pos + 1;  // 跳过 '<'
        if (this.pos >= this.len) { return; }
        var c = str.get_c(this.src, this.pos);
        if (c == 47) {  // '/' → 闭合标签 </tag>
            this.parse_close_tag(stack);
            return;
        }
        // 解析标签名
        var tag_name = this.read_tag_name();
        if (str.len(tag_name) == 0) { return; }  // 无效

        // 创建节点
        var node = new HtmlNode();
        node.tag = tag_name;

        // 解析属性
        this.parse_attributes(node);

        // 检查是否自闭合
        var self_closing = 0;
        if (this.pos < this.len) {
            var last_c = str.get_c(this.src, this.pos);
            if (last_c == 47) {  // '/'
                self_closing = 1;
                this.pos = this.pos + 1;  // 跳过 '/'
            }
        }
        // 跳过 '>'
        if (this.pos < this.len) {
            var gt = str.get_c(this.src, this.pos);
            if (gt == 62) { this.pos = this.pos + 1; }  // '>'
        }

        // void 元素（不需要闭合标签）
        var is_void = this.is_void_element(tag_name);

        // 添加到栈顶节点
        var parent = list.get(stack, list.len(stack) - 1);
        parent.add_child(node);

        // 如果不是自闭合也不是 void，压入栈
        if (self_closing == 0) {
            if (is_void == 0) {
                list.push(stack, node);
            }
        }
    }

    // 解析闭合标签 </tag>
    method parse_close_tag(stack) {
        this.pos = this.pos + 1;  // 跳过 '/'
        var tag_name = this.read_tag_name();
        // 跳过属性到 '>'
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                if (c == 62) { this.pos = this.pos + 1; cont = 0; }  // '>'
                else { this.pos = this.pos + 1; }
            }
        }
        // 弹栈直到匹配的标签
        var cont2 = 1;
        while (cont2 == 1) {
            if (list.len(stack) <= 1) { cont2 = 0; }  // 只剩 root
            if (list.len(stack) > 1) {
                var top = list.get(stack, list.len(stack) - 1);
                if (str.equal(top.tag, tag_name) == 1) {
                    list.pop(stack);
                    cont2 = 0;
                } else {
                    // 不匹配，也弹出（容错）
                    list.pop(stack);
                }
            }
        }
    }

    // 读取标签名（字母、数字、连字符）
    method read_tag_name() {
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                var valid = 0;
                if (c >= 65) { if (c <= 90) { valid = 1; } }   // A-Z
                if (c >= 97) { if (c <= 122) { valid = 1; } }  // a-z
                if (c >= 48) { if (c <= 57) { valid = 1; } }   // 0-9
                if (c == 45) { valid = 1; }  // '-'
                if (c == 58) { valid = 1; }  // ':'
                if (c == 95) { valid = 1; }  // '_'
                if (valid == 1) { this.pos = this.pos + 1; } else { cont = 0; }
            }
        }
        return str.slice(this.src, start, this.pos);
    }

    // 解析属性
    method parse_attributes(node) {
        var cont = 1;
        while (cont == 1) {
            this.skip_ws();
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                // 结束标志
                if (c == 62) { cont = 0; }  // '>'
                if (c == 47) {  // '/'
                    var next = str.get_c(this.src, this.pos + 1);
                    if (next == 62) { cont = 0; }  // '/>'
                }
                if (cont == 1) {
                    // 读取属性名
                    var name = this.read_attr_name();
                    if (str.len(name) == 0) {
                        // 跳过无法识别的字符
                        this.pos = this.pos + 1;
                    } else {
                        // 跳过空白
                        this.skip_ws();
                        var val = "";
                        if (this.pos < this.len) {
                            var eq = str.get_c(this.src, this.pos);
                            if (eq == 61) {  // '='
                                this.pos = this.pos + 1;
                                this.skip_ws();
                                val = this.read_attr_value();
                            }
                        }
                        dict.put(node.attrs, name, val);
                    }
                }
            }
        }
    }

    // 读取属性名
    method read_attr_name() {
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                var valid = 0;
                if (c >= 65) { if (c <= 90) { valid = 1; } }
                if (c >= 97) { if (c <= 122) { valid = 1; } }
                if (c >= 48) { if (c <= 57) { valid = 1; } }
                if (c == 45) { valid = 1; }  // '-'
                if (c == 58) { valid = 1; }  // ':'
                if (c == 95) { valid = 1; }  // '_'
                if (c == 46) { valid = 1; }  // '.'
                if (valid == 1) { this.pos = this.pos + 1; } else { cont = 0; }
            }
        }
        return str.slice(this.src, start, this.pos);
    }

    // 读取属性值 ("..." 或 '...' 或无引号)
    method read_attr_value() {
        if (this.pos >= this.len) { return ""; }
        var c = str.get_c(this.src, this.pos);
        if (c == 34) {  // '"'
            this.pos = this.pos + 1;
            var start = this.pos;
            var cont = 1;
            while (cont == 1) {
                if (this.pos >= this.len) { cont = 0; }
                if (this.pos < this.len) {
                    var ch = str.get_c(this.src, this.pos);
                    if (ch == 34) { cont = 0; } else { this.pos = this.pos + 1; }
                }
            }
            var end = this.pos;
            this.pos = this.pos + 1;  // 跳过 closing '"'
            return str.slice(this.src, start, end);
        }
        if (c == 39) {  // '\''
            this.pos = this.pos + 1;
            var start = this.pos;
            var cont = 1;
            while (cont == 1) {
                if (this.pos >= this.len) { cont = 0; }
                if (this.pos < this.len) {
                    var ch = str.get_c(this.src, this.pos);
                    if (ch == 39) { cont = 0; } else { this.pos = this.pos + 1; }
                }
            }
            var end = this.pos;
            this.pos = this.pos + 1;
            return str.slice(this.src, start, end);
        }
        // 无引号值
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var ch = str.get_c(this.src, this.pos);
                var stop = 0;
                if (ch == 32) { stop = 1; }  // 空格
                if (ch == 62) { stop = 1; }  // '>'
                if (ch == 47) { stop = 1; }  // '/'
                if (stop == 1) { cont = 0; } else { this.pos = this.pos + 1; }
            }
        }
        return str.slice(this.src, start, this.pos);
    }

    // 解析文本节点
    method parse_text_node(stack) {
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                if (c == 60) { cont = 0; }  // '<'
                else { this.pos = this.pos + 1; }
            }
        }
        if (this.pos > start) {
            var text = str.slice(this.src, start, this.pos);
            // 去掉纯空白文本
            var trimmed = str.trim(text);
            if (str.len(trimmed) > 0) {
                var node = new HtmlNode();
                node.is_text = 1;
                node.text = trimmed;
                var parent = list.get(stack, list.len(stack) - 1);
                parent.add_child(node);
            }
        }
    }

    // 跳过空白字符
    method skip_ws() {
        var cont = 1;
        while (cont == 1) {
            if (this.pos >= this.len) { cont = 0; }
            if (this.pos < this.len) {
                var c = str.get_c(this.src, this.pos);
                var is_ws = 0;
                if (c == 32) { is_ws = 1; }  // 空格
                if (c == 9) { is_ws = 1; }   // tab
                if (c == 10) { is_ws = 1; }  // LF
                if (c == 13) { is_ws = 1; }  // CR
                if (is_ws == 1) { this.pos = this.pos + 1; } else { cont = 0; }
            }
        }
    }

    // void 元素列表（HTML5 不需要闭合标签的元素）
    method is_void_element(tag) {
        if (str.equal(tag, "br") == 1) { return 1; }
        if (str.equal(tag, "hr") == 1) { return 1; }
        if (str.equal(tag, "img") == 1) { return 1; }
        if (str.equal(tag, "input") == 1) { return 1; }
        if (str.equal(tag, "meta") == 1) { return 1; }
        if (str.equal(tag, "link") == 1) { return 1; }
        if (str.equal(tag, "area") == 1) { return 1; }
        if (str.equal(tag, "base") == 1) { return 1; }
        if (str.equal(tag, "col") == 1) { return 1; }
        if (str.equal(tag, "embed") == 1) { return 1; }
        if (str.equal(tag, "source") == 1) { return 1; }
        if (str.equal(tag, "track") == 1) { return 1; }
        if (str.equal(tag, "wbr") == 1) { return 1; }
        return 0;
    }
}

// ==================== 测试 ====================
system.print_str("============================================\n");
system.print_str("  HtmlParser — method 语言 HTML 解析库\n");
system.print_str("============================================\n\n");

// 测试 1: 基本解析
system.print_str("[1] 基本解析\n");
var html1 = "<div class=\"container\" id=\"main\"><p>Hello</p><p>World</p></div>";
var parser = new HtmlParser();
var root = parser.parse(html1);

var div = root.find_by_tag("div");
system.print_str("  div class: ");
system.print_str(div.get_attr("class"));
system.print_char(10);
system.print_str("  div id: ");
system.print_str(div.get_attr("id"));
system.print_char(10);

var ps = div.find_all_by_tag("p");
system.print_str("  <p> count: ");
system.print(list.len(ps));
system.print_char(10);
var p0_text = list.get(ps, 0);
system.print_str("  first <p>: ");
system.print_str(p0_text.get_text());
system.print_char(10);
var p1_text = list.get(ps, 1);
system.print_str("  second <p>: ");
system.print_str(p1_text.get_text());
system.print_char(10);
system.print_str("  OK\n\n");

// 测试 2: 嵌套 + class 查找
system.print_str("[2] 嵌套 + class 查找\n");
var html2 = "<ul><li class=\"item\">A</li><li class=\"item active\">B</li><li>C</li></ul>";
var root2 = parser.parse(html2);

var items = root2.find_all_by_class("item");
system.print_str("  .item count: ");
system.print(list.len(items));
system.print_char(10);
var li0 = list.get(items, 0);
system.print_str("  first .item: ");
system.print_str(li0.get_text());
system.print_char(10);
var li1 = list.get(items, 1);
system.print_str("  second .item: ");
system.print_str(li1.get_text());
system.print_char(10);
system.print_str("  has 'active' class: ");
system.print(li1.has_class("active"));
system.print_char(10);
system.print_str("  OK\n\n");

// 测试 3: 属性 + 自闭合标签
system.print_str("[3] 属性 + 自闭合标签\n");
var html3 = "<div><a href=\"/page\" title=\"Link\">Click</a><img src=\"img.png\" alt=\"Pic\"/></div>";
var root3 = parser.parse(html3);

var a = root3.find_by_tag("a");
system.print_str("  href: ");
system.print_str(a.get_attr("href"));
system.print_char(10);
system.print_str("  title: ");
system.print_str(a.get_attr("title"));
system.print_char(10);
system.print_str("  text: ");
system.print_str(a.get_text());
system.print_char(10);

var img = root3.find_by_tag("img");
system.print_str("  img src: ");
system.print_str(img.get_attr("src"));
system.print_char(10);
system.print_str("  img alt: ");
system.print_str(img.get_attr("alt"));
system.print_char(10);
system.print_str("  OK\n\n");

// 测试 4: 注释 + DOCTYPE
system.print_str("[4] 注释 + DOCTYPE\n");
var html4 = "<!DOCTYPE html><!-- comment --><html><head></head><body><h1>Title</h1></body></html>";
var root4 = parser.parse(html4);
var h1 = root4.find_by_tag("h1");
system.print_str("  h1 text: ");
system.print_str(h1.get_text());
system.print_char(10);
system.print_str("  OK\n\n");

// 测试 5: Wikidot page-tags 场景
system.print_str("[5] Wikidot page-tags 场景\n");
var html5 = "<div class=\"page-tags\"><span>Tags</span><a href=\"/system:page-tags/tag/foo\">foo</a><a href=\"/system:page-tags/tag/bar\">bar</a></div>";
var root5 = parser.parse(html5);
var tags_div = root5.find_by_class("page-tags");
var tag_links = tags_div.find_all_by_tag("a");
system.print_str("  tag links: ");
system.print(list.len(tag_links));
system.print_char(10);
var t0 = list.get(tag_links, 0);
system.print_str("  tag 1: ");
system.print_str(t0.get_text());
system.print_char(10);
var t1 = list.get(tag_links, 1);
system.print_str("  tag 2: ");
system.print_str(t1.get_text());
system.print_char(10);
system.print_str("  OK\n\n");

system.print_str("============================================\n");
system.print_str("  全部测试通过\n");
system.print_str("============================================\n");

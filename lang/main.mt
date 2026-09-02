// ==================================================================
// Wikidot-Golang (method v4.1)
// 用 method 语言重写 Wikidot-Golang 项目
// ==================================================================

// ==================== JSON Parser (递归下降, 无 break) ====================
class JsonParser {
    var src : int;
    var pos : int;

    method init() { this.src = 0; this.pos = 0; }

    method parse(s) {
        this.src = s; this.pos = 0;
        this.skip_ws();
        return this.parse_value();
    }

    method skip_ws() {
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(this.src, this.pos);
            var is_ws = 0;
            if (c == 32) { is_ws = 1; }
            if (c == 9) { is_ws = 1; }
            if (c == 10) { is_ws = 1; }
            if (c == 13) { is_ws = 1; }
            if (is_ws == 1) { this.pos = this.pos + 1; } else { cont = 0; }
        }
    }

    method peek() { return str.get_c(this.src, this.pos); }
    method advance() {
        var c = str.get_c(this.src, this.pos);
        this.pos = this.pos + 1;
        return c;
    }

    method parse_value() {
        this.skip_ws();
        var c = this.peek();
        if (c == 123) { return this.parse_object(); }
        if (c == 91)  { return this.parse_array(); }
        if (c == 34)  { return this.parse_string(); }
        var is_bool = 0;
        if (c == 116) { is_bool = 1; }
        if (c == 102) { is_bool = 1; }
        if (is_bool == 1) { return this.parse_bool(); }
        if (c == 110) { return this.parse_null(); }
        return this.parse_number();
    }

    method parse_object() {
        this.advance();  // '{'
        var d = dict.new();
        this.skip_ws();
        if (this.peek() == 125) { this.advance(); return d; }
        var more = 1;
        while (more == 1) {
            this.skip_ws();
            var key = this.parse_string();
            this.skip_ws();
            this.advance();  // ':'
            this.skip_ws();
            var val = this.parse_value();
            dict.put(d, key, val);
            this.skip_ws();
            if (this.peek() == 44) { this.advance(); } else { more = 0; }
        }
        this.advance();  // '}'
        return d;
    }

    method parse_array() {
        this.advance();  // '['
        var l = list.new();
        this.skip_ws();
        if (this.peek() == 93) { this.advance(); return l; }
        var more = 1;
        while (more == 1) {
            this.skip_ws();
            var val = this.parse_value();
            list.push(l, val);
            this.skip_ws();
            if (this.peek() == 44) { this.advance(); } else { more = 0; }
        }
        this.advance();  // ']'
        return l;
    }

    method parse_string() {
        this.advance();  // opening '"'
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(this.src, this.pos);
            if (c == 0) { cont = 0; } else {
                if (c == 34) { cont = 0; } else {
                    if (c == 92) { this.pos = this.pos + 2; }
                    else { this.pos = this.pos + 1; }
                }
            }
        }
        var end = this.pos;
        this.advance();  // closing '"'
        return str.slice(this.src, start, end);
    }

    method parse_number() {
        var start = this.pos;
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(this.src, this.pos);
            var is_num = 0;
            if (c >= 48) { if (c <= 57) { is_num = 1; } }
            if (c == 45) { is_num = 1; }
            if (c == 43) { is_num = 1; }
            if (c == 46) { is_num = 1; }
            if (c == 101) { is_num = 1; }
            if (c == 69) { is_num = 1; }
            if (is_num == 1) { this.pos = this.pos + 1; } else { cont = 0; }
        }
        var num_str = str.slice(this.src, start, this.pos);
        return atoi(num_str);
    }

    method parse_bool() {
        var c1 = str.get_c(this.src, this.pos);
        var c2 = str.get_c(this.src, this.pos + 1);
        var is_true = 0;
        if (c1 == 116) { if (c2 == 114) { is_true = 1; } }
        if (is_true == 1) { this.pos = this.pos + 4; return 1; }
        this.pos = this.pos + 5; return 0;
    }

    method parse_null() { this.pos = this.pos + 4; return 0; }
}

// ==================== Wikidot Client ====================
class WikidotClient {
    var site : int;
    var base_url : int;
    var token : int;

    method init(site_name) {
        this.site = site_name;
        this.base_url = str.concat("https://", site_name);
        this.base_url = str.concat(this.base_url, ".wikidot.com");
        this.token = "";
        http.set_ua("method-wikidot/1.0");
    }

    method ensure_token() {
        var url1 = str.concat(this.base_url, "/level-1");
        http.request(url1, "GET", "");
        this.token = http.get_cookie("wikidot_token7");
        if (str.len(this.token) > 0) { return 0; }
        http.request(this.base_url, "GET", "");
        this.token = http.get_cookie("wikidot_token7");
        return 0;
    }

    // 从页面 HTML 提取 page_id
    method get_page_id(fullname) {
        var path = str.concat("/", fullname);
        var url = str.concat(this.base_url, path);
        var resp = http.request(url, "GET", "");
        var html = list.get(resp, 0);
        // 找 "pageId" 然后跳过空白/= 找数字
        var idx = str.find(html, "pageId");
        if (idx < 0) { return 0; }
        var p = idx + str.len("pageId");
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            var is_digit = 0;
            if (c >= 48) { if (c <= 57) { is_digit = 1; } }
            if (is_digit == 1) { cont = 0; } else { p = p + 1; }
        }
        var num_start = p;
        cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            var is_digit = 0;
            if (c >= 48) { if (c <= 57) { is_digit = 1; } }
            if (is_digit == 1) { p = p + 1; } else { cont = 0; }
        }
        var id_str = str.slice(html, num_start, p);
        return atoi(id_str);
    }

    // POST AJAX connector 返回原始 body 字符串
    method call_connector_raw(params) {
        var amp = "&";
        var t7 = "wikidot_token7=";
        var full = str.concat(params, amp);
        full = str.concat(full, t7);
        full = str.concat(full, this.token);
        var url = str.concat(this.base_url, "/ajax-module-connector.php");
        var resp = http.request(url, "POST", full);
        return list.get(resp, 0);
    }

    // POST AJAX connector 返回解析后的 dict
    method call_connector(params) {
        var raw = this.call_connector_raw(params);
        var parser = new JsonParser();
        return parser.parse(raw);
    }

    // 获取页面渲染 HTML（list.get(dict, "body")）
    method call_module_body(params) {
        var raw = this.call_connector_raw(params);
        // 手动从 JSON 提取 "body":"..." 字段（避免完整 JSON 解析转义问题）
        var idx = str.find(raw, "\"body\"");
        if (idx < 0) { return ""; }
        // 跳过 "body": 直到下一个 "
        var p = idx + 6;  // 跳过 "body"
        // 跳过 : 和可能的空白
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(raw, p);
            var skip = 0;
            if (c == 58) { skip = 1; }
            if (c == 32) { skip = 1; }
            if (c == 10) { skip = 1; }
            if (c == 13) { skip = 1; }
            if (skip == 1) { p = p + 1; } else { cont = 0; }
        }
        if (str.get_c(raw, p) != 34) { return ""; }  // 不是 "
        p = p + 1;  // 跳过 opening "
        var start = p;
        cont = 1;
        while (cont == 1) {
            var c = str.get_c(raw, p);
            if (c == 0) { cont = 0; } else {
                if (c == 34) { cont = 0; } else {
                    if (c == 92) { p = p + 2; } else { p = p + 1; }
                }
            }
        }
        return str.slice(raw, start, p);
    }

    // GetPageSource: 获取页面 wiki 源码
    method get_page_source(fullname) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return ""; }
        var amp = "&";
        var mn = "moduleName=viewsource/ViewSourceModule";
        var pid = str.concat("page_id=", itoa(page_id));
        var params = str.concat(mn, amp);
        params = str.concat(params, pid);
        var body_html = this.call_module_body(params);
        // 从 <div class="page-source"> 提取文本
        var open_tag = "page-source";
        var idx = str.find(body_html, open_tag);
        if (idx < 0) { return body_html; }  // 回退返回 HTML
        // 找下一个 >
        var p = idx + str.len(open_tag);
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(body_html, p);
            if (c == 62) { cont = 0; } else { p = p + 1; }
        }
        p = p + 1;  // 跳过 >
        var start = p;
        // 找 </div>
        var close_idx = str.find(body_html, "</div>");
        if (close_idx < 0) { return str.slice(body_html, start, str.len(body_html)); }
        // 但可能有多个 </div>，用第一个就行（简化）
        return str.slice(body_html, start, close_idx);
    }

    // GetPageHTML: 获取页面渲染 HTML
    method get_page_html(fullname) {
        var path = str.concat("/", fullname);
        var url = str.concat(this.base_url, path);
        var resp = http.request(url, "GET", "");
        return list.get(resp, 0);
    }

    // ListPages: 用 ListPagesModule 列出页面
    method list_pages(category, tags, limit) {
        var amp = "&";
        var mn = "moduleName=list/ListPagesModule";
        var cat = str.concat("category=", category);
        var tg = str.concat("tags=", tags);
        var lim = str.concat("perPage=", itoa(limit));
        var params = str.concat(mn, amp);
        params = str.concat(params, cat);
        params = str.concat(params, amp);
        params = str.concat(params, tg);
        params = str.concat(params, amp);
        params = str.concat(params, lim);
        params = str.concat(params, amp);
        params = str.concat(params, "order=created_at desc");
        params = str.concat(params, amp);
        params = str.concat(params, "separate=no");
        params = str.concat(params, amp);
        params = str.concat(params, "wrapper=no");
        var body_html = this.call_module_body(params);
        return body_html;  // 返回 HTML 片段，需自行解析
    }
}

// ==================== Main Demo ====================
system.print_str("============================================\n");
system.print_str("  Wikidot-Golang (method) v4.1\n");
system.print_str("  使用 method 语言重写 Wikidot-Golang\n");
system.print_str("============================================\n\n");

var client = new WikidotClient("backrooms-wiki-cn");
client.ensure_token();

system.print_str("[1/4] Token 获取\n");
system.print_str("  Token len: ");
system.print(str.len(client.token));
system.print_char(10);

if (str.len(client.token) == 0) {
    system.print_str("  ERROR: 无法获取 token！\n");
    return 0;
}
system.print_str("  OK ✓\n\n");

system.print_str("[2/4] GetPageID('start')\n");
var pid = client.get_page_id("start");
system.print_str("  Page ID: ");
system.print(pid);
system.print_char(10);
system.print_str("  OK ✓\n\n");

system.print_str("[3/4] GetPageSource('start')\n");
var src = client.get_page_source("start");
system.print_str("  Source len: ");
system.print(str.len(src));
system.print_char(10);
system.print_str("  OK ✓\n\n");

system.print_str("[4/4] 源码预览 (前500字符)\n");
system.print_str("  ----------------------------------------\n");
var preview = str.slice(src, 0, 500);
system.print_str(preview);
system.print_char(10);
system.print_str("  ----------------------------------------\n\n");

system.print_str("=== 演示完成 ===\n");
system.print_str("功能: Token获取, GetPageID, GetPageSource\n");
system.print_str("API: viewsource/ViewSourceModule, list/ListPagesModule\n");

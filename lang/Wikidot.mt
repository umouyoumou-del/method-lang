// ==================================================================
// Wikidot-Golang (method) — 完整重写
// 用 method 语言重实现 Wikidot-Golang 全部 API
// ==================================================================

// ==================== JSON Parser (递归下降, 无 break) ====================
class JsonParser {
    var src : int; var pos : int;
    method init() { this.src = 0; this.pos = 0; }
    method parse(s) { this.src = s; this.pos = 0; this.skip_ws(); return this.parse_value(); }
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
    method advance() { var c = str.get_c(this.src, this.pos); this.pos = this.pos + 1; return c; }
    method parse_value() {
        this.skip_ws(); var c = this.peek();
        if (c == 123) { return this.parse_object(); }
        if (c == 91) { return this.parse_array(); }
        if (c == 34) { return this.parse_string(); }
        var is_bool = 0; if (c == 116) { is_bool = 1; } if (c == 102) { is_bool = 1; }
        if (is_bool == 1) { return this.parse_bool(); }
        if (c == 110) { return this.parse_null(); }
        return this.parse_number();
    }
    method parse_object() {
        this.advance(); var d = dict.new(); this.skip_ws();
        if (this.peek() == 125) { this.advance(); return d; }
        var more = 1;
        while (more == 1) {
            this.skip_ws(); var key = this.parse_string();
            this.skip_ws(); this.advance(); this.skip_ws();
            var val = this.parse_value(); dict.put(d, key, val);
            this.skip_ws();
            if (this.peek() == 44) { this.advance(); } else { more = 0; }
        }
        this.advance(); return d;
    }
    method parse_array() {
        this.advance(); var l = list.new(); this.skip_ws();
        if (this.peek() == 93) { this.advance(); return l; }
        var more = 1;
        while (more == 1) {
            this.skip_ws(); var val = this.parse_value(); list.push(l, val);
            this.skip_ws();
            if (this.peek() == 44) { this.advance(); } else { more = 0; }
        }
        this.advance(); return l;
    }
    method parse_string() {
        this.advance(); var start = this.pos; var cont = 1;
        while (cont == 1) {
            var c = str.get_c(this.src, this.pos);
            if (c == 0) { cont = 0; } else {
                if (c == 34) { cont = 0; } else {
                    if (c == 92) { this.pos = this.pos + 2; } else { this.pos = this.pos + 1; }
                }
            }
        }
        var end = this.pos; this.advance();
        return str.slice(this.src, start, end);
    }
    method parse_number() {
        var start = this.pos; var cont = 1;
        while (cont == 1) {
            var c = str.get_c(this.src, this.pos);
            var is_num = 0;
            if (c >= 48) { if (c <= 57) { is_num = 1; } }
            if (c == 45) { is_num = 1; } if (c == 43) { is_num = 1; }
            if (c == 46) { is_num = 1; } if (c == 101) { is_num = 1; } if (c == 69) { is_num = 1; }
            if (is_num == 1) { this.pos = this.pos + 1; } else { cont = 0; }
        }
        return atoi(str.slice(this.src, start, this.pos));
    }
    method parse_bool() {
        var c1 = str.get_c(this.src, this.pos); var c2 = str.get_c(this.src, this.pos + 1);
        var is_true = 0; if (c1 == 116) { if (c2 == 114) { is_true = 1; } }
        if (is_true == 1) { this.pos = this.pos + 4; return 1; }
        this.pos = this.pos + 5; return 0;
    }
    method parse_null() { this.pos = this.pos + 4; return 0; }

    // 从 JSON 字符串提取某个 key 的 string 值（简化版，不处理嵌套同名 key）
    method get_string(json_str, key) {
        var needle = str.concat("\"", key);
        needle = str.concat(needle, "\"");
        var idx = str.find(json_str, needle);
        if (idx < 0) { return ""; }
        var p = idx + str.len(needle);
        // 跳过 : 和空白
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(json_str, p);
            var skip = 0;
            if (c == 58) { skip = 1; } if (c == 32) { skip = 1; }
            if (c == 10) { skip = 1; } if (c == 13) { skip = 1; }
            if (skip == 1) { p = p + 1; } else { cont = 0; }
        }
        if (str.get_c(json_str, p) != 34) { return ""; }
        p = p + 1; var start = p; cont = 1;
        while (cont == 1) {
            var c = str.get_c(json_str, p);
            if (c == 0) { cont = 0; } else {
                if (c == 34) { cont = 0; } else {
                    if (c == 92) { p = p + 2; } else { p = p + 1; }
                }
            }
        }
        return str.slice(json_str, start, p);
    }

    // 从 JSON 字符串提取某个 key 的 int 值
    method get_int(json_str, key) {
        var s = this.get_string(json_str, key);
        if (str.len(s) > 0) { return atoi(s); }
        // 尝试找数字
        var needle = str.concat("\"", key);
        needle = str.concat(needle, "\"");
        var idx = str.find(json_str, needle);
        if (idx < 0) { return 0; }
        var p = idx + str.len(needle);
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(json_str, p);
            var skip = 0;
            if (c == 58) { skip = 1; } if (c == 32) { skip = 1; }
            if (c == 10) { skip = 1; } if (c == 13) { skip = 1; }
            if (skip == 1) { p = p + 1; } else { cont = 0; }
        }
        // 跳过非数字
        cont = 1;
        while (cont == 1) {
            var c = str.get_c(json_str, p);
            var is_d = 0; if (c >= 48) { if (c <= 57) { is_d = 1; } }
            if (is_d == 1) { cont = 0; } else { p = p + 1; }
        }
        var start = p; cont = 1;
        while (cont == 1) {
            var c = str.get_c(json_str, p);
            var is_d = 0; if (c >= 48) { if (c <= 57) { is_d = 1; } }
            if (is_d == 1) { p = p + 1; } else { cont = 0; }
        }
        return atoi(str.slice(json_str, start, p));
    }

    // 提取 "body":"..." 的值（JSON 转义处理）
    method get_body(json_str) {
        return this.get_string(json_str, "body");
    }

    // 提取 "status":"..." 的值
    method get_status(json_str) {
        return this.get_string(json_str, "status");
    }
}

// ==================== HTML Helper (字符串扫描) ====================
class HtmlHelper {
    // 从 HTML 提取 pageId（WIKIREQUEST.info.pageId = <数字>;）
    method extract_page_id(html) {
        var idx = str.find(html, "pageId");
        if (idx < 0) { return 0; }
        var p = idx + 6;  // len("pageId")
        // 跳过非数字
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            var is_d = 0; if (c >= 48) { if (c <= 57) { is_d = 1; } }
            if (is_d == 1) { cont = 0; } else { p = p + 1; }
            if (p >= str.len(html)) { cont = 0; }
        }
        var start = p; cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            var is_d = 0; if (c >= 48) { if (c <= 57) { is_d = 1; } }
            if (is_d == 1) { p = p + 1; } else { cont = 0; }
        }
        if (p == start) { return 0; }
        return atoi(str.slice(html, start, p));
    }

    // 从页面 HTML 提取标题（<div id="page-title">文本</div>）
    method extract_page_title(html) {
        var idx = str.find(html, "page-title");
        if (idx < 0) { return ""; }
        var p = idx + str.len("page-title");
        // 跳过 > 到下一个 >
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            if (c == 62) { cont = 0; } else { p = p + 1; }
        }
        p = p + 1;  // 跳过 >
        // 去掉前导空白
        cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            var skip = 0; if (c == 32) { skip = 1; } if (c == 10) { skip = 1; } if (c == 13) { skip = 1; } if (c == 9) { skip = 1; }
            if (skip == 1) { p = p + 1; } else { cont = 0; }
        }
        var start = p;
        // 找 </div>
        var close_idx = str.find(html, "</div>");
        if (close_idx < 0) { return str.slice(html, start, str.len(html)); }
        return str.slice(html, start, close_idx);
    }

    // 从 <div class="page-source">提取源码文本</div>
    method extract_page_source(html) {
        var idx = str.find(html, "page-source");
        if (idx < 0) { return ""; }
        var p = idx + str.len("page-source");
        var cont = 1;
        while (cont == 1) {
            var c = str.get_c(html, p);
            if (c == 62) { cont = 0; } else { p = p + 1; }
        }
        p = p + 1; var start = p;
        var close_idx = str.find(html, "</div>");
        if (close_idx < 0) { return str.slice(html, start, str.len(html)); }
        return str.slice(html, start, close_idx);
    }

    // 从页面 HTML 提取 page-tags 中的标签（<a>文本</a>）
    method extract_page_tags(html) {
        var tags = list.new();
        var idx = str.find(html, "page-tags");
        if (idx < 0) { return tags; }
        // 从 idx 开始找 </div> 作为结束
        var end_idx = str.find(html, "</div>");
        if (end_idx < 0) { end_idx = str.len(html); }
        var segment = str.slice(html, idx, end_idx);
        // 在 segment 中找所有 <a ...>文本</a>
        var pos = 0;
        var cont = 1;
        while (cont == 1) {
            var a_idx = str.find(segment, "<a ");
            if (a_idx < 0) { cont = 0; }
            if (a_idx >= 0) {
                // 从 a_idx 开始找 >
                segment = str.slice(segment, a_idx + 3, str.len(segment));
                var gt_idx = str.find(segment, ">");
                if (gt_idx < 0) { cont = 0; }
                if (gt_idx >= 0) {
                    segment = str.slice(segment, gt_idx + 1, str.len(segment));
                    var close_a = str.find(segment, "</a>");
                    if (close_a < 0) { cont = 0; }
                    if (close_a >= 0) {
                        var tag_text = str.slice(segment, 0, close_a);
                        if (str.len(tag_text) > 0) { list.push(tags, tag_text); }
                        segment = str.slice(segment, close_a + 4, str.len(segment));
                    }
                }
            }
        }
        return tags;
    }

    // 从 ListPages HTML 提取页面 fullname 列表（href="/xxx"），去重并过滤非页面链接
    method extract_page_fullnames(html) {
        var names = list.new();
        var rest = html;
        var cont = 1;
        while (cont == 1) {
            var h_idx = str.find(rest, "href=\"/");
            if (h_idx < 0) { cont = 0; }
            if (h_idx >= 0) {
                rest = str.slice(rest, h_idx + 7, str.len(rest));
                var q_idx = str.find(rest, "\"");
                if (q_idx < 0) { cont = 0; }
                if (q_idx >= 0) {
                    var path = str.slice(rest, 0, q_idx);
                    rest = str.slice(rest, q_idx + 1, str.len(rest));
                    var ok = 1;
                    if (str.len(path) == 0) { ok = 0; }
                    if (str.find(path, " ") >= 0) { ok = 0; }
                    if (str.find(path, "?") >= 0) { ok = 0; }
                    if (str.find(path, "#") >= 0) { ok = 0; }
                    if (str.find(path, "javascript") >= 0) { ok = 0; }
                    if (ok == 1) {
                        var dup = 0;
                        var n = list.len(names);
                        var i = 0;
                        while (i < n) {
                            if (str.equal(list.get(names, i), path) == 1) { dup = 1; }
                            i = i + 1;
                        }
                        if (dup == 0) { list.push(names, path); }
                    }
                }
            }
        }
        return names;
    }
}

// ==================== Wikidot Client ====================
class WikidotClient {
    var site : int;       // str_idx: 子域名
    var base_url : int;   // str_idx: https://<site>.wikidot.com
    var token : int;      // str_idx: wikidot_token7
    var logged_in : int;  // 0/1
    var www_token : int;  // str_idx: www.wikidot.com 的 token

    method init(site_name) {
        this.site = site_name;
        this.base_url = str.concat("https://", site_name);
        this.base_url = str.concat(this.base_url, ".wikidot.com");
        this.token = "";
        this.logged_in = 0;
        this.www_token = "";
        http.set_ua("method-wikidot/1.0");
    }

    // 获取站点子域的 wikidot_token7
    method ensure_token() {
        var url1 = str.concat(this.base_url, "/level-1");
        http.request(url1, "GET", "");
        this.token = http.get_cookie("wikidot_token7");
        if (str.len(this.token) > 0) { return 0; }
        http.request(this.base_url, "GET", "");
        this.token = http.get_cookie("wikidot_token7");
        return 0;
    }

    // 获取 www.wikidot.com 的 token（用于私信等账户级操作）
    method ensure_www_token() {
        var www_url = "https://www.wikidot.com";
        http.request(www_url, "GET", "");
        this.www_token = http.get_cookie("wikidot_token7");
        return 0;
    }

    // ===== Login =====
    // POST www.wikidot.com/default--flow/login__LoginPopupScreen
    // login=<user>&password=<pass>&action=Login2Action&event=login&wikidot_token7=<token>
    method login(username, password) {
        this.ensure_www_token();
        var amp = "&";
        var u = str.concat("login=", username);
        var p = str.concat(amp, "password=");
        p = str.concat(p, password);
        var a = str.concat(amp, "action=Login2Action");
        a = str.concat(a, amp);
        a = str.concat(a, "event=login");
        var t = str.concat(amp, "wikidot_token7=");
        t = str.concat(t, this.www_token);
        var body = str.concat(u, p);
        body = str.concat(body, a);
        body = str.concat(body, t);
        var login_url = "https://www.wikidot.com/default--flow/login__LoginPopupScreen";
        http.request(login_url, "POST", body);
        var session = http.get_cookie("WIKIDOT_SESSION_ID");
        if (str.len(session) > 0) {
            this.logged_in = 1;
            return 0;
        }
        return -1;
    }

    // ===== 核心 HTTP 请求 =====
    // POST AJAX connector 返回原始 body
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

    // POST 到 www.wikidot.com 的 AJAX connector（账户级操作）
    method call_www_connector_raw(params) {
        var amp = "&";
        var t7 = "wikidot_token7=";
        var full = str.concat(params, amp);
        full = str.concat(full, t7);
        full = str.concat(full, this.www_token);
        var url = "https://www.wikidot.com/ajax-module-connector.php";
        var resp = http.request(url, "POST", full);
        return list.get(resp, 0);
    }

    // POST AJAX 返回解析后的 dict
    method call_module(module_name, params) {
        var amp = "&";
        var mn = "moduleName=";
        var full = str.concat(mn, module_name);
        full = str.concat(full, amp);
        full = str.concat(full, params);
        var raw = this.call_connector_raw(full);
        var parser = new JsonParser();
        return parser.parse(raw);
    }

    // POST AJAX 返回 body HTML 字符串
    method call_module_body(module_name, params) {
        var amp = "&";
        var mn = "moduleName=";
        var full = str.concat(mn, module_name);
        full = str.concat(full, amp);
        full = str.concat(full, params);
        var raw = this.call_connector_raw(full);
        var parser = new JsonParser();
        return parser.get_body(raw);
    }

    // 调用 Action（action=X&event=Y，moduleName 为空）
    method call_action(action_name, event_name, params) {
        var amp = "&";
        var a = str.concat("action=", action_name);
        a = str.concat(a, amp);
        var e = str.concat("event=", event_name);
        var full = str.concat(a, e);
        full = str.concat(full, amp);
        full = str.concat(full, params);
        var raw = this.call_connector_raw(full);
        var parser = new JsonParser();
        return parser.parse(raw);
    }

    // 调用 www 子域的 Action（私信等）
    method call_www_action(action_name, event_name, params) {
        var amp = "&";
        var a = str.concat("action=", action_name);
        a = str.concat(a, amp);
        var e = str.concat("event=", event_name);
        var full = str.concat(a, e);
        full = str.concat(full, amp);
        full = str.concat(full, params);
        var raw = this.call_www_connector_raw(full);
        var parser = new JsonParser();
        return parser.parse(raw);
    }

    // 调用 www 子域的 module（收件箱等）
    method call_www_module(module_name, params) {
        var amp = "&";
        var mn = "moduleName=";
        var full = str.concat(mn, module_name);
        full = str.concat(full, amp);
        full = str.concat(full, params);
        var raw = this.call_www_connector_raw(full);
        var parser = new JsonParser();
        return parser.parse(raw);
    }

    // ===== Page API =====
    // GetPageID: 从 HTML 提取 pageId
    method get_page_id(fullname) {
        var path = str.concat("/", fullname);
        var url = str.concat(this.base_url, path);
        var resp = http.request(url, "GET", "");
        var html = list.get(resp, 0);
        var helper = new HtmlHelper();
        return helper.extract_page_id(html);
    }

    // GetPageSource: viewsource/ViewSourceModule → page-source div
    method get_page_source(fullname) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return ""; }
        var amp = "&";
        var pid = str.concat("page_id=", itoa(page_id));
        var body_html = this.call_module_body("viewsource/ViewSourceModule", pid);
        var helper = new HtmlHelper();
        return helper.extract_page_source(body_html);
    }

    // GetPageHTML: GET 页面原始 HTML
    method get_page_html(fullname) {
        var path = str.concat("/", fullname);
        var url = str.concat(this.base_url, path);
        var resp = http.request(url, "GET", "");
        return list.get(resp, 0);
    }

    // GetPageTags: 从 HTML 提取 page-tags
    method get_page_tags(fullname) {
        var html = this.get_page_html(fullname);
        var helper = new HtmlHelper();
        return helper.extract_page_tags(html);
    }

    // SetPageTags: WikiPageAction/saveTags
    method set_page_tags(fullname, tags_str) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return -1; }
        var amp = "&";
        var pid = str.concat("pageId=", itoa(page_id));
        var tg = str.concat(amp, "tags=");
        tg = str.concat(tg, tags_str);
        var params = str.concat(pid, tg);
        this.call_action("WikiPageAction", "saveTags", params);
        return 0;
    }

    // ===== 批量标签 API =====
    // "a b c" → list
    method split_tags(tags_str) {
        var res = list.new();
        var rest = str.trim(tags_str);
        var cont = 1;
        while (cont == 1) {
            if (str.len(rest) == 0) { cont = 0; }
            if (str.len(rest) > 0) {
                var sp = str.find(rest, " ");
                if (sp < 0) {
                    list.push(res, rest);
                    cont = 0;
                }
                if (sp >= 0) {
                    var word = str.trim(str.slice(rest, 0, sp));
                    if (str.len(word) > 0) { list.push(res, word); }
                    rest = str.slice(rest, sp + 1, str.len(rest));
                }
            }
        }
        return res;
    }

    // list → "a b c"
    method join_tags(tags) {
        var out = "";
        var n = list.len(tags);
        var i = 0;
        while (i < n) {
            if (i > 0) { out = str.concat(out, " "); }
            out = str.concat(out, list.get(tags, i));
            i = i + 1;
        }
        return out;
    }

    // list 是否包含某值（str 内容比较）
    method list_contains(lst, val) {
        var n = list.len(lst);
        var i = 0;
        while (i < n) {
            if (str.equal(list.get(lst, i), val) == 1) { return 1; }
            i = i + 1;
        }
        return 0;
    }

    // ListPages 收集 fullname 列表（按 category/tags 筛选）
    method collect_page_fullnames(category, tags, per_page) {
        var html = this.list_pages(category, tags, per_page);
        var helper = new HtmlHelper();
        return helper.extract_page_fullnames(html);
    }

    // 批量覆盖设置标签（每组整体替换），返回成功数
    method batch_set_tags(fullnames, tags_str) {
        var ok = 0;
        var n = list.len(fullnames);
        var i = 0;
        while (i < n) {
            var name = list.get(fullnames, i);
            var r = this.set_page_tags(name, tags_str);
            if (r == 0) { ok = ok + 1; }
            if (i + 1 < n) { sleep(300); }
            i = i + 1;
        }
        return ok;
    }

    // 批量追加标签（保留原有，自动去重），返回成功数
    method batch_add_tags(fullnames, add_tags_str) {
        var ok = 0;
        var n = list.len(fullnames);
        var i = 0;
        while (i < n) {
            var name = list.get(fullnames, i);
            var cur = this.get_page_tags(name);
            var merged = this.merge_tag_list(cur, add_tags_str);
            var r = this.set_page_tags(name, this.join_tags(merged));
            if (r == 0) { ok = ok + 1; }
            if (i + 1 < n) { sleep(300); }
            i = i + 1;
        }
        return ok;
    }

    // 批量移除标签，返回成功数
    method batch_remove_tags(fullnames, remove_tags_str) {
        var ok = 0;
        var n = list.len(fullnames);
        var i = 0;
        while (i < n) {
            var name = list.get(fullnames, i);
            var cur = this.get_page_tags(name);
            var kept = this.filter_out_tags(cur, remove_tags_str);
            var r = this.set_page_tags(name, this.join_tags(kept));
            if (r == 0) { ok = ok + 1; }
            if (i + 1 < n) { sleep(300); }
            i = i + 1;
        }
        return ok;
    }

    // 合并标签（复制 cur 后追加 add_tags_str 中不存在的标签）
    method merge_tag_list(cur, add_tags_str) {
        var res = list.new();
        var n = list.len(cur);
        var i = 0;
        while (i < n) { list.push(res, list.get(cur, i)); i = i + 1; }
        var adds = this.split_tags(add_tags_str);
        var m = list.len(adds);
        var j = 0;
        while (j < m) {
            var t = list.get(adds, j);
            if (this.list_contains(res, t) == 0) { list.push(res, t); }
            j = j + 1;
        }
        return res;
    }

    // 从标签列表中剔除 remove_tags_str 里包含的标签
    method filter_out_tags(cur, remove_tags_str) {
        var res = list.new();
        var rms = this.split_tags(remove_tags_str);
        var n = list.len(cur);
        var i = 0;
        while (i < n) {
            var t = list.get(cur, i);
            if (this.list_contains(rms, t) == 0) { list.push(res, t); }
            i = i + 1;
        }
        return res;
    }

    // ===== Edit API =====
    // 获取编辑锁: edit/PageEditModule → 返回 dict(lock_id, lock_secret, revision_id)
    method acquire_edit_lock(fullname, page_id) {
        var amp = "&";
        var mode = "mode=page";
        var wp = str.concat(amp, "wiki_page=");
        wp = str.concat(wp, fullname);
        var params = str.concat(mode, wp);
        if (page_id > 0) {
            var pid = str.concat(amp, "page_id=");
            pid = str.concat(pid, itoa(page_id));
            params = str.concat(params, pid);
        }
        var raw = this.call_connector_raw(str.concat(str.concat("moduleName=edit/PageEditModule", amp), params));
        var parser = new JsonParser();
        var lock_id = parser.get_int(raw, "lock_id");
        var lock_secret = parser.get_string(raw, "lock_secret");
        var rev_id = parser.get_int(raw, "page_revision_id");
        var lock = list.new();
        list.push(lock, lock_id);
        list.push(lock, lock_secret);
        list.push(lock, rev_id);
        return lock;
    }

    // 释放编辑锁: WikiPageAction/removePageEditLock
    method release_edit_lock(fullname, page_id, lock_id, lock_secret) {
        var amp = "&";
        var wp = str.concat("wiki_page=", fullname);
        var lid = str.concat(amp, "lock_id=");
        lid = str.concat(lid, itoa(lock_id));
        var lsec = str.concat(amp, "lock_secret=");
        lsec = str.concat(lsec, lock_secret);
        var ld = str.concat(amp, "leave_draft=false");
        var params = str.concat(wp, lid);
        params = str.concat(params, lsec);
        params = str.concat(params, ld);
        if (page_id > 0) {
            var pid = str.concat(amp, "page_id=");
            pid = str.concat(pid, itoa(page_id));
            params = str.concat(params, pid);
        }
        this.call_action("WikiPageAction", "removePageEditLock", params);
        return 0;
    }

    // CreatePage: edit/PageEditModule(无page_id) → WikiPageAction/savePage(无page_id)
    method create_page(fullname, title, content, tags_str, comment) {
        // 1) 取编辑锁
        var lock = this.acquire_edit_lock(fullname, 0);
        var lock_id = list.get(lock, 0);
        var lock_secret = list.get(lock, 1);

        // 2) savePage
        var amp = "&";
        var mode = "mode=page";
        var wp = str.concat(amp, "wiki_page=");
        wp = str.concat(wp, fullname);
        var lid = str.concat(amp, "lock_id=");
        lid = str.concat(lid, itoa(lock_id));
        var lsec = str.concat(amp, "lock_secret=");
        lsec = str.concat(lsec, lock_secret);
        var t = str.concat(amp, "title=");
        t = str.concat(t, title);
        var s = str.concat(amp, "source=");
        s = str.concat(s, content);
        var params = str.concat(mode, wp);
        params = str.concat(params, lid);
        params = str.concat(params, lsec);
        params = str.concat(params, t);
        params = str.concat(params, s);
        if (str.len(comment) > 0) {
            var c = str.concat(amp, "comment=");
            c = str.concat(c, comment);
            params = str.concat(params, c);
        }
        if (str.len(tags_str) > 0) {
            var tg = str.concat(amp, "tags=");
            tg = str.concat(tg, tags_str);
            params = str.concat(params, tg);
        }
        this.call_action("WikiPageAction", "savePage", params);
        return 0;
    }

    // EditPage: edit/PageEditModule(带page_id) → WikiPageAction/savePage(带page_id)
    method edit_page(fullname, title, content, tags_str, comment) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return -1; }

        // 取标题（空则用原标题）
        var use_title = title;
        if (str.len(use_title) == 0) {
            var html = this.get_page_html(fullname);
            var helper = new HtmlHelper();
            use_title = helper.extract_page_title(html);
        }
        // 取源码（空则用原源码）
        var use_source = content;
        if (str.len(use_source) == 0) {
            use_source = this.get_page_source(fullname);
        }

        // 取编辑锁
        var lock = this.acquire_edit_lock(fullname, page_id);
        var lock_id = list.get(lock, 0);
        var lock_secret = list.get(lock, 1);
        var rev_id = list.get(lock, 2);

        var amp = "&";
        var pid = str.concat("page_id=", itoa(page_id));
        var mode = str.concat(amp, "mode=page");
        var wp = str.concat(amp, "wiki_page=");
        wp = str.concat(wp, fullname);
        var lid = str.concat(amp, "lock_id=");
        lid = str.concat(lid, itoa(lock_id));
        var lsec = str.concat(amp, "lock_secret=");
        lsec = str.concat(lsec, lock_secret);
        var rid = str.concat(amp, "revision_id=");
        rid = str.concat(rid, itoa(rev_id));
        var t = str.concat(amp, "title=");
        t = str.concat(t, use_title);
        var s = str.concat(amp, "source=");
        s = str.concat(s, use_source);
        var c = str.concat(amp, "comment=");
        c = str.concat(c, comment);
        var params = str.concat(pid, mode);
        params = str.concat(params, wp);
        params = str.concat(params, lid);
        params = str.concat(params, lsec);
        params = str.concat(params, rid);
        params = str.concat(params, t);
        params = str.concat(params, s);
        params = str.concat(params, c);
        if (str.len(tags_str) > 0) {
            var tg = str.concat(amp, "tags=");
            tg = str.concat(tg, tags_str);
            params = str.concat(params, tg);
        }
        this.call_action("WikiPageAction", "savePage", params);
        return 0;
    }

    // ===== Rename / Delete =====
    // RenamePage: WikiPageAction/renamePage
    method rename_page(fullname, new_name) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return -1; }
        var amp = "&";
        var pid = str.concat("page_id=", itoa(page_id));
        var nn = str.concat(amp, "new_name=");
        nn = str.concat(nn, new_name);
        var params = str.concat(pid, nn);
        this.call_action("WikiPageAction", "renamePage", params);
        return 0;
    }

    // DeletePage: WikiPageAction/deletePage
    method delete_page(fullname) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return -1; }
        var pid = str.concat("page_id=", itoa(page_id));
        this.call_action("WikiPageAction", "deletePage", pid);
        return 0;
    }

    // ===== History =====
    // GetPageHistory: history/PageRevisionListModule
    method get_page_history(fullname) {
        var page_id = this.get_page_id(fullname);
        if (page_id == 0) { return ""; }
        var amp = "&";
        var pid = str.concat("page_id=", itoa(page_id));
        var pg = str.concat(amp, "page=1");
        var pp = str.concat(amp, "perpage=20");
        var opt = str.concat(amp, "options=");
        opt = str.concat(opt, "%7B%22all%22:true%7D");
        var params = str.concat(pid, pg);
        params = str.concat(params, pp);
        params = str.concat(params, opt);
        return this.call_module_body("history/PageRevisionListModule", params);
    }

    // GetPageRevisionSource: history/PageSourceModule
    method get_page_revision_source(revision_id) {
        var rid = str.concat("revision_id=", itoa(revision_id));
        var body_html = this.call_module_body("history/PageSourceModule", rid);
        var helper = new HtmlHelper();
        return helper.extract_page_source(body_html);
    }

    // ===== ListPages =====
    // list/ListPagesModule → HTML 片段
    method list_pages(category, tags, per_page) {
        var amp = "&";
        var cat = str.concat("category=", category);
        var tg = str.concat(amp, "tags=");
        tg = str.concat(tg, tags);
        var pp = str.concat(amp, "perPage=");
        pp = str.concat(pp, itoa(per_page));
        var ord = str.concat(amp, "order=created_at desc");
        var sep = str.concat(amp, "separate=no");
        var wr = str.concat(amp, "wrapper=no");
        var params = str.concat(cat, tg);
        params = str.concat(params, pp);
        params = str.concat(params, ord);
        params = str.concat(params, sep);
        params = str.concat(params, wr);
        return this.call_module_body("list/ListPagesModule", params);
    }

    // ===== Forum API =====
    // GetForumCategories: forum/ForumStartModule
    method get_forum_categories() {
        return this.call_module_body("forum/ForumStartModule", "");
    }

    // CreateForumThread: ForumAction/newThread (moduleName=Empty)
    method create_forum_thread(category_id, title, content) {
        var amp = "&";
        var cid = str.concat("category_id=", itoa(category_id));
        var t = str.concat(amp, "title=");
        t = str.concat(t, title);
        var s = str.concat(amp, "source=");
        s = str.concat(s, content);
        var a = str.concat(amp, "action=ForumAction");
        var e = str.concat(amp, "event=newThread");
        var params = str.concat(cid, t);
        params = str.concat(params, s);
        params = str.concat(params, a);
        params = str.concat(params, e);
        return this.call_module_body("Empty", params);
    }

    // CreateForumPost: ForumAction/savePost
    method create_forum_post(thread_id, content) {
        var amp = "&";
        var tid = str.concat("threadId=", itoa(thread_id));
        var s = str.concat(amp, "source=");
        s = str.concat(s, content);
        var params = str.concat(tid, s);
        return this.call_action("ForumAction", "savePost", params);
    }

    // GetForumThread: forum/ForumViewThreadModule
    method get_forum_thread(thread_id) {
        var t = str.concat("t=", itoa(thread_id));
        return this.call_module_body("forum/ForumViewThreadModule", t);
    }

    // GetForumThreadPosts: forum/ForumViewThreadPostsModule
    method get_forum_thread_posts(thread_id, page_no) {
        var amp = "&";
        var t = str.concat("t=", itoa(thread_id));
        var pg = str.concat(amp, "pageNo=");
        pg = str.concat(pg, itoa(page_no));
        var params = str.concat(t, pg);
        return this.call_module_body("forum/ForumViewThreadPostsModule", params);
    }

    // ===== Mail API =====
    // lookupUserID: GET /quickmodule.php?module=UserLookupQModule&q=<username>
    method lookup_user_id(username) {
        var url1 = str.concat(this.base_url, "/quickmodule.php?module=UserLookupQModule&q=");
        url1 = str.concat(url1, username);
        var resp = http.request(url1, "GET", "");
        var raw = list.get(resp, 0);
        var parser = new JsonParser();
        return parser.get_int(raw, "user_id");
    }

    // SendMail: DashboardMessageAction/send (www 子域)
    method send_mail(username, subject, content) {
        this.ensure_www_token();
        var user_id = this.lookup_user_id(username);
        if (user_id == 0) { return -1; }
        var amp = "&";
        var uid = str.concat("to_user_id=", itoa(user_id));
        var sub = str.concat(amp, "subject=");
        sub = str.concat(sub, subject);
        var src = str.concat(amp, "source=");
        src = str.concat(src, content);
        var params = str.concat(uid, sub);
        params = str.concat(params, src);
        this.call_www_action("DashboardMessageAction", "send", params);
        return 0;
    }

    // GetInboxMessages: dashboard/messages/DMInboxModule (www 子域)
    method get_inbox_messages(page) {
        var pg = str.concat("page=", itoa(page));
        return this.call_www_module("dashboard/messages/DMInboxModule", pg);
    }

    // DeleteMail: DashboardMessageAction/delete (www 子域)
    method delete_mail(message_id) {
        var amp = "&";
        var mid = str.concat("message_id=", itoa(message_id));
        var params = str.concat(mid, amp);
        this.call_www_action("DashboardMessageAction", "delete", params);
        return 0;
    }

    // ===== User Page =====
    // getUserID: GET www.wikidot.com/user:info/<username>
    method get_user_id(username) {
        var url = str.concat("https://www.wikidot.com/user:info/", username);
        var resp = http.request(url, "GET", "");
        var html = list.get(resp, 0);
        var helper = new HtmlHelper();
        return helper.extract_page_id(html);
    }
}

// ==================== Main Demo ====================
system.print_str("============================================\n");
system.print_str("  Wikidot-Golang (method) 完整重写\n");
system.print_str("  https://github.com/umouyoumou-del/Wikidot-Golang\n");
system.print_str("============================================\n\n");

var client = new WikidotClient("backrooms-wiki-cn");
client.ensure_token();

// --- Token ---
system.print_str("[1] Token 获取\n");
system.print_str("  wikidot_token7 len: ");
system.print(str.len(client.token));
system.print_char(10);
if (str.len(client.token) == 0) {
    system.print_str("  ERROR: 无法获取 token!\n");
    return 0;
}
system.print_str("  OK\n\n");

// --- GetPageID ---
system.print_str("[2] GetPageID('start')\n");
var pid = client.get_page_id("start");
system.print_str("  Page ID: ");
system.print(pid);
system.print_char(10);
system.print_str("  OK\n\n");

// --- GetPageSource ---
system.print_str("[3] GetPageSource('start')\n");
var src = client.get_page_source("start");
system.print_str("  Source len: ");
system.print(str.len(src));
system.print_char(10);
if (str.len(src) > 0) {
    system.print_str("  Preview:\n  ");
    var preview = str.slice(src, 0, 200);
    system.print_str(preview);
    system.print_char(10);
}
system.print_str("  OK\n\n");

// --- GetPageTags ---
system.print_str("[4] GetPageTags('start')\n");
var tags = client.get_page_tags("start");
system.print_str("  Tags count: ");
system.print(list.len(tags));
system.print_char(10);
system.print_str("  OK\n\n");

// --- ListPages ---
system.print_str("[5] ListPages(category='', tags='', perPage=5)\n");
var pages_html = client.list_pages("", "", 5);
system.print_str("  HTML len: ");
system.print(str.len(pages_html));
system.print_char(10);
system.print_str("  OK\n\n");

// --- GetForumCategories ---
system.print_str("[6] GetForumCategories()\n");
var forum_html = client.get_forum_categories();
system.print_str("  HTML len: ");
system.print(str.len(forum_html));
system.print_char(10);
system.print_str("  OK\n\n");

// --- GetPageHistory ---
system.print_str("[7] GetPageHistory('start')\n");
var hist_html = client.get_page_history("start");
system.print_str("  History HTML len: ");
system.print(str.len(hist_html));
system.print_char(10);
system.print_str("  OK\n\n");

// --- API 汇总 ---
system.print_str("============================================\n");
system.print_str("  已实现 API 列表\n");
system.print_str("============================================\n");
system.print_str("  [Core]\n");
system.print_str("    Login(username, password)\n");
system.print_str("    ensure_token() / ensure_www_token()\n");
system.print_str("    call_module / call_action / call_www_action\n");
system.print_str("  [Page]\n");
system.print_str("    GetPageID / GetPageSource / GetPageHTML\n");
system.print_str("    GetPageTags / SetPageTags\n");
system.print_str("    BatchTags: collect_page_fullnames\n");
system.print_str("               batch_set_tags / batch_add_tags / batch_remove_tags\n");
system.print_str("    ListPages / GetPageHistory\n");
system.print_str("    GetPageRevisionSource\n");
system.print_str("  [Edit]\n");
system.print_str("    CreatePage / EditPage\n");
system.print_str("    AcquireEditLock / ReleaseEditLock\n");
system.print_str("  [Rename/Delete]\n");
system.print_str("    RenamePage / DeletePage\n");
system.print_str("  [Forum]\n");
system.print_str("    GetForumCategories\n");
system.print_str("    CreateForumThread / CreateForumPost\n");
system.print_str("    GetForumThread / GetForumThreadPosts\n");
system.print_str("  [Mail]\n");
system.print_str("    SendMail / GetInboxMessages / DeleteMail\n");
system.print_str("    LookupUserID\n");
system.print_str("  [User]\n");
system.print_str("    GetUserID\n");
system.print_str("============================================\n");
system.print_str("  演示完成\n");

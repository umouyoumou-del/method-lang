// ==================================================================
// delete.mt —— 批量删除 Wikidot 站内私信
//
// 用法：编辑同目录下 config.json 填入账户信息后运行
//   methodc lang\delete-mail\delete.mt
//
// 流程：读取 config.json → 登录 → try 循环拉取收件箱(DMInboxModule)
//       → 提取消息 ID → 逐封 try 执行 delete_mail(message_id)，
//       直到删够指定数量
// ==================================================================
import wikidot;

// ==================== MailDeleter ====================
class MailDeleter {
    var client;      // WikidotClient
    var target;      // 目标删除数
    var deleted;     // 已删除数
    var seen;        // dict: 已处理过的 message_id（去重）
    var max_pages;   // 收件箱翻页安全上限

    method init(site, count) {
        this.client = new WikidotClient(site);
        this.target = count;
        this.deleted = 0;
        this.seen = dict.new();
        this.max_pages = 50;
    }

    // 登录并获取 www 域 token（私信接口需要）
    method connect(user, password) {
        this.client.ensure_www_token();
        var r = this.client.login(user, password);
        if (this.client.logged_in != 1) {
            raise "login failed";
        }
        return r;
    }

    // 从收件箱模块 HTML 中提取消息 ID（匹配 message-row-<id> / message_row_<id>）
    method extract_ids(html, marker, out) {
        var seg = html;
        var scanning = 1;
        while (scanning == 1) {
            var idx = str.find(seg, marker);
            if (idx < 0) {
                scanning = 0;
            }
            if (idx >= 0) {
                var p = idx + str.len(marker);
                var start = p;
                var collecting = 1;
                while (collecting == 1) {
                    var c = str.get_c(seg, p);
                    var is_d = 0;
                    if (c >= 48) { if (c <= 57) { is_d = 1; } }
                    if (is_d == 1) { p = p + 1; } else { collecting = 0; }
                }
                if (p > start) {
                    var idv = atoi(str.slice(seg, start, p));
                    if (idv > 0) { list.push(out, idv); }
                }
                seg = str.slice(seg, p, str.len(seg));
            }
        }
        return out;
    }

    // 删除单封邮件：成功返回 1，失败（抛异常/状态非 ok）返回 0
    method delete_one(message_id) {
        var key = itoa(message_id);
        var resp = this.client.delete_mail(message_id);
        var st = dict.get(resp, "status");
        if (str.equal(st, "ok") != 1) {
            raise str.concat("delete ", str.concat(key, str.concat(" bad status: ", st)));
        }
        dict.put(this.seen, key, 1);
        this.deleted = this.deleted + 1;
        system.print_str("  [OK] deleted message ");
        system.print_str(key);
        system.print_str("  (");
        system.print(this.deleted);
        system.print_str("/");
        system.print(this.target);
        system.print_str(")\n");
        return 1;
    }

    // 主流程：try 循环，直到删够 target 封
    method run() {
        var page = 1;
        var pages_done = 0;
        var running = 1;
        while (running == 1) {
            // 终止条件
            if (this.deleted >= this.target) { running = 0; }
            if (pages_done >= this.max_pages) { running = 0; }

            if (running == 1) {
                var ids = list.new();
                var fetch_ok = 1;
                var page_empty = 0;

                // ---- 拉取一页收件箱（失败可捕获）----
                try {
                    var result = this.client.get_inbox_messages(page);
                    var st = dict.get(result, "status");
                    if (str.equal(st, "ok") != 1) {
                        raise str.concat("inbox page ", str.concat(itoa(page), " bad status"));
                    }
                    var body = dict.get(result, "body");
                    if (str.len(body) == 0) {
                        page_empty = 1;
                    }
                    if (page_empty == 0) {
                        this.extract_ids(body, "message-row-", ids);
                        this.extract_ids(body, "message_row_", ids);
                    }
                } except as e {
                    fetch_ok = 0;
                    system.print_str("  [ERR] fetch page ");
                    system.print(page);
                    system.print_str(": ");
                    system.print_str(e);
                    system.print_char(10);
                }

                if (fetch_ok == 0) {
                    // 拉取失败：终止（避免死循环）
                    running = 0;
                }
                if (page_empty == 1) {
                    system.print_str("  inbox empty at page ");
                    system.print(page);
                    system.print_char(10);
                    running = 0;
                }

                if (running == 1) {
                    // ---- 逐封删除（每封独立 try，失败跳过继续）----
                    var n = list.len(ids);
                    var j = 0;
                    var more = 1;
                    while (more == 1) {
                        if (j >= n) { more = 0; }
                        if (this.deleted >= this.target) { more = 0; }
                        if (more == 1) {
                            var idv = list.get(ids, j);
                            var key = itoa(idv);
                            var dup = dict.has(this.seen, key);
                            if (dup == 0) {
                                try {
                                    this.delete_one(idv);
                                } except as e {
                                    system.print_str("  [SKIP] message ");
                                    system.print_str(key);
                                    system.print_str(": ");
                                    system.print_str(e);
                                    system.print_char(10);
                                }
                            }
                            j = j + 1;
                        }
                    }
                    pages_done = pages_done + 1;
                    page = page + 1;
                }
            }
        }
        return this.deleted;
    }
}

// ==================== 读取配置 ====================
// config.json 与本脚本同目录：
// { "site": "...", "user": "...", "password": "...", "count": 10 }
// 依次尝试运行目录和脚本相对路径，两处都找不到才报错
var cfg = dict.new();
var load_ok = 1;
var content = "";
var last_err = "";
var found = 0;
try {
    var paths = list.new();
    list.push(paths, "config.json");
    list.push(paths, "lang/delete-mail/config.json");
    var i = 0;
    var scanning = 1;
    while (scanning == 1) {
        if (i >= list.len(paths)) { scanning = 0; }
        if (scanning == 1) {
            var p = list.get(paths, i);
            var rf = system.read_file(p);
            if (list.get(rf, 1) == 1) {
                content = list.get(rf, 0);
                found = 1;
                scanning = 0;
            } else {
                last_err = list.get(rf, 0);
            }
            i = i + 1;
        }
    }
    if (found != 1) {
        raise str.concat("config.json not found: ", last_err);
    }
    var parser = new JsonParser();
    cfg = parser.parse(content);
} except as e {
    load_ok = 0;
    system.print_str("FATAL: load config failed: ");
    system.print_str(e);
    system.print_char(10);
}

var SITE = "";
var USER = "";
var PASS = "";
var DELETE_COUNT = 0;
if (load_ok == 1) {
    SITE = dict.get(cfg, "site");
    USER = dict.get(cfg, "user");
    PASS = dict.get(cfg, "password");
    DELETE_COUNT = dict.get(cfg, "count");
    if (str.len(SITE) == 0) { load_ok = 0; system.print_str("FATAL: config.json missing \"site\"\n"); }
    if (str.len(USER) == 0) { load_ok = 0; system.print_str("FATAL: config.json missing \"user\"\n"); }
    if (str.len(PASS) == 0) { load_ok = 0; system.print_str("FATAL: config.json missing \"password\"\n"); }
    if (DELETE_COUNT <= 0) { load_ok = 0; system.print_str("FATAL: config.json \"count\" must be > 0\n"); }
}

// ==================== Main ====================
var deleter = new MailDeleter(SITE, DELETE_COUNT);
var total = 0;
if (load_ok == 1) {
    try {
        deleter.connect(USER, PASS);
        system.print_str("== login ok, target: ");
        system.print(DELETE_COUNT);
        system.print_str(" mail(s) ==\n");
        total = deleter.run();
    } except as e {
        system.print_str("FATAL: ");
        system.print_str(e);
        system.print_char(10);
    } finally {
        system.print_str("== done: deleted ");
        system.print(total);
        system.print_str(" mail(s) ==\n");
    }
}


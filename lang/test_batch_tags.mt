// ==================== 批量标签 API 单元测试（不触网） ====================
import Wikidot;

// 返回 1=通过 0=失败（str 内容比较）
method expect_eq_s(actual, expected, name) {
    if (str.equal(actual, expected) == 1) {
        system.print_str("  [PASS] ");
        system.print_str(name);
        system.print_char(10);
        return 1;
    }
    system.print_str("  [FAIL] ");
    system.print_str(name);
    system.print_str("  expected=");
    system.print_str(expected);
    system.print_str(" actual=");
    system.print_str(actual);
    system.print_char(10);
    return 0;
}

// 返回 1=通过 0=失败（int 比较）
method expect_eq_i(actual, expected, name) {
    if (actual == expected) {
        system.print_str("  [PASS] ");
        system.print_str(name);
        system.print_char(10);
        return 1;
    }
    system.print_str("  [FAIL] ");
    system.print_str(name);
    system.print_str("  expected=");
    system.print(expected);
    system.print_str(" actual=");
    system.print(actual);
    system.print_char(10);
    return 0;
}

var passed = 0;
var failed = 0;
var r = 0;

var client = new WikidotClient("test-site");
var helper = new HtmlHelper();

// ---- 1. extract_page_fullnames ----
system.print_str("[1] extract_page_fullnames\n");
var html = str.concat("<div><a href=\"/page-one\">P1</a></div>", "<div><a href=\"/page-two\">P2</a></div>");
html = str.concat(html, "<div><a href=\"/page-one\">dup</a></div>");
html = str.concat(html, "<div><a href=\"/forum:start\">forum</a></div>");
html = str.concat(html, "<div><a href=\"/search?q=x\">search</a></div>");
html = str.concat(html, "<div><a href=\"javascript:void(0)\">js</a></div>");
html = str.concat(html, "<div><a href=\"https://example.com/external\">ext</a></div>");

var names = helper.extract_page_fullnames(html);
passed = passed + expect_eq_i(list.len(names), 3, "extract count (dup/ext/js/search filtered)");
if (list.len(names) > 2) {
    var n0 = list.get(names, 0);
    var n1 = list.get(names, 1);
    var n2 = list.get(names, 2);
    passed = passed + expect_eq_s(n0, "page-one", "first fullname");
    passed = passed + expect_eq_s(n1, "page-two", "second fullname");
    passed = passed + expect_eq_s(n2, "forum:start", "colon fullname kept");
}
if (list.len(names) <= 2) { failed = failed + 1; system.print_str("  [FAIL] names too few, skip item checks\n"); }

var empty_names = helper.extract_page_fullnames("");
passed = passed + expect_eq_i(list.len(empty_names), 0, "empty html -> 0 names");

// ---- 2. split_tags / join_tags ----
system.print_str("[2] split_tags / join_tags\n");
var t1 = client.split_tags("a b c");
passed = passed + expect_eq_i(list.len(t1), 3, "split 3 tags");
var t1_0 = list.get(t1, 0);
var t1_2 = list.get(t1, 2);
passed = passed + expect_eq_s(t1_0, "a", "split[0]=a");
passed = passed + expect_eq_s(t1_2, "c", "split[2]=c");

var t2 = client.split_tags("  alpha   beta  ");
passed = passed + expect_eq_i(list.len(t2), 2, "split trims extra spaces");

var t3 = client.split_tags("single");
passed = passed + expect_eq_i(list.len(t3), 1, "split single");
var j3 = client.join_tags(t3);
passed = passed + expect_eq_s(j3, "single", "join single");

var t4 = client.split_tags("");
passed = passed + expect_eq_i(list.len(t4), 0, "split empty");

var jt1 = client.join_tags(t1);
passed = passed + expect_eq_s(jt1, "a b c", "join round-trip");
var empty_list = list.new();
var je = client.join_tags(empty_list);
passed = passed + expect_eq_s(je, "", "join empty list");

// ---- 3. list_contains ----
system.print_str("[3] list_contains\n");
r = client.list_contains(t1, "b");
passed = passed + expect_eq_i(r, 1, "contains b -> 1");
r = client.list_contains(t1, "z");
passed = passed + expect_eq_i(r, 0, "contains z -> 0");

// ---- 4. merge_tag_list ----
system.print_str("[4] merge_tag_list\n");
var cur = client.split_tags("test meta");
var merged = client.merge_tag_list(cur, "new1 new2");
passed = passed + expect_eq_i(list.len(merged), 4, "merge adds 2 new");
r = client.list_contains(merged, "new1");
passed = passed + expect_eq_i(r, 1, "merged has new1");

var merged_dup = client.merge_tag_list(cur, "meta extra");
passed = passed + expect_eq_i(list.len(merged_dup), 3, "merge dedups existing tag");
r = client.list_contains(merged_dup, "meta");
passed = passed + expect_eq_i(r, 1, "original tag kept");

// ---- 5. filter_out_tags ----
system.print_str("[5] filter_out_tags\n");
var cur2 = client.split_tags("a b c d");
var kept = client.filter_out_tags(cur2, "b d");
passed = passed + expect_eq_i(list.len(kept), 2, "filter removes 2");
var kept_j = client.join_tags(kept);
passed = passed + expect_eq_s(kept_j, "a c", "filter keeps a c");

var kept_none = client.filter_out_tags(cur2, "x y");
passed = passed + expect_eq_i(list.len(kept_none), 4, "filter with no match keeps all");

// ---- 汇总 ----
system.print_str("\n============================================\n");
system.print_str("  passed=");
system.print(passed);
system.print_str("  failed=");
system.print(failed);
system.print_char(10);
if (failed == 0) {
    system.print_str("  ALL TESTS PASSED\n");
}
if (failed > 0) {
    system.print_str("  SOME TESTS FAILED\n");
    return 1;
}
return 0;

/*
 * difficulty-rules.selftest.c —— 装载期外的**纯逻辑自测**（不进游戏、不碰客户端内存）
 *
 * 目的：把从 Go 侧逐字搬过来的算法（属性加解密、原生 float/整数解码、
 * scaleHP、maxima、rescaleEffectiveHP、rules.json 解析、默认规则文件内容）
 * 在本进程里跑一遍，证明移植没有走样。
 *
 * 做法：直接 #include 插件源码（同一份 .c），然后**只调用纯函数**；
 * 有 extern 依赖的宿主 API（_beginthreadex）在下面补一个桩，避免链接错误。
 *
 * 构建与运行见 build-selftest.cmd。退出码 0 = 全部通过。
 */

#include "difficulty-rules.c"

/* ---- 只为链接：ModStart 引用的线程创建函数，本自测永远不会调到它 ---- */
uintptr_t __cdecl _beginthreadex(void *a, unsigned b, unsigned(__stdcall *c)(void *), void *d,
                                 unsigned e, unsigned *f) {
    (void)a; (void)b; (void)c; (void)d; (void)e; (void)f;
    return 0;
}

static int g_fail = 0;
static int g_pass = 0;

#define CHECK(cond, what)                                                        \
    do {                                                                         \
        if (cond) {                                                              \
            g_pass++;                                                            \
            printf("  [ok]   %s\n", what);                                       \
        } else {                                                                 \
            g_fail++;                                                            \
            printf("  [FAIL] %s  (%s:%d)\n", what, __FILE__, __LINE__);          \
        }                                                                        \
    } while (0)

#define CHECK_EQ_U64(got, want, what)                                            \
    do {                                                                         \
        unsigned long long g_ = (unsigned long long)(got), w_ = (unsigned long long)(want); \
        if (g_ == w_) {                                                          \
            g_pass++;                                                            \
            printf("  [ok]   %s = %llu\n", what, w_);                            \
        } else {                                                                 \
            g_fail++;                                                            \
            printf("  [FAIL] %s: got %llu, want %llu  (%s:%d)\n", what, g_, w_,  \
                   __FILE__, __LINE__);                                          \
        }                                                                        \
    } while (0)

#define CHECK_EQ_I64(got, want, what)                                            \
    do {                                                                         \
        long long g_ = (long long)(got), w_ = (long long)(want);                 \
        if (g_ == w_) {                                                          \
            g_pass++;                                                            \
            printf("  [ok]   %s = %lld\n", what, w_);                            \
        } else {                                                                 \
            g_fail++;                                                            \
            printf("  [FAIL] %s: got %lld, want %lld  (%s:%d)\n", what, g_, w_,  \
                   __FILE__, __LINE__);                                          \
        }                                                                        \
    } while (0)

/* ================================================================== */
static void test_attr_crypto(void) {
    printf("\n-- 属性加解密（session_windows.go:39-40）\n");
    /* (v+8) ^ 0x1f2a015c4bfa2b1c */
    CHECK_EQ_U64(encrypt_attr64(100), (100ULL + 8) ^ 0x1f2a015c4bfa2b1cULL, "encrypt_attr64(100)");
    CHECK_EQ_U64(decrypt_attr64(encrypt_attr64(123456789)), 123456789, "round-trip 123456789");
    CHECK_EQ_U64(decrypt_attr64(encrypt_attr64(0)), 0, "round-trip 0");
    CHECK_EQ_U64(decrypt_attr64(encrypt_attr64(0x7fffffffffffULL)), 0x7fffffffffffULL,
                 "round-trip 大值");
    /* 与 Go 的原始式子一致（DecryptAttr64 = (raw ^ K) - 8） */
    CHECK_EQ_U64(encrypt_attr64(0), 0x1f2a015c4bfa2b1cULL ^ 8ULL, "encrypt(0) 与 Go 同式");
}

static void test_native_float(void) {
    printf("\n-- 原生 float32 解码（monster_hp_formula.go:35-47）\n");
    /* 造一个 stored/guard 对：bits = (stored ^ 0x1f2a025c) - 4 ⇒ stored = (bits+4) ^ 0x1f2a025c
     * guard = stored + bits + 0xc4 */
    struct {
        int32_t bits;
        float value;
    } cases[] = {
        {0x3f800000, 1.0f},  /* 1.0 */
        {0x40000000, 2.0f},  /* 2.0 */
        {0x3f000000, 0.5f},  /* 0.5 */
        {0x3fc00000, 1.5f},  /* 1.5 */
        {0x42c80000, 100.0f} /* 100.0 */
    };
    int i;
    for (i = 0; i < (int)(sizeof(cases) / sizeof(cases[0])); i++) {
        unsigned char b[8];
        uint32_t stored = ((uint32_t)cases[i].bits + NATIVE_FLOAT_SUB) ^ NATIVE_FLOAT_XOR;
        uint32_t guard = stored + (uint32_t)cases[i].bits + NATIVE_FLOAT_GUARD_ADD;
        float got = -1.0f;
        int ok;
        memcpy(b, &stored, 4);
        memcpy(b + 4, &guard, 4);
        ok = decode_native_float(b, &got);
        if (!ok || got != cases[i].value) {
            g_fail++;
            printf("  [FAIL] 解码 %g: ok=%d got=%g\n", (double)cases[i].value, ok, (double)got);
        } else {
            g_pass++;
            printf("  [ok]   解码 %g\n", (double)cases[i].value);
        }
    }
    /* guard = 0 时不做校验（Go: guard != 0 && expected != 0 && guard != expected） */
    {
        unsigned char b[8] = {0};
        uint32_t stored = ((uint32_t)0x3f800000 + NATIVE_FLOAT_SUB) ^ NATIVE_FLOAT_XOR;
        float got = 0.0f;
        memcpy(b, &stored, 4);
        CHECK(decode_native_float(b, &got) && got == 1.0f, "guard=0 也接受");
    }
    /* guard 明显不符 → 拒绝 */
    {
        unsigned char b[8];
        uint32_t stored = ((uint32_t)0x3f800000 + NATIVE_FLOAT_SUB) ^ NATIVE_FLOAT_XOR;
        uint32_t bad = 0xdeadbeef;
        float got = 0.0f;
        memcpy(b, &stored, 4);
        memcpy(b + 4, &bad, 4);
        CHECK(!decode_native_float(b, &got), "guard 不符 → 拒绝");
    }
    /* NaN/Inf 要拒绝：bits = 0x7f800000 = +Inf */
    {
        unsigned char b[8];
        uint32_t stored = ((uint32_t)0x7f800000 + NATIVE_FLOAT_SUB) ^ NATIVE_FLOAT_XOR;
        uint32_t guard = 0;
        float got = 0.0f;
        memcpy(b, &stored, 4);
        memcpy(b + 4, &guard, 4);
        CHECK(!decode_native_float(b, &got), "Inf → 拒绝");
    }
}

static void test_scale_hp(void) {
    printf("\n-- scaleHP（monster_runtime.go:290-299）\n");
    uint64_t v = 0;
    CHECK(scale_hp(0, 1000, &v) && v == 0, "0 * 10 = 0");
    CHECK(scale_hp(100, 100, &v) && v == 100, "100 * 1.00 = 100");
    CHECK(scale_hp(100, 1000, &v) && v == 1000, "100 * 10 = 1000");
    CHECK(scale_hp(100, 200, &v) && v == 200, "100 * 2 = 200");
    CHECK(scale_hp(1, 10, &v) && v == 1, "1 * 0.10 → 向上取 1（Go: next==0 && value>0 → 1）");
    CHECK(scale_hp(3, 50, &v) && v == 1, "3 * 0.50 = 1（整数除法 1.5→1）");
    CHECK(scale_hp(4, 50, &v) && v == 2, "4 * 0.50 = 2");
    CHECK(!scale_hp(0x7fffffffffffffffULL, 100000, &v), "溢出 → 拒绝");
    CHECK(!scale_hp(100, 0, &v), "percent=0 → 拒绝");
}

static void test_maxima_and_rescale(void) {
    printf("\n-- maxima / rescaleEffectiveHP（monster_hp_formula.go:23-66）\n");
    HpLayerAudit h;
    int64_t before = 0, eff = 0;
    memset(&h, 0, sizeof(h));
    {
        int i;
        for (i = 0; i < RATE_COUNT; i++) h.rates[i] = 1.0f;
    }
    h.final_rate = 1.0f;
    h.bonus_rate = 0.0f;
    h.addition = 0;
    CHECK(hp_maxima(&h, 1000, &before, &eff), "maxima(1000) 成功");
    CHECK_EQ_I64(eff, 1000, "全 1.0 系数 → 上限 = base");
    /* rate 全 2.0 → 16 倍（float32 连乘） */
    {
        int i;
        for (i = 0; i < RATE_COUNT; i++) h.rates[i] = 2.0f;
    }
    CHECK(hp_maxima(&h, 1000, &before, &eff), "maxima x16 成功");
    CHECK_EQ_I64(eff, 16000, "4 层 rate=2 → 16 倍");
    /* addition 与 bonus 参与 */
    {
        int i;
        for (i = 0; i < RATE_COUNT; i++) h.rates[i] = 1.0f;
    }
    h.addition = 500;
    h.bonus_rate = 0.5f;
    CHECK(hp_maxima(&h, 1000, &before, &eff), "maxima +加法 +加成 成功");
    CHECK_EQ_I64(eff, 2250, "(1000+500)*1.5 = 2250");
    /* final_rate 只在 value > 1 时生效 */
    h.addition = 0;
    h.bonus_rate = 0.0f;
    h.final_rate = 2.0f;
    CHECK(hp_maxima(&h, 1000, &before, &eff), "maxima finalRate 成功");
    CHECK_EQ_I64(eff, 2000, "finalRate=2 → 2000");
    CHECK_EQ_I64(before, 1000, "beforeFinal 记录 finalRate 之前的值");

    /* rescaleEffectiveHP：50% 血 → 上限 x10，当前血量也 x10 */
    {
        uint64_t cur = 0;
        CHECK(rescale_effective_hp(500, 1000, 10000, &cur), "rescale(500,1000→10000)");
        CHECK_EQ_U64(cur, 5000, "保住 50% 受伤比例");
        CHECK(rescale_effective_hp(1000, 1000, 10000, &cur), "rescale(满血)");
        CHECK_EQ_U64(cur, 10000, "满血 → 满血");
        CHECK(rescale_effective_hp(1, 1000, 100000, &cur), "极低血量");
        CHECK_EQ_U64(cur, 100, "1/1000 → 100/100000");
        CHECK(rescale_effective_hp(1, 1000, 100, &cur), "缩小倍率");
        CHECK_EQ_U64(cur, 1, "1/1000 → 0.1 → 取 1（Go: current>0 && next==0 → 1）");
        /* 大数：不溢出（Go 用 128 位乘法再除） */
        CHECK(rescale_effective_hp(0x7fffffffffffULL, 0x7fffffffffffLL, 100000, &cur),
              "大值 rescale 不溢出");
        CHECK_EQ_U64(cur, 100000, "max → max/ratio");
        CHECK(!rescale_effective_hp(1001, 1000, 10000, &cur), "当前血量 > 旧上限 → 拒绝");
        CHECK(!rescale_effective_hp(10, 0, 10000, &cur), "旧上限 <= 0 → 拒绝");
        CHECK(!rescale_effective_hp(10, 1000, 0, &cur), "新上限 <= 0 → 拒绝");
    }
}

static void test_rules_json(void) {
    printf("\n-- rules.json 解析（手写 JSON）\n");
    RuleSet set;
    char why[256];

    /* 默认生成的那份内容必须能被自己的解析器解析 */
    CHECK(parse_rules(kDefaultRules, sizeof(kDefaultRules) - 1, &set, why, sizeof(why)),
          "内置默认规则可解析");
    CHECK_EQ_I64(PARSED_RULE_COUNT(set) > 0, 1, "解析成功（单文件）");
    CHECK_EQ_I64(PARSED_ENABLED(set), 0, "默认 enabled = false（不偷偷加强）");
    CHECK_EQ_I64(PARSED_RULE_COUNT(set), 2, "默认 2 条规则");
    CHECK_EQ_I64(PARSED_RULES(set)[0].enabled, 0, "示例规则 1 停用");
    CHECK_EQ_I64(PARSED_RULES(set)[0].percent, 1000, "示例规则 1 percent=1000");
    CHECK_EQ_I64(PARSED_RULES(set)[0].id_count, 1, "示例规则 1 有 1 个副本 id");
    CHECK_EQ_U64(PARSED_RULES(set)[0].ids[0], 100005014, "示例规则 1 副本 id");
    CHECK_EQ_I64(PARSED_RULES(set)[1].all, 1, "示例规则 2 all=true");
    CHECK_EQ_I64(PARSED_RULES(set)[1].percent, 200, "示例规则 2 percent=200");

    /* 字符串形式的 id、多余空白、未知键 */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"extra\":{\"deep\":[1,2,{\"x\":\"y\"}]},"
                        "\"rules\":[{\"id\":\"a\",\"enabled\":true,\"dungeonIds\":[\"100005014\",2],"
                        "\"percent\":500}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "字符串 id + 未知嵌套键可解析");
        CHECK_EQ_I64(PARSED_ENABLED(set), 1, "enabled=true");
        CHECK_EQ_I64(PARSED_RULES(set)[0].id_count, 2, "字符串与数字混写的 id 数组");
        CHECK_EQ_U64(PARSED_RULES(set)[0].ids[0], 100005014, "字符串 id 转数字");
        CHECK_EQ_U64(PARSED_RULES(set)[0].ids[1], 2, "数字 id");
        CHECK_EQ_I64(PARSED_RULES(set)[0].percent, 500, "percent=500");
    }
    /* all:true 不需要 dungeonIds */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"w\",\"enabled\":true,"
                        "\"all\":true,\"percent\":150}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "all:true 可解析");
        CHECK_EQ_I64(PARSED_RULES(set)[0].all, 1, "all=1");
    }
    /* 拒绝：schema 不对 / 缺键 / 语法错 / 空规则范围 / percent 越界 / 规则太多 */
    {
        const char *cases[] = {
            "{\"schema\":2,\"enabled\":true,\"rules\":[]}",
            "{\"schema\":1,\"rules\":[]}",
            "{\"enabled\":true,\"rules\":[]}",
            "{\"schema\":1,\"enabled\":true}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"enabled\":true,\"percent\":200}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"enabled\":true,\"all\":true,\"percent\":0}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"enabled\":true,\"all\":true,\"percent\":100001}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"enabled\":true,\"dungeonIds\":[]}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"enabled\":true,\"all\":true,\"percent\":200}",
            "[]",
            "{\"schema\":1,\"enabled\":maybe,\"rules\":[]}",
        };
        int i;
        for (i = 0; i < (int)(sizeof(cases) / sizeof(cases[0])); i++) {
            char tag[64];
            _snprintf_s(tag, sizeof(tag), _TRUNCATE, "拒绝非法规则 %d", i);
            if (parse_rules(cases[i], strlen(cases[i]), &set, why, sizeof(why))) {
                g_fail++;
                printf("  [FAIL] %s —— 竟然通过了\n", tag);
            } else {
                g_pass++;
                printf("  [ok]   %s（%s）\n", tag, why);
            }
        }
    }
    /* 停用规则里的非法 percent 不阻断（Go 只校验 enabled 的规则） */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"off\",\"enabled\":false,"
                        "\"all\":true,\"percent\":0}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "停用规则不校验 percent");
    }
    /* 没写 enabled 的规则 = 停用（默认安全）：允许存在，但永远不命中 */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"all\":true,\"percent\":200}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "省略 enabled 的规则可解析");
        CHECK_EQ_I64(PARSED_RULES(set)[0].enabled, 0, "省略 enabled → 默认停用");
        memset(&g_engine.set, 0, sizeof(g_engine.set));
        g_engine.set = set;
        CHECK(match_rule(1, 1) == NULL, "省略 enabled 的 all 规则不命中（默认不加强）");
        memset(&g_engine.set, 0, sizeof(g_engine.set));
    }
    /* 31 条以内可以通过，32 条以上拒绝 */
    {
        char big[8192];
        int i, n = 0;
        n += _snprintf_s(big + n, sizeof(big) - n, _TRUNCATE, "{\"schema\":1,\"enabled\":true,\"rules\":[");
        for (i = 0; i < 33; i++)
            n += _snprintf_s(big + n, sizeof(big) - n, _TRUNCATE, "%s{\"id\":\"r%d\",\"enabled\":false}",
                             i ? "," : "", i);
        _snprintf_s(big + n, sizeof(big) - n, _TRUNCATE, "]}");
        CHECK(!parse_rules(big, strlen(big), &set, why, sizeof(why)), "33 条规则 → 拒绝（上限 32）");
    }
}


/* ================================================================== */
/* rules.d 多文件加载：合并顺序 = 匹配优先级、坏文件只跳过它自己、         */
/* 目录不存在时与旧版行为一致。                                          */
/* ================================================================== */
#include <direct.h>
#include <sys/stat.h>

#define TMP_ROOT L"selftest-tmp"
#define TMP_RULESDIR TMP_ROOT L"\\rules.d"

static void tmp_write_rulesdir(const wchar_t *name, const char *text) {
    wchar_t path[MAX_PATH];
    FILE *f;
    _snwprintf(path, MAX_PATH, L"%s\\%s", TMP_RULESDIR, name);
    f = _wfopen(path, L"wb");
    if (!f) {
        g_fail++;
        printf("  [FAIL] 写不出 %ls\n", path);
        return;
    }
    fwrite(text, 1, strlen(text), f);
    fclose(f);
}

/* 按 scan_rules_dir 的结果加载 rules.d（与 config_try 同一套调用顺序）。
 * 返回：0 = 目录不存在/没有 .json（不算错误）；1 = 已并入 set。 */
static int selftest_load_rules_dir(RuleSet *set) {
    DirRuleFile files[MAX_RULE_FILES];
    uint64_t fp = 0;
    int n;
    n = scan_rules_dir(files, MAX_RULE_FILES, &fp);
    if (n <= 0) return 0;
    return load_rules_dir_files(set, files, n, 0);
}

static void tmp_setup(void) {
    _wrmdir(TMP_RULESDIR);
    _wrmdir(TMP_ROOT);
    _wmkdir(TMP_ROOT);
    _wmkdir(TMP_RULESDIR);
}

static void tmp_teardown(void) {
    const wchar_t *names[] = {L"a-mod.json", L"b-user.json", L"c-broken.json", L"z-last.json",
                              L"cap-ok.json", L"cap-bad.json"};
    int i;
    for (i = 0; i < 6; i++) {
        wchar_t p[MAX_PATH];
        _snwprintf(p, MAX_PATH, L"%s\\%s", TMP_RULESDIR, names[i]);
        _wremove(p);
    }
    _wrmdir(TMP_RULESDIR);
    _wrmdir(TMP_ROOT);
}

/* 把 g_dir 指到 TMP_ROOT，于是规则路径 = TMP_ROOT\rules.d\*.json / TMP_ROOT\rules.json */
#define WITH_TMP_DIR_BEGIN()                                                     \
    {                                                                            \
        wchar_t saved_dir[MAX_PATH];                                             \
        wcscpy_s(saved_dir, MAX_PATH, g_dir);                                    \
        wcscpy_s(g_dir, MAX_PATH, TMP_ROOT);

#define WITH_TMP_DIR_END()                                                       \
    wcscpy_s(g_dir, MAX_PATH, saved_dir);                                        \
    }

static void test_rules_dir_multi(void) {
    char why[256];
    const char *user_all =
        "{\"schema\":1,\"enabled\":true,\"rules\":["
        "{\"id\":\"user-all\",\"enabled\":true,\"all\":true,\"percent\":200}]}";
    const char *mod_odyssey =
        "{\"schema\":1,\"enabled\":true,\"rules\":["
        "{\"id\":\"mod-odyssey\",\"enabled\":true,\"dungeonIds\":[100004934,100004990],"
        "\"percent\":1000,\"attackPercent\":1000}]}";

    printf("\n-- rules.d 多文件加载（优先级 / 坏文件隔离 / 目录缺失）\n");
    tmp_setup();
    WITH_TMP_DIR_BEGIN();

    /* 1) rules.d 不存在：跳过，不动 set，也不算错误（与旧版逐字节一致） */
    {
        RuleSet set;
        _wrmdir(TMP_RULESDIR);
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 0, "rules.d 不存在 → 跳过（不算错误）");
        CHECK_EQ_I64(set.rule_file_count, 0, "rules.d 不存在 → 规则文件数仍为 0");
        CHECK(config_usable() == 0, "set 为空 → 配置不可用（与旧版同义）");
        _wmkdir(TMP_RULESDIR);
    }

    /* 2) 目录存在但一个 .json 都没有：同样跳过 */
    {
        RuleSet set;
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 0, "rules.d 为空目录 → 跳过");
        CHECK_EQ_I64(set.rule_file_count, 0, "空目录 → 0 份文件");
    }

    /* 3) 文件名升序 = 跨文件优先级；mod 自带规则赢过玩家的 all 规则 */
    tmp_write_rulesdir(L"b-user.json", user_all);
    tmp_write_rulesdir(L"a-mod.json", mod_odyssey);
    {
        RuleSet set, main_set;
        const char *main_json =
            "{\"schema\":1,\"enabled\":true,\"rules\":["
            "{\"id\":\"user-main\",\"enabled\":true,\"all\":true,\"percent\":300}]}";
        memset(&set, 0, sizeof(set));
        memset(&main_set, 0, sizeof(main_set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 1, "rules.d 两份文件加载成功");
        CHECK_EQ_I64(set.rule_file_count, 2, "合并后 2 份文件");
        CHECK(strcmp(set.files[0].name, "a-mod.json") == 0, "文件顺序 = 文件名升序（a-mod 先）");
        CHECK(strcmp(set.files[1].name, "b-user.json") == 0, "b-user 在后（优先级更低）");
        CHECK(parse_rules(main_json, strlen(main_json), &main_set, why, sizeof(why)),
              "玩家 rules.json 可解析");
        CHECK_EQ_I64(add_rule_file(&set, "rules.json", &main_set), 1, "rules.json 排最后并入");
        g_engine.set = set;
        {
            const Rule *r = match_rule(100004934, 1);
            CHECK(r != NULL && strcmp(r->id, "mod-odyssey") == 0,
                  "奥德赛副本 100004934 → **mod 自带规则赢**（×10）");
            CHECK(strcmp(g_engine.set.matched_file, "a-mod.json") == 0,
                  "matched_file = a-mod.json（来源可追溯）");
            r = match_rule(100004990, 1);
            CHECK(r != NULL && strcmp(r->id, "mod-odyssey") == 0, "另一个奥德赛副本 → 同一份规则");
            r = match_rule(12345, 1);
            CHECK(r != NULL && strcmp(r->id, "user-all") == 0 && r->percent == 200,
                  "普通副本 → 落到 b-user.json 的通用规则（×2），mod 规则不越界");
        }
        /* 优先级反证：把 mod 规则放到"名字更靠后"的文件里，就该轮到它前面的赢 */
        tmp_write_rulesdir(L"a-mod.json", user_all);
        tmp_write_rulesdir(L"z-last.json", mod_odyssey);
        {
            RuleSet s2;
            memset(&s2, 0, sizeof(s2));
            selftest_load_rules_dir(&s2);
            g_engine.set = s2;
            {
                const Rule *r = match_rule(100004934, 1);
                CHECK(r != NULL && strcmp(r->id, "user-all") == 0,
                      "把 mod 规则挪到 z-last.json → 前面的 a-mod.json 先命中（证明排序真的生效）");
            }
        }
        _wremove(TMP_RULESDIR L"\\z-last.json");
        tmp_write_rulesdir(L"a-mod.json", mod_odyssey);
    }

    /* 4) 坏文件只跳过它自己：另两份好文件照常生效 */
    tmp_write_rulesdir(L"c-broken.json", "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"x\",");
    {
        RuleSet set;
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 1,
                     "含 1 份坏文件 → rules.d 仍算加载成功（**只跳过那一份**，不让整份失效）");
        CHECK_EQ_I64(set.rule_file_count, 2, "坏文件不计入 → 仍是 2 份（a-mod / b-user）");
        CHECK(strcmp(set.files[0].name, "a-mod.json") == 0, "坏文件不影响好文件的顺序");
        g_engine.set = set;
        {
            const Rule *r = match_rule(100004934, 1);
            CHECK(r != NULL && strcmp(r->id, "mod-odyssey") == 0, "坏文件里的规则一条都没混进来");
        }
    }
    /* 顺带：玩家 rules.json 坏掉仍然是**整份不可用**（旧语义不变）——
     * 这里用 parse_rules 的返回值代表 config_try 里那条分支。 */
    {
        RuleSet bad_set;
        const char *bad_json = "{\"schema\":1,\"enabled\":true,";
        memset(&bad_set, 0, sizeof(bad_set));
        why[0] = 0;
        CHECK(!parse_rules(bad_json, strlen(bad_json), &bad_set, why, sizeof(why)),
              "rules.json 坏掉 -> parse_rules 失败（config_try 里对应整份不可用分支）");
    }

    /* 5) rules.d 里全是坏文件 → 0 份生效（= 配置不可用），但不报"加载失败" */
    tmp_write_rulesdir(L"a-mod.json", "{ 不是 JSON");
    tmp_write_rulesdir(L"b-user.json", "[\"顶层是数组\"]");
    {
        RuleSet set;
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 1, "全坏也返回 1（不把整份配置判死）");
        CHECK_EQ_I64(set.rule_file_count, 0, "全坏 → 0 份生效文件");
        g_engine.set = set;
        CHECK(match_rule(100004934, 1) == NULL, "全坏 → 不命中任何规则（保持原版）");
        CHECK(config_usable() == 0, "全坏 → 配置不可用（还原并停止接管）");
    }

    /* 6) 只有 rules.d、没有 rules.json 时，set 非空 ⇒ 可用 */
    tmp_write_rulesdir(L"a-mod.json", mod_odyssey);
    _wremove(TMP_RULESDIR L"\\b-user.json");
    _wremove(TMP_RULESDIR L"\\c-broken.json");
    {
        RuleSet set;
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 1, "只装 mod 规则（无 rules.json）时加载成功");
        g_engine.set = set;
        CHECK(config_usable() == 1, "只有 rules.d 的规则也能用（rules.json 缺失时由 config_try 处理）");
        g_engine.set = set;
        {
            const Rule *r = match_rule(100004934, 1);
            CHECK(r != NULL && r->percent == 1000, "奥德赛副本 → ×10");
            r = match_rule(12345, 1);
            CHECK(r == NULL, "普通副本 → 没有规则（原版，不受影响）");
        }
    }

    /* 7) 文件顶层 enabled=false → 整份不参与（mod 想临时关掉自己的范围规则） */
    {
        RuleSet set, mod_off, user_set;
        const char *mod_off_json =
            "{\"schema\":1,\"enabled\":false,\"rules\":["
            "{\"id\":\"odyssey-10x\",\"enabled\":true,\"all\":true,\"percent\":1000}]}";
        memset(&set, 0, sizeof(set));
        memset(&mod_off, 0, sizeof(mod_off));
        memset(&user_set, 0, sizeof(user_set));
        CHECK(parse_rules(mod_off_json, strlen(mod_off_json), &mod_off, why, sizeof(why)),
              "停用文件可解析");
        CHECK(parse_rules(user_all, strlen(user_all), &user_set, why, sizeof(why)),
              "玩家规则可解析");
        add_rule_file(&set, "odyssey.hardcore.json", &mod_off);
        add_rule_file(&set, "rules.json", &user_set);
        g_engine.set = set;
        {
            const Rule *r = match_rule(100004934, 1);
            CHECK(r != NULL && strcmp(r->id, "user-all") == 0,
                  "mod 文件顶层 enabled=false → 整份跳过，落到玩家规则");
        }
    }

    /* 8) 规则总条数上限（跨文件共享 32 条池） */
    {
        RuleSet set, one;
        char big[8192];
        int i, k = 0;
        memset(&set, 0, sizeof(set));
        memset(&one, 0, sizeof(one));
        k += _snprintf_s(big + k, sizeof(big) - k, _TRUNCATE,
                         "{\"schema\":1,\"enabled\":true,\"rules\":[");
        for (i = 0; i < 32; i++)
            k += _snprintf_s(big + k, sizeof(big) - k, _TRUNCATE,
                             "%s{\"id\":\"r%d\",\"enabled\":false}", i ? "," : "", i);
        _snprintf_s(big + k, sizeof(big) - k, _TRUNCATE, "]}");
        CHECK(parse_rules(big, strlen(big), &one, why, sizeof(why)), "32 条可解析");
        CHECK_EQ_I64(add_rule_file(&set, "a.json", &one), 1, "第一份 32 条可并入");
        CHECK_EQ_I64(add_rule_file(&set, "b.json", &one), 0, "再加 32 条 → 超过 32 条上限，拒绝");
    }

    memset(&g_engine.set, 0, sizeof(g_engine.set));
    WITH_TMP_DIR_END();
    tmp_teardown();
}


static void test_rule_match(void) {
    printf("\n-- 规则匹配（第一条命中生效）\n");
    char why[256];
    const char *t =
        "{\"schema\":1,\"enabled\":true,\"rules\":["
        "{\"id\":\"first\",\"enabled\":true,\"dungeonIds\":[100],\"percent\":300},"
        "{\"id\":\"second\",\"enabled\":true,\"dungeonIds\":[100,200],\"percent\":900},"
        "{\"id\":\"all\",\"enabled\":true,\"all\":true,\"percent\":150}]}";
    memset(&g_engine.set, 0, sizeof(g_engine.set));
    CHECK(parse_rules(t, strlen(t), &g_engine.set, why, sizeof(why)), "测试规则集可解析");
    {
        const Rule *r = match_rule(100, 1);
        CHECK(r != NULL && strcmp(r->id, "first") == 0, "副本 100 → 第一条（300）");
        r = match_rule(200, 1);
        CHECK(r != NULL && strcmp(r->id, "second") == 0, "副本 200 → 第二条（900）");
        r = match_rule(999, 1);
        CHECK(r != NULL && strcmp(r->id, "all") == 0, "其它副本 → all 规则（150）");
    }
    /* 顶层 enabled=false → 不命中任何规则 */
    g_engine.set.files[0].enabled = 0;
    CHECK(match_rule(100, 1) == NULL, "顶层 enabled=false → 全部保持原版");
    g_engine.set.files[0].enabled = 1;
    /* all 规则优先时，dungeonIds 命中也走 all */
    {
        const char *t2 = "{\"schema\":1,\"enabled\":true,\"rules\":["
                         "{\"id\":\"all\",\"enabled\":true,\"all\":true,\"percent\":150},"
                         "{\"id\":\"one\",\"enabled\":true,\"dungeonIds\":[100],\"percent\":300}]}";
        memset(&g_engine.set, 0, sizeof(g_engine.set));
        CHECK(parse_rules(t2, strlen(t2), &g_engine.set, why, sizeof(why)), "all 在前的规则集可解析");
        {
            const Rule *r = match_rule(100, 1);
            CHECK(r != NULL && strcmp(r->id, "all") == 0, "all 在前 → 先命中 all");
        }
    }
    memset(&g_engine.set, 0, sizeof(g_engine.set));
}

/* ================================================================== */
static void test_native_int(void) {
    printf("\n-- 原生 32 位整数属性（monster_movement.go:24-39）\n");
    unsigned char b[8];
    uint32_t v = 0;
    /* 编码/解码往返 */
    {
        uint32_t cases[] = {0, 1, 100, 12345, 0x7fffffffu};
        int i;
        for (i = 0; i < (int)(sizeof(cases) / sizeof(cases[0])); i++) {
            char tag[64];
            uint32_t got = 0;
            encode_native_int_pair(b, cases[i]);
            _snprintf_s(tag, sizeof(tag), _TRUNCATE, "原生整数往返 %u", cases[i]);
            CHECK(decode_native_int(b, &got) && got == cases[i], tag);
        }
    }
    /* stored = (v+4) ^ K —— 与 Go nativeIntPair 同式 */
    encode_native_int_pair(b, 100);
    CHECK_EQ_U64(raw32(b), ((100u + NATIVE_INT_SUB) ^ NATIVE_INT_XOR), "stored = (v+4)^0x1f2a025c");
    CHECK_EQ_U64(raw32(b + 4), raw32(b) + 100u + NATIVE_INT_GUARD_ADD, "guard = stored + v + 0xc4");
    /* guard = 0 也接受（Go: guard != 0 && ...） */
    {
        uint32_t stored = (500u + NATIVE_INT_SUB) ^ NATIVE_INT_XOR;
        put32(b, stored);
        put32(b + 4, 0);
        CHECK(decode_native_int(b, &v) && v == 500, "guard=0 也接受");
    }
    /* guard 不符 → 拒绝 */
    encode_native_int_pair(b, 500);
    put32(b + 4, 0xdeadbeefu);
    CHECK(!decode_native_int(b, &v), "guard 不符 → 拒绝");
    /* (stored^K)-4 > MaxInt32 → 拒绝（Go: value > math.MaxInt32） */
    {
        uint32_t bad = 0x80000000u; /* (stored^K)-4 = 0x80000000 > MaxInt32 */
        put32(b, (bad + NATIVE_INT_SUB) ^ NATIVE_INT_XOR);
        put32(b + 4, 0);
        CHECK(!decode_native_int(b, &v), "超过 MaxInt32 → 拒绝");
    }
    /* 白名单（monster_movement.go:50-53） */
    CHECK(native_stat_whitelisted(OFF_STAT_ATK_PHYS) && native_stat_whitelisted(OFF_STAT_ATK_MAG),
          "0x398 / 0x3b8 在白名单里");
    CHECK(!native_stat_whitelisted(0x400), "0x400 不在白名单里");
    /* 倍率取整与溢出（Go: uint64(Original)*percent/100，截断） */
    {
        uint64_t v10 = (uint64_t)333 * 1000 / 100;
        uint64_t v1 = (uint64_t)3 * 50 / 100;
        CHECK_EQ_U64(v10, 3330, "333 * 10 = 3330");
        CHECK_EQ_U64(v1, 1, "3 * 0.50 = 1（整数除法截断，与 Go 一致）");
        CHECK((uint64_t)0x7fffffff * 100000 / 100 > (uint64_t)NATIVE_INT_MAX, "大值乘倍率会越界");
    }
}

static void test_attack_percent_json(void) {
    printf("\n-- attackPercent 解析（新增字段，默认 100）\n");
    RuleSet set;
    char why[256];

    /* 老文件（只有 percent）必须照旧工作，attackPercent 默认 100 = 不改 */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":["
                        "{\"id\":\"old\",\"enabled\":true,\"all\":true,\"percent\":1000}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "老规则（无 attackPercent）可解析");
        CHECK_EQ_I64(PARSED_RULES(set)[0].percent, 1000, "percent 照旧");
        CHECK_EQ_I64(PARSED_RULES(set)[0].attack_percent, 100, "attackPercent 省略 → 默认 100（不改）");
    }
    /* 任务书示例 */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":["
                        "{\"id\":\"all-10x\",\"enabled\":true,\"all\":true,\"dungeonIds\":[],"
                        "\"percent\":1000,\"attackPercent\":1000}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)), "示例规则可解析");
        CHECK_EQ_I64(PARSED_RULES(set)[0].percent, 1000, "percent=1000");
        CHECK_EQ_I64(PARSED_RULES(set)[0].attack_percent, 1000, "attackPercent=1000");
    }
    /* 边界：1 / 100000 通过；0 / 100001 / 负数 拒绝，且整份配置不可用（不"用一半"） */
    {
        const char *ok1 = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"a\",\"enabled\":true,"
                          "\"all\":true,\"percent\":100,\"attackPercent\":1}]}";
        const char *ok2 = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"a\",\"enabled\":true,"
                          "\"all\":true,\"percent\":100,\"attackPercent\":100000}]}";
        const char *bad[] = {
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"a\",\"enabled\":true,\"all\":true,"
            "\"percent\":1000,\"attackPercent\":0}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"a\",\"enabled\":true,\"all\":true,"
            "\"percent\":1000,\"attackPercent\":100001}]}",
            "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"a\",\"enabled\":true,\"all\":true,"
            "\"percent\":1000,\"attackPercent\":-1}]}",
        };
        int i;
        CHECK(parse_rules(ok1, strlen(ok1), &set, why, sizeof(why)), "attackPercent=1 通过");
        CHECK(parse_rules(ok2, strlen(ok2), &set, why, sizeof(why)), "attackPercent=100000 通过");
        for (i = 0; i < (int)(sizeof(bad) / sizeof(bad[0])); i++) {
            char tag[96];
            _snprintf_s(tag, sizeof(tag), _TRUNCATE, "拒绝非法 attackPercent 用例 %d", i);
            if (parse_rules(bad[i], strlen(bad[i]), &set, why, sizeof(why))) {
                g_fail++;
                printf("  [FAIL] %s —— 竟然通过了\n", tag);
            } else {
                g_pass++;
                printf("  [ok]   %s（%s）\n", tag, why);
            }
        }
    }
    /* 停用的规则里 attackPercent 越界不阻断（与 percent 同策略） */
    {
        const char *t = "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"off\",\"enabled\":false,"
                        "\"all\":true,\"percent\":1000,\"attackPercent\":0}]}";
        CHECK(parse_rules(t, strlen(t), &set, why, sizeof(why)),
              "停用规则的 attackPercent 不校验");
    }
    /* 内置默认规则文件也要能被解析，并且示例里的 attackPercent 读得出来 */
    {
        CHECK(parse_rules(kDefaultRules, sizeof(kDefaultRules) - 1, &set, why, sizeof(why)),
              "内置默认规则（含 attackPercent）可解析");
        CHECK_EQ_I64(PARSED_RULES(set)[0].attack_percent, 100, "示例 1 attackPercent=100");
        CHECK_EQ_I64(PARSED_RULES(set)[1].attack_percent, 1000, "示例 2 attackPercent=1000");
    }
}

static void test_hp_new_maxima(void) {
    printf("\n-- 改后有效上限：同一份审计 + maxima(new_base)（2026-10-07 实机 bug 的核心）\n");
    HpLayerAudit h;
    int64_t before = 0, eff = 0, eff10 = 0;
    memset(&h, 0, sizeof(h));
    {
        int i;
        for (i = 0; i < RATE_COUNT; i++) h.rates[i] = 1.0f;
    }
    h.final_rate = 1.0f;
    h.bonus_rate = 0.0f;
    h.addition = 0;
    CHECK(hp_maxima(&h, 1000, &before, &eff), "旧上限");
    CHECK_EQ_I64(eff, 1000, "旧上限 = 1000");
    /* 关键：**不改 h**（不重读内存），直接换成改后的基础值再算一次 */
    CHECK(hp_maxima(&h, 10000, &before, &eff10), "新上限（同一份审计）");
    CHECK_EQ_I64(eff10, 10000, "新上限 = 10000（10 倍基础值 → 10 倍上限）");
    /* 同一份审计可以算两次：旧基础值那次仍然是 1000（说明 h 没被改坏） */
    CHECK(hp_maxima(&h, 1000, &before, &eff), "同一份审计还能算旧基础值");
    CHECK_EQ_I64(eff, 1000, "旧上限仍是 1000（审计结构没被改写）");
    /* 旧写法为什么必错：拿"改后的 base"再读一遍现场（现场还是旧 base）——
     * 下面用一个假的现场字节复现这个判据。 */
    {
        unsigned char b[DESC_READ_LEN];
        uint64_t old_base = 1000, new_base = 10000;
        memset(b, 0, sizeof(b));
        put64(b + OFF_DESC_BASE, encrypt_attr64(old_base));
        CHECK(decrypt_attr64(raw64(b + OFF_DESC_BASE)) == old_base,
              "现场字节里还是旧 base");
        CHECK(decrypt_attr64(raw64(b + OFF_DESC_BASE)) != new_base,
              "旧写法 hp_layer(..., new_base) 的 base 核对必然失败 → 静默丢弃（这就是实机 bug）");
    }
}

/* ================================================================== */
/* 假内存集成测试：在真实可读写内存里搭一个"假怪"，跑一遍                       */
/* collect_one → hp_reconcile → attack_reconcile → 还原往返。                 */
/* 这一步能直接抓住"applied 恒为 0"那类 bug（实机跑不了的逻辑也能验）。        */
/* ================================================================== */
#define FAKE_SIZE 0x40000
static uint64_t g_fake;

static void fput64(uint64_t a, uint64_t v) { memcpy((void *)(uintptr_t)a, &v, 8); }
static void fput32(uint64_t a, uint32_t v) { memcpy((void *)(uintptr_t)a, &v, 4); }
static uint64_t fget64(uint64_t a) {
    uint64_t v = 0;
    memcpy(&v, (const void *)(uintptr_t)a, 8);
    return v;
}

/* 原生 float 对：stored=(bits+4)^K；guard=stored+bits+0xc4 */
static void fput_float_pair(uint64_t a, float f) {
    int32_t bits;
    uint32_t stored, guard;
    memcpy(&bits, &f, 4);
    stored = ((uint32_t)bits + NATIVE_FLOAT_SUB) ^ NATIVE_FLOAT_XOR;
    guard = stored + (uint32_t)bits + NATIVE_FLOAT_GUARD_ADD;
    fput32(a, stored);
    fput32(a + 4, guard);
}

static void fake_build(void) {
    uint64_t actor = g_fake + 0x10000;
    uint64_t vtable = g_fake + 0x1000;
    uint64_t node = g_fake + 0x20000;
    uint64_t control = g_fake + 0x20100;
    int layer, i;
    memset((void *)(uintptr_t)g_fake, 0, FAKE_SIZE);
    /* 虚表：vtable+0x12c8 必须 == va(0x145c14140)（g_base = EXPECTED_IMAGE_BASE 时就是原值） */
    fput64(vtable + OFF_VT_DESC_L0, va(RVA_ACTOR_DESC_L0));
    fput64(actor, vtable);
    /* 节点：+0x10 key（低 32 位，>>16 == 3）、+0x20 control、+0x28 = actor+0x30 */
    fput64(node + OFF_NODE_KEY, 0x30001ULL);
    fput64(node + OFF_NODE_CONTROL, control);
    fput64(node + OFF_NODE_REF, actor + ACTOR_REF_DELTA);
    fput64(control + OFF_CONTROL_COUNT, 7);
    /* 四层血量描述块 */
    for (layer = 0; layer < HP_LAYERS; layer++) {
        uint64_t d = actor + OFF_ACTOR_DESC0 + (uint64_t)layer * DESC_STRIDE;
        fput64(d + OFF_DESC_BASE, encrypt_attr64(1000 + (uint64_t)layer * 100));
        fput32(d + 8 + 4, 0); /* base 的 guard = 0 */
        for (i = 0; i < RATE_COUNT; i++) fput_float_pair(d + OFF_DESC_RATE0 + (size_t)i * 8, 1.0f);
        fput_float_pair(d + OFF_DESC_FINAL, 1.0f);
        fput_float_pair(d + OFF_DESC_BONUS, 0.0f);
        fput64(d + OFF_DESC_ADD, encrypt_attr64(0));
        /* 原生整数属性：物攻 / 魔攻 */
        {
            unsigned char pair[8];
            uint64_t off;
            encode_native_int_pair(pair, 300);
            off = d + OFF_STAT_ATK_PHYS;
            memcpy((void *)(uintptr_t)off, pair, 8);
            encode_native_int_pair(pair, 900);
            off = d + OFF_STAT_ATK_MAG;
            memcpy((void *)(uintptr_t)off, pair, 8);
        }
    }
    fput64(actor + OFF_ACTOR_HP, encrypt_attr64(500)); /* 半血 */
    fput32(actor + 0x66c0, 1);
}

static void fake_reset_tables(void) {
    if (g_engine.entries)
        memset(g_engine.entries, 0, (size_t)g_engine.entry_cap * sizeof(Entry));
    g_engine.entry_count = 0;
    if (g_engine.stats) memset(g_engine.stats, 0, (size_t)g_engine.stat_cap * sizeof(StatEntry));
    g_engine.stat_count = 0;
    g_live_count = 0;
}

static void fake_id(MonsterId *id) {
    memset(id, 0, sizeof(*id));
    id->scene = g_fake + 0x30000;
    id->manager = g_fake + 0x30100;
    id->node = g_fake + 0x20000;
    id->control = g_fake + 0x20100;
    id->actor = g_fake + 0x10000;
    id->vtable = g_fake + 0x1000;
    id->key = 0x30001u;
}

static void test_fake_memory_apply(void) {
    printf("\n-- 假内存集成：血量 + 伤害 应用 / 还原往返\n");
    MonsterId id;
    MonsterSnap snap;
    SceneSnap scene;
    TickCtx ctx;
    StatCtx sctx;
    uint64_t actor, d0, d3;

    g_fake = (uint64_t)(uintptr_t)VirtualAlloc(NULL, FAKE_SIZE, MEM_COMMIT | MEM_RESERVE,
                                               PAGE_READWRITE);
    if (!g_fake) {
        g_fail++;
        printf("  [FAIL] VirtualAlloc 失败，跳过假内存测试\n");
        return;
    }
    fake_build();
    fake_id(&id);
    actor = id.actor;
    d0 = actor + OFF_ACTOR_DESC0;
    d3 = actor + OFF_ACTOR_DESC0 + 3 * DESC_STRIDE;

    CHECK(identity_valid(&id), "假怪的 identity_valid（5 条 + vtable+0x12c8）通过");

    /* 1) 快照：应当收下 1 只合格怪 */
    memset(&scene, 0, sizeof(scene));
    g_live_count = 0;
    collect_one(&id, &scene);
    CHECK_EQ_I64(scene.count, 1, "collect_one 收下 1 只（层 base / guard / 当前血量都合格）");
    CHECK_EQ_I64(scene.rejected, 0, "没有被丢弃的实体");
    snap = scene.mons[0];

    /* 2) 血量 10 倍：这一步在旧版里必然 applied=0（hp_layer(new_base) 自相矛盾） */
    memset(&ctx, 0, sizeof(ctx));
    ctx.percent = 1000;
    hp_reconcile(&snap, 1000, &ctx);
    CHECK_EQ_I64(ctx.applied, 1, "血量写入成功 1 次（旧版这里恒为 0）");
    CHECK_EQ_I64(ctx.failed, 0, "血量写入没有失败");
    CHECK_EQ_U64(decrypt_attr64(fget64(d0)), 10000, "层0 基础值 1000 → 10000");
    CHECK_EQ_U64(decrypt_attr64(fget64(d3)), 13000, "层3 基础值 1300 → 13000");
    CHECK_EQ_U64(decrypt_attr64(fget64(actor + OFF_ACTOR_HP)), 5000, "半血 500 → 5000（保住受伤比例）");

    /* 3) 幂等：同一倍率再来一次，不该复利 */
    memset(&scene, 0, sizeof(scene));
    g_live_count = 0;
    collect_one(&id, &scene);
    memset(&ctx, 0, sizeof(ctx));
    hp_reconcile(&scene.mons[0], 1000, &ctx);
    CHECK_EQ_I64(ctx.applied, 0, "同倍率重复应用 → 0 次（不复利）");
    CHECK_EQ_U64(decrypt_attr64(fget64(d0)), 10000, "层0 仍是 10000");

    /* 4) 还原往返：回到 100 必须逐字节回到原值 */
    memset(&scene, 0, sizeof(scene));
    g_live_count = 0;
    collect_one(&id, &scene);
    memset(&ctx, 0, sizeof(ctx));
    hp_reconcile(&scene.mons[0], 100, &ctx);
    CHECK_EQ_I64(ctx.applied, 1, "血量还原 1 次");
    CHECK_EQ_U64(decrypt_attr64(fget64(d0)), 1000, "层0 回到 1000");
    CHECK_EQ_U64(decrypt_attr64(fget64(d3)), 1300, "层3 回到 1300");
    CHECK_EQ_U64(decrypt_attr64(fget64(actor + OFF_ACTOR_HP)), 500, "当前血量回到 500");

    /* 5) 伤害 10 倍：物攻 300 → 3000，魔攻 900 → 9000 */
    memset(&sctx, 0, sizeof(sctx));
    sctx.percent = 1000;
    attack_reconcile(&snap, 1000, &sctx);
    CHECK_EQ_I64(sctx.applied, 2, "物攻 + 魔攻 各写 1 次");
    CHECK_EQ_I64(sctx.failed, 0, "伤害写入没有失败");
    {
        unsigned char pair[8];
        uint32_t v = 0;
        memcpy(pair, (const void *)(uintptr_t)(d0 + OFF_STAT_ATK_PHYS), 8);
        CHECK(decode_native_int(pair, &v) && v == 3000, "层0 物攻 300 → 3000");
        memcpy(pair, (const void *)(uintptr_t)(d3 + OFF_STAT_ATK_MAG), 8);
        CHECK(decode_native_int(pair, &v) && v == 9000, "层3 魔攻 900 → 9000");
        /* guard 必须同步更新（stored + v + 0xc4） */
        CHECK_EQ_U64(raw32(pair + 4), raw32(pair) + 9000u + NATIVE_INT_GUARD_ADD,
                     "写回的 guard = stored + v + 0xc4");
    }
    /* 6) 伤害还原往返 */
    memset(&sctx, 0, sizeof(sctx));
    attack_reconcile(&snap, 100, &sctx);
    CHECK_EQ_I64(sctx.applied, 2, "伤害还原 2 次");
    {
        unsigned char pair[8];
        uint32_t v = 0;
        memcpy(pair, (const void *)(uintptr_t)(d0 + OFF_STAT_ATK_PHYS), 8);
        CHECK(decode_native_int(pair, &v) && v == 300, "层0 物攻回到 300");
        memcpy(pair, (const void *)(uintptr_t)(d3 + OFF_STAT_ATK_MAG), 8);
        CHECK(decode_native_int(pair, &v) && v == 900, "层3 魔攻回到 900");
    }

    /* 7) 逐字节核对：guard 被动过 → 拒绝写入（宁失效不崩） */
    {
        uint64_t a = d0 + OFF_STAT_ATK_PHYS + 4;
        uint32_t before;
        fake_reset_tables();
        fake_build();
        memset(&sctx, 0, sizeof(sctx));
        before = raw32((const unsigned char *)(uintptr_t)(d0 + OFF_STAT_ATK_PHYS));
        fput32(a, 0xdeadbeefu); /* 只破坏层0 的 guard */
        stat_reconcile(&snap, OFF_STAT_ATK_PHYS, 1000, &sctx);
        CHECK_EQ_I64(sctx.applied, 0, "guard 不符 → 不写入");
        CHECK(sctx.failed > 0, "guard 不符 → 记失败");
        CHECK_EQ_U64(raw32((const unsigned char *)(uintptr_t)(d0 + OFF_STAT_ATK_PHYS)), before,
                     "guard 不符时 stored 没被动过");
    }
    /* 8) 倍率溢出：原值 0x7ffffff0 × 100000 / 100 > MaxInt32 → 拒绝，且不改内存 */
    {
        unsigned char pair[8];
        uint64_t a = d0 + OFF_STAT_ATK_PHYS;
        uint32_t before;
        fake_reset_tables();
        fake_build();
        memset(&sctx, 0, sizeof(sctx));
        encode_native_int_pair(pair, 0x7ffffff0u);
        memcpy((void *)(uintptr_t)a, pair, 8);
        before = raw32((const unsigned char *)(uintptr_t)a);
        stat_reconcile(&snap, OFF_STAT_ATK_PHYS, 100000, &sctx);
        CHECK_EQ_I64(sctx.applied, 0, "溢出 → 不写入");
        CHECK(sctx.failed > 0, "溢出 → 记失败");
        CHECK_EQ_U64(raw32((const unsigned char *)(uintptr_t)a), before, "溢出时内存没被动过");
        fake_reset_tables();
        fake_build();
    }
    /* 9) 身份链任何一条不符 → 拒绝 */
    {
        MonsterId bad = id;
        bad.key = 0x30002u; /* 与现场 node+0x10 不符 */
        CHECK(!identity_valid(&bad), "key 不符 → identity_valid 拒绝");
        bad = id;
        bad.actor = id.actor + 8; /* node+0x28 != actor+0x30 */
        CHECK(!identity_valid(&bad), "actor 引用不符 → identity_valid 拒绝");
        bad = id;
        bad.vtable = id.vtable + 8;
        CHECK(!identity_valid(&bad), "*actor != vtable → identity_valid 拒绝");
    }

    VirtualFree((LPVOID)(uintptr_t)g_fake, 0, MEM_RELEASE);
    g_fake = 0;
}

/* ================================================================== */
/* status.json：必须是**合法 JSON**（实机 bug：有副本 id 时落盘成            */
/* `"dungeonId": ,`，整份文件解析不了）                                     */
/* ================================================================== */
static void test_status_json(void) {
    printf("\n-- difficulty-rules.status.json 内容合法性\n");
    char buf[2048];
    size_t n;
    FILE *f;
    wchar_t saved_dir[MAX_PATH];
    FILE *saved_log = g_log;
    TickInfo saved_info = g_info;

    wcscpy_s(saved_dir, MAX_PATH, g_dir);
    wcscpy_s(g_dir, MAX_PATH, L".");
    g_log = stderr;

    memset(&g_info, 0, sizeof(g_info));
    g_info.has_dungeon = 1;
    g_info.dungeon_id = 123456789u;
    g_info.matched = 1;
    _snprintf_s(g_info.rule_id, sizeof(g_info.rule_id), _TRUNCATE, "all-10x");
    g_info.percent = 1000;
    g_info.attack_percent = 1000;
    g_info.monsters = 7;
    g_info.applied = 7;
    g_info.tracked = 7;
    g_info.attack_applied = 14;
    g_info.attack_failed = 0;
    g_info.attack_tracked = 14;
    update_info();

    buf[0] = 0;
    f = fopen("difficulty-rules.status.json", "rb");
    if (!f) {
        g_fail++;
        printf("  [FAIL] status.json 没写出来\n");
    } else {
        n = fread(buf, 1, sizeof(buf) - 1, f);
        buf[n] = 0;
        fclose(f);
        CHECK(strstr(buf, "\"dungeonId\": 123456789,") != NULL,
              "有副本 id 时写的是数字（旧版是 `\"dungeonId\": ,`）");
        CHECK(strstr(buf, "\": ,") == NULL, "没有 `\": ,` 这种非法片段");
        CHECK(strstr(buf, "\"attackPercent\": 1000") != NULL, "attackPercent 落盘");
        CHECK(strstr(buf, "\"attackApplied\": 14") != NULL, "attackApplied 落盘");
        /* 粗略结构校验：括号配平 + 没有空值 */
        {
            int depth = 0, ok = 1;
            const char *p;
            for (p = buf; *p; p++) {
                if (*p == '{') depth++;
                else if (*p == '}') depth--;
            }
            if (depth != 0) ok = 0;
            CHECK(ok, "status.json 大括号配平");
        }
    }
    /* 取不到副本 id 时必须写 null，而不是空 */
    memset(&g_info, 0, sizeof(g_info));
    g_info.has_dungeon = 0;
    update_info();
    buf[0] = 0;
    f = fopen("difficulty-rules.status.json", "rb");
    if (f) {
        n = fread(buf, 1, sizeof(buf) - 1, f);
        buf[n] = 0;
        fclose(f);
    }
    CHECK(strstr(buf, "\"dungeonId\": null,") != NULL, "取不到副本 id → null");

    remove("difficulty-rules.status.json");
    wcscpy_s(g_dir, MAX_PATH, saved_dir);
    g_log = saved_log;
    g_info = saved_info;
}

/* ================================================================== */
/* 单条规则的 dungeonIds 容量：256 项通过、257 项整流拒绝（不用一半）      */
/* ================================================================== */

/* 造一份"单条规则带 n 个副本 id"的 rules 文本（id 依次 1..n）。返回写入长度。 */
static int build_rule_with_ids(char *buf, size_t cap, int n, unsigned percent) {
    int i, w = 0;
    w += _snprintf_s(buf + w, cap - (size_t)w, _TRUNCATE,
                     "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"cap\",\"enabled\":true,"
                     "\"dungeonIds\":[");
    for (i = 0; i < n; i++)
        w += _snprintf_s(buf + w, cap - (size_t)w, _TRUNCATE, "%s%u", i ? "," : "",
                         (unsigned)(i + 1));
    w += _snprintf_s(buf + w, cap - (size_t)w, _TRUNCATE, "],\"percent\":%u}]}", percent);
    return w;
}

static void test_rule_ids_capacity(void) {
    char why[256];
    char *buf = (char *)malloc(1u << 16);

    printf("\n-- 单条规则的 dungeonIds 容量（MAX_RULE_IDS=%d）\n", MAX_RULE_IDS);
    if (!buf) {
        g_fail++;
        printf("  [FAIL] 造不出测试缓冲\n");
        return;
    }

    /* 0) 内存实测：容量由**堆**数组承载，RuleSet 本身仍很小（进栈安全）。
     *    这条断言是给"抬容量会不会再把栈撑爆"兜底的 —— 旧版把 Rule 内嵌进 RuleSet 时
     *    工作线程栈直接 0xC00000FD。 */
    {
        size_t rule_sz = sizeof(Rule), file_sz = sizeof(RuleFile), set_sz = sizeof(RuleSet);
        size_t worst_heap = (size_t)MAX_RULE_FILES * MAX_RULES * rule_sz;
        printf("  [info] sizeof(Rule)=%lluB sizeof(RuleFile)=%lluB sizeof(RuleSet)=%lluB "
               "最坏堆=%lluB（%d 份 × %d 条 × Rule）\n",
               (unsigned long long)rule_sz, (unsigned long long)file_sz,
               (unsigned long long)set_sz, (unsigned long long)worst_heap, MAX_RULE_FILES,
               MAX_RULES);
        CHECK(set_sz < 8u * 1024u, "RuleSet 仍是几 KiB（抬容量不改栈占用）");
        CHECK(set_sz < rule_sz * MAX_RULES,
              "RuleSet 不内嵌 Rule 数组（只存指针）—— 内嵌就是几十 KiB 的栈对象");
    }

    /* 1) 刚好 256 项：通过，且 256 项一个不少、顺序不变 */
    {
        RuleSet set;
        int n = build_rule_with_ids(buf, 1u << 16, MAX_RULE_IDS, 1000);
        memset(&set, 0, sizeof(set));
        CHECK(parse_rules(buf, (size_t)n, &set, why, sizeof(why)), "256 个 dungeonIds → 通过");
        CHECK_EQ_I64(PARSED_RULES(set)[0].id_count, MAX_RULE_IDS, "id_count = 256");
        CHECK_EQ_I64(PARSED_RULES(set)[0].id_overflow, 0, "没有越界标记");
        CHECK_EQ_U64(PARSED_RULES(set)[0].ids[0], 1, "第一个 id 保留");
        CHECK_EQ_U64(PARSED_RULES(set)[0].ids[MAX_RULE_IDS - 1], (unsigned)MAX_RULE_IDS,
                     "最后一个 id 保留（没被截断到前 64 项）");
        ruleset_clear(&set);
    }

    /* 2) 257 项：**整流拒绝**，绝不"只取前 256 项"。 */
    {
        RuleSet set;
        int n = build_rule_with_ids(buf, 1u << 16, MAX_RULE_IDS + 1, 1000);
        memset(&set, 0, sizeof(set));
        CHECK(!parse_rules(buf, (size_t)n, &set, why, sizeof(why)),
              "257 个 dungeonIds → 拒绝（上限 256）");
        printf("  [info] 拒绝原因：%s\n", why);
        CHECK(strstr(why, "dungeonIds") != NULL, "拒绝原因点名单条规则的容量");
        ruleset_clear(&set);
    }

    /* 3) 端到端（rules.d）：超容那份**整份不可用**（一个 id 都不进 set），
     *    同目录里另一份好文件照常生效 —— 证明"不用一半"，也证明坏文件只跳过自己。 */
    {
        RuleSet set;
        int n_bad = build_rule_with_ids(buf, 1u << 16, MAX_RULE_IDS + 1, 1000);
        char *good = (char *)malloc(512);
        tmp_setup();
        WITH_TMP_DIR_BEGIN();
        tmp_write_rulesdir(L"cap-bad.json", buf);
        CHECK(n_bad > 0, "超容文件已写出");
        if (good) {
            _snprintf_s(good, 512, _TRUNCATE,
                        "{\"schema\":1,\"enabled\":true,\"rules\":[{\"id\":\"cap-ok\","
                        "\"enabled\":true,\"dungeonIds\":[100004934],\"percent\":1000}]}");
            tmp_write_rulesdir(L"cap-ok.json", good);
            free(good);
        }
        memset(&set, 0, sizeof(set));
        CHECK_EQ_I64(selftest_load_rules_dir(&set), 1, "rules.d 加载成功（坏文件只跳过自己）");
        CHECK_EQ_I64(set.rule_file_count, 1, "超容那份整份不可用：只有 1 份文件进 set");
        CHECK(set.rule_file_count == 1 && strcmp(set.files[0].name, "cap-ok.json") == 0,
              "进 set 的是 cap-ok.json（超容的 cap-bad.json 一个 id 都没生效）");
        ruleset_clear(&set);
        tmp_teardown();
        WITH_TMP_DIR_END();
    }

    free(buf);
}

int main(void) {
    printf("DifficultyRules 纯逻辑自测（不启动游戏、不读写客户端内存）\n");
    test_attr_crypto();
    test_native_float();
    test_scale_hp();
    test_maxima_and_rescale();
    test_native_int();
    test_hp_new_maxima();
    test_rules_json();
    test_attack_percent_json();
    test_rule_match();
    test_rules_dir_multi();
    test_rule_ids_capacity();
    test_fake_memory_apply();
    test_status_json();
    printf("\n结果：通过 %d，失败 %d\n", g_pass, g_fail);
    return g_fail == 0 ? 0 : 1;
}

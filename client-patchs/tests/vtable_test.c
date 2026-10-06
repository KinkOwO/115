/* 虚表槽替换机制自测：造一个假控件对象（vtable 里有"取文本"槽），
 * 把槽换成包装，调用包装与其它实例，验证：
 *   - 我们盯住的实例返回被替换的值
 *   - 其它实例仍然拿到原值（包装必须按实例判定，否则会波及其它输入框）*/
#define WIN32_LEAN_AND_MEAN
#include <windows.h>
#include <stdio.h>
#include <string.h>

typedef const wchar_t *(__fastcall *GetTextFn)(void *self);

static GetTextFn g_orig;
static void *g_watched;
static const wchar_t *g_phrase = L"delete character";

static const wchar_t *__fastcall real_get_text(void *self) {
    return (const wchar_t *)((void **)self)[1]; /* 对象里第二格放着本实例的文本指针 */
}

static const wchar_t *__fastcall hook_get_text(void *self) {
    if (self == g_watched) return g_phrase;
    return g_orig(self);
}

int main(void) {
    /* 两个假实例、两张（其实共用一张）虚表 */
    static void *vt[4];
    vt[0] = (void *)real_get_text;
    const wchar_t *textA = L"aaaa", *textB = L"bbbb";
    void *objA[2] = {(void *)vt, (void *)textA};
    void *objB[2] = {(void *)vt, (void *)textB};

    GetTextFn call = (GetTextFn)vt[0];
    printf("打补丁前：A=%ls B=%ls\n", call(objA), call(objB));
    if (wcscmp(call(objA), L"aaaa") != 0 || wcscmp(call(objB), L"bbbb") != 0) {
        printf("FAIL: 基准值不对\n");
        return 1;
    }

    g_watched = objA;
    g_orig = (GetTextFn)vt[0];
    DWORD old = 0;
    VirtualProtect(&vt[0], sizeof(void *), PAGE_READWRITE, &old);
    vt[0] = (void *)hook_get_text;
    DWORD tmp = 0;
    VirtualProtect(&vt[0], sizeof(void *), old, &tmp);

    call = (GetTextFn)vt[0];
    const wchar_t *ra = call(objA), *rb = call(objB);
    printf("打补丁后：A=%ls（应为替换值） B=%ls（应保持原值）\n", ra, rb);
    if (wcscmp(ra, g_phrase) != 0) {
        printf("FAIL: 盯住的实例没有返回替换值\n");
        return 1;
    }
    if (wcscmp(rb, L"bbbb") != 0) {
        printf("FAIL: 其它实例被波及了\n");
        return 1;
    }
    printf("PASS：虚表槽替换按实例生效，不影响其它实例\n");
    return 0;
}

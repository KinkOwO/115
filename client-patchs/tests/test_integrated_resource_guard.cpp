#include "../local-features/src/local_features.cpp"
#include <cstdio>
int wmain(int argc, wchar_t** argv) {
    if (argc != 2) return 2;
    bool expected = wcscmp(argv[1], L"accept") == 0;
    bool actual = native_inventory::resourcesMatch();
    std::printf("resourcesMatch=%s expected=%s\n", actual ? "accept" : "reject", expected ? "accept" : "reject");
    return actual == expected ? 0 : 1;
}

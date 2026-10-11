#pragma once
#include <windows.h>

// Both source loading (N1382) and AI cloning (N1879) construct pets.
// Restore the caller's thread-local depth even when native SEH propagates.
static void RunCompanionLoad(LONG& depth,void(__fastcall* original)()) {
    ++depth;
    __try { original(); }
    __finally { --depth; }
}

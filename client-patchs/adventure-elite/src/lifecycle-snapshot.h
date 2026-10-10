#pragma once
#include <stdint.h>
#include <stddef.h>

// Current native manager stores (control, actor+48, auxiliary) at +72+24*i.
// Read-only observations never call getters, acquire strong refs or retain actors.
using EliteSnapshotRead = bool (*)(const void*, void*, size_t);
struct EliteCompanionSnapshot {
    bool available = false, consistent = false;
    int alive = 0;
    int strong[3] = {-1,-1,-1}, kind[3] = {-1,-1,-1};
    int controllerId[3] = {-1,-1,-1}, controllerWire[3] = {-1,-1,-1};
    int controllerBound[3] = {-1,-1,-1};
    // Identity +356/+360 is separate from controller +74020.
    uint32_t identityWords[3][2] = {};
    int identityValid[3] = {-1,-1,-1}, sceneAttached[3] = {-1,-1,-1};
    int64_t objectId[3] = {-1,-1,-1}, ownerObjectId[3] = {-1,-1,-1};
    int ownerStrong[3] = {-1,-1,-1}, ownerKind[3] = {-1,-1,-1};
    int ownerControllerId[3] = {-1,-1,-1};
    int ownerIdentityValid[3] = {-1,-1,-1}, ownerMatchesCurrentPlayer[3] = {-1,-1,-1};
};
// Mirrors only the exact integer transform in 146E920A0 and checksum in
// 145B8C670. Invalid/zero checksums stay diagnostic, never become identities.
inline bool EliteSnapshotIdentity(uint32_t encoded, uint32_t checksum, uint32_t* value) {
    const uint32_t decoded = (encoded ^ UINT32_C(0x1F2A025C)) - UINT32_C(4);
    if (!checksum || checksum != encoded + decoded + UINT32_C(196)) return false;
    *value = decoded;
    return true;
}
inline EliteCompanionSnapshot EliteReadCompanions(uintptr_t manager, EliteSnapshotRead read, const uintptr_t* currentPlayer = nullptr) {
    EliteCompanionSnapshot s;
    if (!manager || !read) return s;
    s.available = true; s.consistent = true;
    for (size_t i = 0; i < 3; ++i) {
        const auto slot = reinterpret_cast<const void*>(manager+72+24*i);
        uintptr_t weak[2] = {}, after[2] = {};
        if (!read(slot, weak, sizeof(weak))) { s.consistent = false; continue; }
        if (!weak[0]) { s.strong[i] = 0; continue; }
        int strong = 0, strongAfter = 0;
        if (!read(reinterpret_cast<void*>(weak[0]+8), &strong, sizeof(strong))) { s.consistent = false; continue; }
        s.strong[i] = strong;
        if (strong <= 0) continue;
        if (weak[1] < 48) { s.consistent = false; continue; }
        const auto actor = weak[1]-48;
        uintptr_t context = 0, controller = 0;
        uint32_t identityAfter[2] = {}, decoded = 0;
        uintptr_t ownerRef[3] = {}, ownerAfter[3] = {}, sceneRef[2] = {}, sceneAfter[2] = {};
        bool extra = read(reinterpret_cast<void*>(actor+356), s.identityWords[i], sizeof(s.identityWords[i])) &&
            read(reinterpret_cast<void*>(actor+25264), ownerRef, sizeof(ownerRef)) &&
            read(reinterpret_cast<void*>(actor+400), sceneRef, sizeof(sceneRef));
        int ownerStrongAfter = 0, sceneStrong = 0, sceneStrongAfter = 0;
        uint32_t ownerIdentity[2] = {}, ownerIdentityAfter[2] = {};
        if (extra) {
            s.identityValid[i] = EliteSnapshotIdentity(s.identityWords[i][0],s.identityWords[i][1],&decoded) ? 1 : 0;
            if (s.identityValid[i]) s.objectId[i] = decoded;
            s.ownerStrong[i] = 0;
            if (ownerRef[1]) {
                extra = read(reinterpret_cast<void*>(ownerRef[1]+8), &s.ownerStrong[i], sizeof(int));
                if (extra && s.ownerStrong[i] > 0) {
                    if (currentPlayer && currentPlayer[0] && currentPlayer[1])
                        s.ownerMatchesCurrentPlayer[i] = ownerRef[1] == currentPlayer[0] && ownerRef[2] == currentPlayer[1] ? 1 : 0;
                    extra = ownerRef[2] >= 48 &&
                        read(reinterpret_cast<void*>(ownerRef[2]-48+356),ownerIdentity,sizeof(ownerIdentity)) &&
                        read(reinterpret_cast<void*>(ownerRef[2]-48+352),&s.ownerKind[i],sizeof(int)) &&
                        read(reinterpret_cast<void*>(ownerRef[2]-48+74020),&s.ownerControllerId[i],sizeof(int));
                    if (extra) {
                        s.ownerIdentityValid[i] = EliteSnapshotIdentity(ownerIdentity[0],ownerIdentity[1],&decoded) ? 1 : 0;
                        if (s.ownerIdentityValid[i]) s.ownerObjectId[i] = decoded;
                    }
                }
            }
            s.sceneAttached[i] = 0;
            if (extra && sceneRef[0]) {
                extra = read(reinterpret_cast<void*>(sceneRef[0]+8),&sceneStrong,sizeof(sceneStrong));
                if (extra && sceneStrong > 0 && sceneRef[1]) s.sceneAttached[i] = 1;
            }
        }
        unsigned short wire = 0;
        bool fields = read(reinterpret_cast<void*>(actor+352), &s.kind[i], sizeof(int)) &&
            read(reinterpret_cast<void*>(actor+74020), &s.controllerId[i], sizeof(int)) &&
            read(reinterpret_cast<void*>(actor+100888), &context, sizeof(context)) && context &&
            read(reinterpret_cast<void*>(context+5424), &controller, sizeof(controller)) && controller &&
            read(reinterpret_cast<void*>(controller+112), &wire, sizeof(wire));
        // Controller +2072 is a three-word CRef: (referent, control, alias).
        // 146EA47F0 stores actor+48 in the third word, not the first word.
        uintptr_t binding[3] = {}; int boundStrong = 0;
        if (fields && read(reinterpret_cast<void*>(controller+2072), binding, sizeof(binding)) && binding[1] &&
            read(reinterpret_cast<void*>(binding[1]+8), &boundStrong, sizeof(boundStrong)))
            s.controllerBound[i] = binding[1] == weak[0] && binding[2] == weak[1] && boundStrong > 0 ? 1 : 0;
        else fields = false;
        if (!extra || !read(reinterpret_cast<void*>(actor+356),identityAfter,sizeof(identityAfter)) ||
            identityAfter[0] != s.identityWords[i][0] || identityAfter[1] != s.identityWords[i][1] ||
            !read(reinterpret_cast<void*>(actor+25264),ownerAfter,sizeof(ownerAfter)) ||
            ownerAfter[0] != ownerRef[0] || ownerAfter[1] != ownerRef[1] || ownerAfter[2] != ownerRef[2] ||
            !read(reinterpret_cast<void*>(actor+400),sceneAfter,sizeof(sceneAfter)) ||
            sceneAfter[0] != sceneRef[0] || sceneAfter[1] != sceneRef[1] ||
            (s.ownerStrong[i] > 0 && (!read(reinterpret_cast<void*>(ownerRef[1]+8),&ownerStrongAfter,sizeof(int)) || ownerStrongAfter <= 0 ||
                !read(reinterpret_cast<void*>(ownerRef[2]-48+356),ownerIdentityAfter,sizeof(ownerIdentityAfter)) ||
                ownerIdentityAfter[0] != ownerIdentity[0] || ownerIdentityAfter[1] != ownerIdentity[1])) ||
            (sceneStrong > 0 && (!read(reinterpret_cast<void*>(sceneRef[0]+8),&sceneStrongAfter,sizeof(int)) || sceneStrongAfter <= 0))) {
            s.consistent = false;
        }
        if (!read(slot, after, sizeof(after)) || weak[0] != after[0] || weak[1] != after[1] ||
            !read(reinterpret_cast<void*>(weak[0]+8), &strongAfter, sizeof(strongAfter)) || strongAfter <= 0) {
            s.consistent = false; continue;
        }
        ++s.alive;
        if (fields) s.controllerWire[i] = wire;
        else s.consistent = false;
    }
    return s;
}

#pragma once
#include <stdint.h>
struct EliteRegistrationFacts {
    bool ready, identity, world, dungeon, scene, stable, nativePlayer, references, notRegistered, managerIdle;
    unsigned int alive;
};
inline bool EliteRegistrationAllowed(const EliteRegistrationFacts& f) {
    return f.ready && f.identity && f.world && f.dungeon && f.scene && f.stable &&
        f.nativePlayer && f.references && f.notRegistered && f.managerIdle && f.alive>0 && f.alive<=3;
}

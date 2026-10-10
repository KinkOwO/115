#pragma once
#include "lifecycle-snapshot.h"
#include <string.h>
// Diagnostic fields from 145DD0070 -> 145C3E8A0 -> 145DC86D0.
// No virtual getters, refcount changes, native writes or retained actor handles.
struct EliteCombatSnapshot {
    bool readable=false, stable=false, unique=false, bound=false;
    uintptr_t victim=0, sourceRef[3]={}, playerRef[2]={};
    int victimKind=-1, sourceKind=-1, sourceStrong=0, slot=-1;
    int sourceController=-1, ownerMatches=-1, ownerController=-1, controllerWire=-1;
    int64_t victimId=-1, sourceId=-1, ownerId=-1;
    uint32_t killer=0, sourceField=0;
    uintptr_t combatGetter=0;
};
inline EliteCombatSnapshot EliteReadCombat(uintptr_t victim,uintptr_t manager,const uintptr_t* player,EliteSnapshotRead read) {
    EliteCombatSnapshot s; s.victim=victim;
    if(!victim || !read)return s;
    uint32_t words[2]={},afterWords[2]={},decoded=0,afterKiller=0,afterSource=0;
    uintptr_t afterRef[3]={};
    s.readable=read(reinterpret_cast<void*>(victim+352),&s.victimKind,4) &&
        read(reinterpret_cast<void*>(victim+356),words,8) &&
        read(reinterpret_cast<void*>(victim+26784),&s.killer,4) &&
        read(reinterpret_cast<void*>(victim+35076),&s.sourceField,4) &&
        read(reinterpret_cast<void*>(victim+29488),s.sourceRef,24);
    if(!s.readable)return s;
    if(EliteSnapshotIdentity(words[0],words[1],&decoded))s.victimId=decoded;
    if(player)memcpy(s.playerRef,player,sizeof(s.playerRef));
    bool sourceStable=true;
    if(s.sourceRef[1]) {
        sourceStable=read(reinterpret_cast<void*>(s.sourceRef[1]+8),&s.sourceStrong,4);
        if(sourceStable && s.sourceStrong>0 && s.sourceRef[2]>=48) {
            uintptr_t actor=s.sourceRef[2]-48,vtable=0;
            uint32_t sourceWords[2]={},afterSourceWords[2]={}; int afterStrong=0,afterController=0;
            sourceStable=read(reinterpret_cast<void*>(actor+352),&s.sourceKind,4) &&
                read(reinterpret_cast<void*>(actor+356),sourceWords,8) &&
                read(reinterpret_cast<void*>(actor+74020),&s.sourceController,4) &&
                read(reinterpret_cast<void*>(actor),&vtable,8) && vtable &&
                read(reinterpret_cast<void*>(vtable+1984),&s.combatGetter,8);
            if(sourceStable && EliteSnapshotIdentity(sourceWords[0],sourceWords[1],&decoded))s.sourceId=decoded;
            auto companions=EliteReadCompanions(manager,read,player);
            int referenceMatches=0,identityMatches=0;
            if(companions.available && companions.consistent) {
                for(size_t i=0;i<3;++i) {
                    if(companions.strong[i]>0 && companions.objectId[i]==s.sourceId)++identityMatches;
                    uintptr_t weak[2]={},weakAfter[2]={};
                    if(read(reinterpret_cast<void*>(manager+72+24*i),weak,16) &&
                        weak[0]==s.sourceRef[1] && weak[1]==s.sourceRef[2] &&
                        companions.strong[i]>0 && companions.identityValid[i]==1 && companions.kind[i]==5 &&
                        companions.objectId[i]==s.sourceId &&
                        read(reinterpret_cast<void*>(manager+72+24*i),weakAfter,16) && !memcmp(weak,weakAfter,16)) {
                        ++referenceMatches;s.bound=companions.controllerBound[i]==1; s.controllerWire=companions.controllerWire[i];
                        s.slot=static_cast<int>(i);s.ownerId=companions.ownerObjectId[i];
                        s.ownerController=companions.ownerControllerId[i];s.ownerMatches=companions.ownerMatchesCurrentPlayer[i];
                    }
                }
            }
            s.unique=referenceMatches==1 && identityMatches==1;
            sourceStable=sourceStable && read(reinterpret_cast<void*>(actor+356),afterSourceWords,8) &&
                !memcmp(sourceWords,afterSourceWords,8) && read(reinterpret_cast<void*>(actor+74020),&afterController,4) &&
                afterController==s.sourceController && read(reinterpret_cast<void*>(s.sourceRef[1]+8),&afterStrong,4) && afterStrong>0;
        } else if(s.sourceStrong>0)sourceStable=false;
    }
    s.stable=sourceStable && read(reinterpret_cast<void*>(victim+356),afterWords,8) && !memcmp(words,afterWords,8) &&
        read(reinterpret_cast<void*>(victim+26784),&afterKiller,4) && afterKiller==s.killer &&
        read(reinterpret_cast<void*>(victim+35076),&afterSource,4) && afterSource==s.sourceField &&
        read(reinterpret_cast<void*>(victim+29488),afterRef,24) && !memcmp(s.sourceRef,afterRef,24);
    return s;
}

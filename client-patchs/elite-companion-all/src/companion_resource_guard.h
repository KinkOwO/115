#pragma once
#include <stdint.h>
enum class PreloadResult { Native, MissingManager, MissingResource };
// Applies only while original N1382 source loading or N1879 AI cloning runs
// in the authorized pilot. Never substitutes a resource, item or statistic.
template<class Lookup,class Original>
PreloadResult CompanionPreload(bool active,void* manager,uint32_t id,
                               Lookup lookup,Original original,void* self) {
    if(active) {
        if(!manager)return PreloadResult::MissingManager;
        if(!lookup(manager,id,true))return PreloadResult::MissingResource;
    }
    original(self,id);
    return PreloadResult::Native;
}

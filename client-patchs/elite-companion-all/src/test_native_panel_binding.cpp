#include "native_panel_binding.h"
#include <assert.h>
#include <stdio.h>
int main() {
    // First-login metadata arrives after the window, then one role at a time.
    PanelBindingSnapshot prior={},next={};
    next.window=1;next.roles=2;next.config=3;next.slots[0]=4;
    assert(PanelBindingChanged(prior,next));prior=next;
    assert(!PanelBindingChanged(prior,next));
    next.sources[0]=40;assert(PanelBindingChanged(prior,next));prior=next;
    assert(!PanelBindingChanged(prior,next));
    next.slots[0]=6;next.sources[0]=60;
    assert(PanelBindingChanged(prior,next));prior=next;
    next.clones[0]=600;assert(PanelBindingChanged(prior,next));prior=next;
    next.window=10;assert(PanelBindingChanged(prior,next));
    // r5 accidentally compared source with instance. Distinct identities are
    // required to match their corresponding values, never each other.
    assert(PilotRoomIdentity(true,0x1000,0x2000,0x1000,0x2000));
    assert(!PilotRoomIdentity(true,0x1000,0x2000,0x1000,0x1000));
    assert(!PilotRoomIdentity(true,0x3000,0x2000,0x1000,0x2000));
    assert(!PilotRoomIdentity(true,0x1000,0x4000,0x1000,0x2000));
    assert(!PilotRoomIdentity(false,0x1000,0x2000,0x1000,0x2000));
    assert(!PilotRoomIdentity(true,0,0,0,0));
    puts("PASS native panel late-data/selection/clone/window transitions and distinct room instance/source identity guards");
}

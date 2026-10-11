#pragma once
#include <cstdint>
#include <string>
#include <utility>

// Local extension UI state only. Slot counts, item eligibility and character
// effects belong to the server's source-bound view, never to this controller.
namespace symbol_page {
struct Identity {
    uint64_t role=0, connection=0;
    std::string source;
    bool operator==(const Identity& x)const {
        return role==x.role && connection==x.connection && source==x.source;
    }
};
struct Ticket {
    Identity identity;
    uint64_t generation=0, sequence=0, request=0;
    std::string group;
    uint16_t socket=0;
};
class State {
    Identity identity_;
    uint64_t generation_=0, sequence_=0, pending_=0, request_=0;
    bool active_=false, ready_=false, uncertain_=false;
    std::string group_;
    uint16_t socket_=0, count_=0;
public:
    void close() {
        ++generation_;
        active_=ready_=uncertain_=false;
        pending_=0; count_=socket_=0; group_.clear(); identity_={};
    }
    void enter(Identity identity) {
        close(); identity_=std::move(identity);
        active_=identity_.role && identity_.connection && identity_.source.size()==64;
    }
    uint64_t generation()const{return generation_;}
    bool ready()const{return active_ && ready_ && !pending_ && !uncertain_;}
    bool panelOpen()const{return active_ && !group_.empty();}
    bool snapshot(const Identity& identity,uint64_t generation,uint64_t sequence) {
        if(!active_ || !(identity==identity_) || generation!=generation_ || pending_)return false;
        if(ready_ && sequence<sequence_)return false;
        sequence_=sequence;ready_=true;uncertain_=false;return true;
    }
    bool openGroup(std::string group,uint16_t sourceCount) {
        if(!ready() || group.empty() || !sourceCount)return false;
        group_=std::move(group);count_=sourceCount;socket_=0;return true;
    }
    bool selectSocket(uint16_t socket) {
        if(!ready() || !panelOpen() || socket>=count_)return false;
        socket_=socket;return true;
    }
    void closePanel() {group_.clear();count_=socket_=0;}
    bool begin(Ticket& ticket) {
        if(!ready() || !panelOpen() || sequence_==UINT64_MAX || request_==UINT64_MAX)return false;
        pending_=++request_;
        ticket={identity_,generation_,sequence_,pending_,group_,socket_};return true;
    }
    bool accept(const Ticket& ticket,uint64_t savedSequence) {
        if(!active_ || !(ticket.identity==identity_) || ticket.generation!=generation_ ||
           !pending_ || ticket.request!=pending_ || ticket.sequence!=sequence_ ||
           ticket.sequence==UINT64_MAX || savedSequence!=ticket.sequence+1)return false;
        pending_=0;sequence_=savedSequence;return true;
    }
    // A timeout may hide a committed operation. Do not restore an optimistic
    // inventory or resend a new operation until an authoritative refresh.
    bool fail(const Ticket& ticket) {
        if(!(ticket.identity==identity_) || ticket.generation!=generation_ ||
           !pending_ || ticket.request!=pending_)return false;
        pending_=0;uncertain_=true;return true;
    }
};
}

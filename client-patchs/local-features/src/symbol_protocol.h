#pragma once
#include "protocol.h"
#include <set>

namespace symbol_wire {
struct Item {uint32_t id=0,expiry=0,flags=0;};
struct Star {bool present=false;uint32_t grade=0,level=0,fame=0;std::string fixed[2],resonance[2],perEmblem[4][2];};
struct Group {std::string name;uint16_t mask=0;std::vector<Item> items;std::string attributes;Star star;};
struct BagItem {uint16_t slot=0,mask=0;uint32_t id=0,amount=0,expiry=0;};
struct View {
    uint64_t role=0,sequence=0;std::string source;
    std::vector<Group> groups;std::vector<BagItem> bag;
    std::string attributes,buffAttributes;
    std::vector<std::pair<uint32_t,uint32_t>> powers;
    const Group* find(const std::string& name)const {for(auto&g:groups)if(g.name==name)return &g;return nullptr;}
};
inline bool groupName(const std::string&s){if(s.empty()||s.size()>64)return false;for(char c:s)if(c!='_'&&(c<'a'||c>'z'))return false;return true;}
inline View decode(const std::vector<uint8_t>&p){
    features::Reader r{p};View v;v.role=r.num(8);v.sequence=r.num(8);
    for(int i=0;i<64;++i)v.source.push_back(char(r.num(1)));
    auto groups=r.num(2),rows=r.num(2);
    if(!v.role||!features::hex(v.source,64)||groups==0||groups>16||rows>1024)throw std::runtime_error("invalid symbol view");
    std::set<std::string> seen;
    for(size_t i=0;i<groups;++i){
        Group g;g.name=r.text();g.mask=uint16_t(r.num(2));auto n=r.num(2);
        if(!groupName(g.name)||!seen.insert(g.name).second||!g.mask||!n||n>256)throw std::runtime_error("invalid symbol group");
        for(size_t j=0;j<n;++j){Item x{uint32_t(r.num(4)),uint32_t(r.num(4)),uint32_t(r.num(4))};if(x.id==1||(x.flags&~0xffff01u)||(!x.id&&(x.expiry||x.flags)))throw std::runtime_error("invalid socket item");g.items.push_back(x);}
        v.groups.push_back(std::move(g));
    }
    std::set<uint16_t> slots;
    for(size_t i=0;i<rows;++i){BagItem x{uint16_t(r.num(2)),uint16_t(r.num(2)),uint32_t(r.num(4)),uint32_t(r.num(4)),uint32_t(r.num(4))};
        if(!slots.insert(x.slot).second||!x.mask||x.id<2||!x.amount)throw std::runtime_error("invalid emblem inventory");v.bag.push_back(x);}
    if(r.at<p.size()){
        if(r.num(4)!=0x31544153)throw std::runtime_error("invalid attribute trailer");
        v.attributes=r.text();v.buffAttributes=r.text();for(auto&g:v.groups)g.attributes=r.text();
    }
    if(r.at<p.size()){
        if(r.num(4)!=0x31525353)throw std::runtime_error("invalid star trailer");
        auto n=r.num(2);if(n>64)throw std::runtime_error("too many star options");
        for(size_t i=0;i<n;++i){auto a=uint32_t(r.num(4)),b=uint32_t(r.num(4));if(a<2||b<2)throw std::runtime_error("invalid star option");v.powers.push_back({a,b});}
        for(auto&g:v.groups){auto present=r.num(1);if(present>1)throw std::runtime_error("invalid star state");g.star.present=present!=0;if(!present)continue;
            auto&s=g.star;s.grade=uint32_t(r.num(4));s.level=uint32_t(r.num(4));s.fame=uint32_t(r.num(4));
            if(s.grade>3||!s.level||s.level>5)throw std::runtime_error("invalid star progress");
            for(auto&x:s.fixed)x=r.text();for(auto&x:s.resonance)x=r.text();for(auto&pair:s.perEmblem)for(auto&x:pair)x=r.text();
        }
    }
    r.end();return v;
}
inline std::vector<uint8_t> refresh(const View&v){
    if(!v.role||!features::hex(v.source,64))throw std::runtime_error("invalid symbol refresh");
    std::vector<uint8_t> p;features::put(p,v.role,8);features::put(p,v.sequence,8);p.insert(p.end(),v.source.begin(),v.source.end());return p;
}
inline std::vector<uint8_t> exchange(const View&v,const std::string&name,uint16_t socket,const BagItem*incoming){
    auto g=v.find(name);
    if(!v.role||!features::hex(v.source,64)||!g||socket>=g->items.size()||v.sequence==UINT64_MAX||
       (incoming&&(!(incoming->mask&g->mask)||!incoming->amount||incoming->id<2)))throw std::runtime_error("invalid symbol exchange");
    std::vector<uint8_t> p;features::put(p,v.role,8);features::put(p,v.sequence,8);p.insert(p.end(),v.source.begin(),v.source.end());
    features::put(p,name.size(),2);p.insert(p.end(),name.begin(),name.end());features::put(p,socket,2);features::put(p,g->items[socket].id,4);
    features::put(p,incoming?incoming->slot:0,2);features::put(p,incoming?incoming->id:0,4);return p;
}
}

#pragma once
#include <cstdint>
#include <cstring>
#include <string>
#include <vector>
#include <stdexcept>

namespace features {
constexpr uint32_t MaxBody=60*1024;
inline bool utf8(const std::string&s){for(size_t i=0;i<s.size();){unsigned char a=s[i++];if(a<0x80){if(a==0)return false;continue;}int n=a>=0xc2&&a<=0xdf?1:a>=0xe0&&a<=0xef?2:a>=0xf0&&a<=0xf4?3:-1;if(n<0||size_t(n)>s.size()-i)return false;uint32_t v=a&((1u<<(6-n))-1);for(int k=0;k<n;k++){unsigned char b=s[i++];if((b&0xc0)!=0x80)return false;v=(v<<6)|(b&63);}if((n==1&&v<0x80)||(n==2&&v<0x800)||(n==3&&v<0x10000)||v>0x10ffff||(v>=0xd800&&v<=0xdfff))return false;}return true;}
struct Row { uint8_t group=0,state=0;uint32_t index=0,item=0,count=0;uint64_t progress=0,goal=0;std::string label; };
struct Campaign {uint8_t family=0,state=0;std::string title,detail,source,key;std::vector<Row> rows;};
struct View {uint64_t role=0;std::vector<Campaign> campaigns;};
struct Reader {
 const std::vector<uint8_t>&p;size_t at=0;
 uint64_t num(size_t n){if(n>8||at>p.size()||n>p.size()-at)throw std::runtime_error("truncated frame");uint64_t v=0;for(size_t i=0;i<n;i++)v|=uint64_t(p[at++])<<(i*8);return v;}
 std::string text(){size_t n=size_t(num(2));if(n>4096||at>p.size()||n>p.size()-at)throw std::runtime_error("invalid text length");std::string s(reinterpret_cast<const char*>(p.data()+at),n);at+=n;if(!utf8(s))throw std::runtime_error("invalid UTF8 text");return s;}
 void end(){if(at!=p.size())throw std::runtime_error("trailing data");}
};
inline bool hex(const std::string&s,size_t n){if(s.size()!=n)return false;for(char c:s)if(!((c>='0'&&c<='9')||(c>='a'&&c<='f')))return false;return true;}
inline void put(std::vector<uint8_t>&p,uint64_t v,size_t n){for(size_t i=0;i<n;i++)p.push_back(uint8_t(v>>(i*8)));}
inline std::vector<uint8_t> frame(uint16_t type,uint64_t id,const std::vector<uint8_t>&body={}){
 if(type<1||type>5||!id||body.size()>MaxBody)throw std::runtime_error("invalid request");std::vector<uint8_t>p={'U','1','1','5'};put(p,1,2);put(p,type,2);put(p,id,8);put(p,body.size(),4);put(p,0,4);p.insert(p.end(),body.begin(),body.end());return p;
}
inline uint32_t replyHeader(const std::vector<uint8_t>&p,uint16_t type,uint64_t id,uint32_t&status){
 if(p.size()!=24||memcmp(p.data(),"U115",4))throw std::runtime_error("bad reply magic");Reader r{p};r.num(4);if(r.num(2)!=1||r.num(2)!=(type|0x8000)||r.num(8)!=id)throw std::runtime_error("wrong reply generation or identity");auto n=uint32_t(r.num(4));status=uint32_t(r.num(4));if(n>MaxBody||status>4||(status!=0&&n!=0))throw std::runtime_error("invalid reply bounds");return n;
}
inline View view(const std::vector<uint8_t>&p){
 Reader r{p};View v;v.role=r.num(8);auto n=r.num(4);if(!v.role||n>16)throw std::runtime_error("invalid view");
 for(size_t i=0;i<n;i++){Campaign c;c.family=uint8_t(r.num(1));c.state=uint8_t(r.num(1));size_t rows=size_t(r.num(2));if(c.family<1||c.family>5||c.state>2||rows>128)throw std::runtime_error("invalid campaign");c.title=r.text();c.detail=r.text();c.source=r.text();c.key=r.text();if(c.state==0&&(!hex(c.source,64)||!hex(c.key,32)))throw std::runtime_error("unbound campaign");
  for(size_t j=0;j<rows;j++){Row row;row.group=uint8_t(r.num(1));row.state=uint8_t(r.num(1));if(r.num(2)||row.group<1||row.group>3||row.state>2)throw std::runtime_error("invalid row flags");row.index=uint32_t(r.num(4));row.item=uint32_t(r.num(4));row.count=uint32_t(r.num(4));row.progress=r.num(8);row.goal=r.num(8);row.label=r.text();if(!row.index||row.item<2||!row.count)throw std::runtime_error("invalid reward");c.rows.push_back(row);}
  v.campaigns.push_back(c);
 }r.end();return v;
}
inline std::vector<uint8_t> claim(const Campaign&c,const Row&r){
 if(c.state||r.state!=1||!hex(c.source,64)||!hex(c.key,32)||!r.index)throw std::runtime_error("claim unavailable");std::vector<uint8_t>p;put(p,c.family,1);put(p,r.group,1);put(p,0,2);put(p,r.index,4);p.insert(p.end(),c.source.begin(),c.source.end());p.insert(p.end(),c.key.begin(),c.key.end());return p;
}
// One outstanding operation; failed/time-out requests invalidate the complete
// cached view. Only a matching response can reopen the claim action.
struct Controller {
 uint64_t next=0,pending=0;uint16_t kind=0;bool ready=false;View current;
 uint64_t begin(uint16_t type){if(pending)throw std::runtime_error("operation pending");if(++next==0)throw std::runtime_error("request IDs exhausted");pending=next;kind=type;ready=false;return pending;}
 void accept(uint64_t id,uint16_t type,View v){if(!pending||pending!=id||kind!=type)throw std::runtime_error("stale response");current=std::move(v);pending=0;ready=true;}
 void fail(){pending=0;ready=false;current=View{};}
};
}

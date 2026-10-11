#include "protocol.h"
#include <cassert>
#include <fstream>
#include <iostream>
template<class F>void refuses(F f){bool failed=false;try{f();}catch(const std::exception&){failed=true;}assert(failed);}
int main(int argc,char**argv){
 auto p=features::frame(1,1);const uint8_t expected[]={0x55,0x31,0x31,0x35,1,0,1,0,1,0,0,0,0,0,0,0,0,0,0,0,0,0,0,0};assert(p==std::vector<uint8_t>(expected,expected+24));
 auto reply=p;reply[7]=0x80;uint32_t status=0;assert(features::replyHeader(reply,1,1,status)==0&&status==0);refuses([&]{features::replyHeader(reply,1,2,status);});reply[16]=255;reply[17]=255;refuses([&]{features::replyHeader(reply,1,1,status);});
 features::Controller c;auto id=c.begin(1);refuses([&]{c.begin(2);});refuses([&]{c.accept(id+1,1,{});});assert(!c.ready&&c.pending==id);c.fail();assert(!c.ready&&!c.pending);refuses([&]{c.accept(id,1,{});});id=c.begin(1);c.accept(id,1,{7,{}});assert(c.ready&&!c.pending&&c.current.role==7);
 assert(features::utf8("周常"));assert(!features::utf8(std::string("\xc0\x80",2)));assert(!features::utf8(std::string("\xed\xa0\x80",3)));assert(!features::utf8(std::string("\xf4\x90\x80\x80",4)));assert(!features::utf8(std::string("a\0b",3)));
 if(argc==2){std::ifstream file(argv[1],std::ios::binary);std::vector<uint8_t>data((std::istreambuf_iterator<char>(file)),{});assert(!data.empty());auto v=features::view(data);assert(v.role==7&&v.campaigns.size()==1&&v.campaigns[0].title=="周常");const auto&r=v.campaigns[0].rows[0];assert(r.item==10001&&r.count==3&&r.goal==60&&r.progress==60);auto body=features::claim(v.campaigns[0],r);assert(body.size()==104&&body[0]==1&&body[1]==1&&body[4]==1);for(size_t n=0;n<data.size();n++){auto truncated=std::vector<uint8_t>(data.begin(),data.begin()+n);refuses([&]{features::view(truncated);});}data.push_back(0);refuses([&]{features::view(data);});}
 std::cout<<"PASS protocol bounds, UTF8, single pending request, stale replies, Go view/claim interoperability\n";
}

#pragma once
#include "native_symbol_window.h"
#include "symbol_protocol.h"
#include "native_symbol_effects.h"

namespace native_symbol_info {
using native_symbol_window::Control;
using native_symbol_window::visible;
using native_symbol_window::text;
using native_symbol_window::hovered;
using native_symbol_window::clicked;
static void* lastTab=nullptr;
static void* lastSummary=nullptr;
static uint64_t lastInfoGeneration=0;
static uint64_t tabRole=0,tabSequence=UINT64_MAX,summaryRole=0,summarySequence=UINT64_MAX;
static std::string tooltipGroup,flashGroup;
static ULONGLONG flashUntil=0;
static int effectMode(){auto who=native_symbol_effects::identity();return who.character&&native_symbol_effects::buffEnabled(reinterpret_cast<void*>(who.character))?1:0;}
static const wchar_t* gradeName(uint32_t grade){const wchar_t* names[]={L"稀有",L"神器",L"传说",L"史诗"};return grade<4?names[grade]:L"";}
// New_EmblemRemodel_Symbol_title_00..03 in the original CN string table.
static const wchar_t* titleGrade(uint32_t grade){const wchar_t* names[]={L"华丽",L"灿烂",L"玲珑",L"神圣"};return grade<4?names[grade]:L"";}
static int dotFrame(const symbol_wire::Item& item){const auto grade=(item.flags>>16)&255,rarity=(item.flags>>8)&255;return grade==1?12:grade==2?13:grade==3?14:grade==7?15:grade==8?16:rarity==6?15:12;}

static std::wstring wide(const std::string& s){
    if(s.empty())return L"";
    const int n=MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,s.data(),int(s.size()),nullptr,0);
    std::wstring result(n,L'\0');if(n)MultiByteToWideChar(CP_UTF8,MB_ERR_INVALID_CHARS,s.data(),int(s.size()),&result[0],n);return result;
}
static std::wstring compactOption(const std::string& s){std::wstring result;for(auto c:wide(s))if(c!=L' ')result+=c;return result;}
static void coloredText(void* parent,const wchar_t* name,const std::wstring& value,uint32_t grade){
    for(uint32_t i=0;i<4;++i){const auto id=std::wstring(name)+(i?L"_"+std::to_wstring(i):L"");Control c(parent,id.c_str(),true);text(c.value,value);visible(c.value,i==grade);}
}
static void attributes(void* parent,const wchar_t* prefix,const std::string& value,size_t columns,size_t count,bool child=true){
    // Explicit native text rows avoid relying on an undocumented multiline
    // flag. Wrap between attributes, retaining the complete source values.
    std::vector<std::wstring> lines;std::wstring line;size_t width=0;
    auto s=wide(value);size_t begin=0;
    while(begin<s.size()){
        auto end=s.find(L"  ·  ",begin);auto field=s.substr(begin,end==std::wstring::npos?end:end-begin);
        size_t n=0;for(auto c:field)n+=c<128?1:2;
        if(width&&width+3+n>columns){lines.push_back(line);line.clear();width=0;}
        if(width){line+=L" / ";width+=3;}line+=field;width+=n;
        if(end==std::wstring::npos)break;begin=end+5;
    }
    if(!line.empty())lines.push_back(line);
    for(size_t i=0;i<count;++i){Control c(parent,(std::wstring(prefix)+std::to_wstring(i)).c_str(),child);text(c.value,i<lines.size()?lines[i]:L"");}
}
static const wchar_t* label(const std::string& name){
    if(name=="red_symbol")return L"红色星徽";
    if(name=="blue_symbol")return L"蓝色星徽";
    if(name=="green_symbol")return L"绿色星徽";
    if(name=="yellow_symbol")return L"黄色星徽";
    if(name=="multi_symbol")return L"彩色星徽";
    if(name=="platinum")return L"白金徽章";
    if(name=="buff_platinum")return L"增益白金徽章";
    return L"";
}
static size_t occupied(const symbol_wire::Group& g){size_t n=0;for(auto&i:g.items)if(i.id)++n;return n;}
static void items(void* parent,const symbol_wire::Group& g){
    for(size_t i=0;i<6;++i){
        Control item(parent,(L"emblem_"+std::to_wstring(i)).c_str(),true);
        const auto id=i<g.items.size()?g.items[i].id:0;
        if(item.value){native_symbol_window::widgets.setTemplate(item.value,id);visible(item.value,id!=0);}
    }
}
static void flash(const std::string& name){flashGroup=name;flashUntil=GetTickCount64()+900;}
static void reset(){lastTab=lastSummary=nullptr;tooltipGroup.clear();flashGroup.clear();tabSequence=summarySequence=UINT64_MAX;}
static void header(void* tab,const symbol_wire::View& view,bool ready){
    if(!tab)return;
    Control tooltip(tab,L"individual_info",true);
    Control platinum(tab,L"platinum_info",true);
    const bool changed=lastTab!=tab||tabRole!=view.role||tabSequence!=view.sequence;
    if(changed){lastTab=tab;tabRole=view.role;tabSequence=view.sequence;tooltipGroup.clear();}
    std::string overGroup;
    bool overTooltip=hovered(tooltip.value)||hovered(platinum.value);
    for(size_t i=0;i<2;++i){Control c(platinum.value,(L"emblem_"+std::to_wstring(i)).c_str(),true);overTooltip=overTooltip||hovered(c.value);}
    for(auto& g:view.groups){
        Control star(tab,std::wstring(g.name.begin(),g.name.end()).c_str(),true);
        Control over(star.value,L"over",true),level(star.value,L"level_image",true);
        const bool hot=hovered(star.value)||hovered(over.value)||hovered(level.value);
        visible(over.value,hot);
        if(hot)overGroup=g.name;
        if(changed)for(size_t i=0;i<g.items.size();++i){
            Control dot(star.value,(L"slot_"+std::to_wstring(i)).c_str(),true);
            if(g.items[i].id)native_symbol_window::imageIndex(dot.value,dotFrame(g.items[i]));
            visible(dot.value,ready&&g.items[i].id!=0);
        }
        if(changed&&g.star.present){native_symbol_window::imageIndex(star.value,6+int(g.star.grade));native_symbol_window::imageIndex(level.value,636+int(g.star.level));}
        Control glow(star.value,L"click_feedback",true);
        visible(glow.value,g.name==flashGroup&&GetTickCount64()<flashUntil);
        Control line(tab,(std::wstring(g.name.begin(),g.name.end())+L"_sync_ani").c_str(),true);
        visible(line.value,g.name==flashGroup&&GetTickCount64()<flashUntil);
    }
    if(!ready){visible(tooltip.value,false);visible(platinum.value,false);return;}
    if(overGroup.empty()&&overTooltip)overGroup=tooltipGroup;
    if(auto g=view.find(overGroup)){
        if(tooltipGroup!=overGroup||changed){
            tooltipGroup=overGroup;
            const auto&s=g->star;const int mode=effectMode();
            if(s.present){
                Control fame(tooltip.value,L"fame_text",true),fixed(tooltip.value,L"fix_option_text",true);
                coloredText(tooltip.value,L"symbol_name_text",std::wstring(titleGrade(s.grade))+label(g->name)+(s.grade<3?L" ["+std::to_wstring(s.level)+L"阶段]":L""),s.grade);
                coloredText(tooltip.value,L"rarity_text",gradeName(s.grade),s.grade);coloredText(tooltip.value,L"level_text",std::to_wstring(s.level)+L"阶段",s.grade);
                text(fame.value,std::to_wstring(s.fame));text(fixed.value,wide(s.fixed[mode]));
                const wchar_t* sections[]={L"splendid_emblem_option",L"glittering_emblem_option",L"morebright_emblem_option",L"holylight_emblem_option"};
                for(size_t i=0;i<4;++i){Control section(tooltip.value,sections[i],true),option(section.value,L"option_text",true);text(option.value,wide(s.perEmblem[i][mode]));}
            }else{
                Control title(platinum.value,L"symbol_name_text",true),count(platinum.value,L"socket_count",true),normal(platinum.value,L"normal_description",true),buff(platinum.value,L"buff_description",true);
                text(title.value,std::wstring(label(g->name))+L"栏位");text(count.value,L"已镶嵌 "+std::to_wstring(occupied(*g))+L" / "+std::to_wstring(g->items.size()));
                visible(normal.value,g->name=="platinum");visible(buff.value,g->name=="buff_platinum");items(platinum.value,*g);attributes(platinum.value,L"attribute_",g->attributes,36,4);
            }
        }
        visible(tooltip.value,g->star.present);visible(platinum.value,!g->star.present);
    }else {visible(tooltip.value,false);visible(platinum.value,false);tooltipGroup.clear();}
    Control detail(tab,L"detail_info_Button",true);
    if(clicked(detail.value))native_symbol_window::openInfo();
}
static void summary(const symbol_wire::View& view,bool ready){
    auto popup=native_symbol_window::infoPopup;
    if(!popup||!ready)return;
    if(lastSummary==popup&&summaryRole==view.role&&summarySequence==view.sequence&&lastInfoGeneration==native_symbol_window::infoGeneration)return;
    lastInfoGeneration=native_symbol_window::infoGeneration;
    lastSummary=popup;summaryRole=view.role;summarySequence=view.sequence;
    const wchar_t* rows[]={L"RedSymbol",L"YellowSymbol",L"GreenSymbol",L"BlueSymbol",L"MultiSymbol"};
    const int mode=effectMode();
    for(size_t i=0;i<5;++i){
        const std::wstring w(native_symbol_window::groups[i]);auto g=view.find(std::string(w.begin(),w.end()));if(!g)continue;
        Control row(popup,rows[i]);
        Control title(row.value,L"GradeText",true),fame(row.value,L"FameText",true),fixed(row.value,L"FixOptionText",true),extra(row.value,L"FlexibleOptionText",true),icon(row.value,L"SymbolTypeImage",true);
        // Same original CTP [symbol image info] type order as groups above.
        native_symbol_window::imageIndex(icon.value,i<4?int(i)+1:0);
        const auto&s=g->star;
        coloredText(row.value,L"GradeText",s.present?std::wstring(gradeName(s.grade))+(s.grade<3?L" "+std::to_wstring(s.level)+L"阶段":L""):L"—",s.grade);
        text(fame.value,s.present?std::to_wstring(s.fame):L"—");text(fixed.value,s.present?wide(s.fixed[mode]):L"—");
        auto resonance=s.present?compactOption(s.resonance[mode]):L"—";
        const auto split=resonance.find_first_of(L"+-0123456789");
        Control amount(row.value,L"FlexibleValueText",true);
        text(extra.value,split==std::wstring::npos?resonance:resonance.substr(0,split));
        text(amount.value,split==std::wstring::npos?L"":resonance.substr(split));
        for(size_t j=0;j<g->items.size();++j){Control dot(row.value,(L"slot_"+std::to_wstring(j)).c_str(),true);if(g->items[j].id)native_symbol_window::imageIndex(dot.value,dotFrame(g->items[j]));visible(dot.value,g->items[j].id!=0);}
    }

}
}

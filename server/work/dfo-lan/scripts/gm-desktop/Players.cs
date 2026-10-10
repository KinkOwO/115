using System;
using System.Collections.Generic;
using System.Drawing;
using System.Linq;
using System.Threading.Tasks;
using System.Windows.Forms;

partial class GMForm {
 readonly DataGridView playerGrid=new DataGridView();
 readonly TextBox playerSearch=new TextBox();readonly Label playerCount=new Label();
 Dictionary<string,object> playerDirectory;string directoryFingerprint="";DateTime directoryRead=DateTime.MinValue;
 void BuildPlayers(){
  var page=Page("玩家列表");workspace.TabPages.Add(page);
  var layout=new TableLayoutPanel{Dock=DockStyle.Fill,RowCount=2,Margin=new Padding(0)};layout.RowStyles.Add(new RowStyle(SizeType.Absolute,64));layout.RowStyles.Add(new RowStyle(SizeType.Percent,100));page.Controls.Add(layout);
  var top=Flow();StyleInput(playerSearch);playerSearch.Width=280;NativeTheme.Cue(playerSearch,"搜索账号或角色名字");top.Controls.Add(playerSearch);top.Controls.Add(Button("刷新玩家列表",()=>LoadPlayerDirectory(true)));playerCount.ForeColor=Muted;playerCount.AutoSize=true;playerCount.Margin=new Padding(8,15,0,0);top.Controls.Add(playerCount);layout.Controls.Add(top,0,0);
  Grid(playerGrid,"账号","账号编号","角色","角色编号","等级","职业编号");var surface=new Surface{Dock=DockStyle.Fill,Padding=new Padding(12),Margin=new Padding(0)};surface.Controls.Add(playerGrid);layout.Controls.Add(surface,0,1);playerSearch.TextChanged+=(s,e)=>RenderPlayers();
  playerGrid.CellDoubleClick+=async(s,e)=>{if(e.RowIndex<0||playerGrid.Rows[e.RowIndex].Tag==null)return;var ids=(long[])playerGrid.Rows[e.RowIndex].Tag;await Run(async()=>{loading=true;for(int i=0;i<accounts.Items.Count;i++)if(((Choice)accounts.Items[i]).Id==ids[0])accounts.SelectedIndex=i;loading=false;await LoadCharacters();loading=true;for(int i=0;i<characters.Items.Count;i++)if(((Choice)characters.Items[i]).Id==ids[1])characters.SelectedIndex=i;loading=false;await LoadCharacter();});};
 }
 async Task LoadPlayerDirectory(bool force){
  if(preview){playerDirectory=Program.Parse("{\"accounts\":[{\"id\":1,\"username\":\"演示账号\",\"characters\":[{\"id\":1,\"name\":\"演示角色\",\"level\":115,\"profession\":0}]}]}");RenderPlayers();return;}
  if(!force&&playerDirectory!=null&&(DateTime.UtcNow-directoryRead).TotalSeconds<15)return;
  var d=await Api("/control/players");if(Program.Str(d,"edition")!="cloud"||!Program.Bool(d,"read_only"))throw new Exception("玩家目录版本验证失败。");if(closing)return;
  string fingerprint=Program.Json.Serialize(d);bool changed=fingerprint!=directoryFingerprint;playerDirectory=d;directoryRead=DateTime.UtcNow;directoryFingerprint=fingerprint;
  if(changed||force)RenderPlayers();
  if(!gmReady&&(changed||force)){long previousAccount=account,previousRole=role;loading=true;try{accounts.Items.Clear();accounts.Items.Add(new Choice{Id=0,Text="请选择账号（只读）"});foreach(var a in DirectoryAccounts())accounts.Items.Add(new Choice{Id=Convert.ToInt64(a["id"]),Text=Program.Str(a,"username")+" (#"+a["id"]+")"});accounts.SelectedIndex=0;for(int i=1;i<accounts.Items.Count;i++)if(((Choice)accounts.Items[i]).Id==previousAccount)accounts.SelectedIndex=i;account=((Choice)accounts.SelectedItem).Id;FillDirectoryCharacters();for(int i=0;i<characters.Items.Count;i++)if(((Choice)characters.Items[i]).Id==previousRole)characters.SelectedIndex=i;}finally{loading=false;}ShowDirectoryCharacter();}
 }
 IEnumerable<Dictionary<string,object>> DirectoryAccounts(){return playerDirectory==null?new Dictionary<string,object>[0]:Program.Rows(playerDirectory,"accounts").Select(Program.Dict);}
 void FillDirectoryCharacters(){bool previousLoading=loading;loading=true;try{characters.Items.Clear();characters.Items.Add(new Choice{Id=0,Text="请选择角色（只读）"});var a=DirectoryAccounts().FirstOrDefault(x=>Convert.ToInt64(x["id"])==account);if(a!=null)foreach(var c in Program.Rows(a,"characters").Select(Program.Dict))characters.Items.Add(new Choice{Id=Convert.ToInt64(c["id"]),Text=Program.Str(c,"name")+" · Lv."+Program.Str(c,"level")});characters.SelectedIndex=0;}finally{loading=previousLoading;}}
 void ShowDirectoryCharacter(){role=characters.SelectedItem is Choice?((Choice)characters.SelectedItem).Id:0;target.Text=role==0?"开服中 · 玩家列表只读":characters.SelectedItem.ToString();balances.Text=role==0?"查看账号和角色无需停服":"等级为最近保存值\n修改前请进入维护";}
 void RenderPlayers(){
  playerGrid.Rows.Clear();string search=playerSearch.Text.Trim();int accountsTotal=0,charactersTotal=0;
  foreach(var a in DirectoryAccounts()){accountsTotal++;var roles=Program.Rows(a,"characters").Select(Program.Dict).ToArray();charactersTotal+=roles.Length;string username=Program.Str(a,"username");if(roles.Length==0){if(username.IndexOf(search,StringComparison.OrdinalIgnoreCase)>=0)playerGrid.Rows.Add(username,a["id"],"尚未创建角色","—","—","—");continue;}
   foreach(var c in roles){if(username.IndexOf(search,StringComparison.OrdinalIgnoreCase)<0&&Program.Str(c,"name").IndexOf(search,StringComparison.OrdinalIgnoreCase)<0)continue;int i=playerGrid.Rows.Add(username,a["id"],c["name"],c["id"],c["level"],c["profession"]);playerGrid.Rows[i].Tag=new[]{Convert.ToInt64(a["id"]),Convert.ToInt64(c["id"])};}
  }
  playerCount.Text=accountsTotal+" 个账号 · "+charactersTotal+" 个角色";
 }
}

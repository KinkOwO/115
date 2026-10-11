using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.Drawing;
using System.IO;
using System.Linq;
using System.Net;
using System.Reflection;
using System.Security.Cryptography;
using System.Security.Cryptography.X509Certificates;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows.Forms;


static class Program {
#if CLOUD
 public static readonly bool Cloud=true;
#else
 public static readonly bool Cloud=false;
#endif
 public static JavaScriptSerializer Json {get{return new JavaScriptSerializer { MaxJsonLength=16777216 };}}
 public static string ProfileFile { get { return OwnerProfile.Primary; } }
 public static string Hash(byte[] b) { using(var sha=SHA256.Create())return BitConverter.ToString(sha.ComputeHash(b)).Replace("-","").ToLowerInvariant(); }
 public static Dictionary<string,object> Parse(string s) { return Json.Deserialize<Dictionary<string,object>>(s); }
 public static string Str(Dictionary<string,object> d,string key) { object v;return d.TryGetValue(key,out v)&&v!=null?Convert.ToString(v):""; }
 public static bool Bool(Dictionary<string,object>d,string key){object v;return d.TryGetValue(key,out v)&&v is bool&&(bool)v;}
 public static Dictionary<string,object> Dict(object o){return (Dictionary<string,object>)o;}
 public static IEnumerable<object> Rows(Dictionary<string,object>d,string key){object v;return d.TryGetValue(key,out v)&&v!=null?((System.Collections.IEnumerable)v).Cast<object>():new object[0];}
 public static string ExtractBackend(string root){
  byte[] bytes;using(var stream=Assembly.GetExecutingAssembly().GetManifestResourceStream("backend.exe")){using(var m=new MemoryStream()){stream.CopyTo(m);bytes=m.ToArray();}}
  var hash=Hash(bytes);if(hash!=Local115US.ReleaseBuild.GMHash)throw new Exception("内嵌 GM 后台校验失败。");
  var cache=Path.Combine(Local115US.Core.Module(root),"runtime","tool-cache",hash);Directory.CreateDirectory(cache);var exe=Path.Combine(cache,"gm-local.exe");
  if(!File.Exists(exe)||Hash(File.ReadAllBytes(exe))!=hash){var temp=exe+"."+Guid.NewGuid().ToString("N")+".tmp";File.WriteAllBytes(temp,bytes);try{if(File.Exists(exe))File.Replace(temp,exe,null);else File.Move(temp,exe);}finally{if(File.Exists(temp))File.Delete(temp);}}
  return exe;
 }
 public static void Import(string path) { OwnerProfile.Import(path); }
 public static void ValidateProfile(Dictionary<string,object>d) {
  if(d==null)throw new InvalidOperationException("服主凭据格式无效。"); Uri uri;if(!Uri.TryCreate(Str(d,"endpoint"),UriKind.Absolute,out uri)||uri.Scheme!="https"||!String.IsNullOrEmpty(uri.UserInfo)||uri.AbsolutePath!="/"||!String.IsNullOrEmpty(uri.Query)||!String.IsNullOrEmpty(uri.Fragment))throw new InvalidOperationException("联网GM地址无效，必须使用专用HTTPS入口。");
  if(!Regex.IsMatch(Str(d,"token"),"^[a-f0-9]{64}$")||!Regex.IsMatch(Str(d,"certificate_sha256"),"^[a-f0-9]{64}$"))throw new InvalidOperationException("服主管理凭证格式无效。");
 }

}

class Choice { public long Id; public string Text; public override string ToString(){return Text;} }

partial class GMForm:Form {
 readonly bool preview;
 readonly Color bg=Color.FromArgb(15,18,23),panel=Color.FromArgb(23,28,35),line=Color.FromArgb(48,55,66),text=Color.FromArgb(234,239,247),gold=Color.FromArgb(232,190,111);
 readonly ComboBox accounts=new DarkComboBox(),characters=new DarkComboBox();
 readonly Label state=new Label(),target=new Label(),balances=new Label(),location=new Label();
 readonly TextBox output=new TextBox(),query=new TextBox(),currencyReason=new TextBox(),levelReason=new TextBox(),itemReason=new TextBox();
 readonly NumericUpDown goldAmount=new DarkNumber(),ceraAmount=new DarkNumber(),level=new DarkNumber(),template=new DarkNumber(),amount=new DarkNumber();
 readonly DataGridView items=new DataGridView(),bag=new DataGridView();
 readonly List<Button> actions=new List<Button>();
 readonly Dictionary<string,string> pending=new Dictionary<string,string>();
 readonly Timer poll=new Timer();
 bool busy,loading,loaded,gmReady,closing;
 string endpoint="",token="",pin="",root="",phase="";
 Process backend;
 Task backgroundWork;
 long account,role;
 decimal levelMax=115;
 public GMForm(bool preview,Local115US.PackageContext package) {
  this.package=package;root=package.Root;
  this.preview=preview;FormBorderStyle=FormBorderStyle.None;MaximizedBounds=Screen.FromControl(this).WorkingArea;Text=Program.Cloud?"115US 联网服主 GM":"115US 单机 GM";Size=new Size(Math.Min(1380,MaximizedBounds.Width-24),Math.Min(940,MaximizedBounds.Height-24));MinimumSize=new Size(1120,720);StartPosition=FormStartPosition.CenterScreen;BackColor=bg;ForeColor=text;Font=new Font("Microsoft YaHei UI",9F);AutoScaleMode=AutoScaleMode.None;
  try{using(var iconStream=Assembly.GetExecutingAssembly().GetManifestResourceStream("app.ico"))Icon=new Icon(iconStream);}catch{}
  Build();if(Program.Cloud&&!preview)SelectPage(10);NativeTheme.DarkTree(this);TopLevel=false;ShowInTaskbar=false;FormClosing+=OnClosing;
  poll.Interval=3500;poll.Tick+=async(s,e)=>{if(!preview&&!String.IsNullOrEmpty(endpoint))await RunBackground(RefreshLocal);};
 }
 Label Label(string value,int size=9){return new Label { Text=value,AutoSize=true,ForeColor=text,Font=new Font(Font.FontFamily,size),Margin=new Padding(0,6,0,7) };}
 Button Button(string value,Func<Task> action,bool accent=false){var b=new ActionButton{Text=value,Accent=accent,Danger=value.StartsWith("删除")||value.StartsWith("放弃")||value.StartsWith("停止"),Font=Font,AutoSize=true,Height=38,MinimumSize=new Size(96,38),Margin=new Padding(0,5,10,5),Padding=new Padding(14,0,14,0)};b.Click+=async(sender,e)=>await Run(action);actions.Add(b);return b;}
 FlowLayoutPanel Flow(){return new FlowLayoutPanel{AutoSize=true,Dock=DockStyle.Top,WrapContents=true,BackColor=Color.Transparent,Margin=new Padding(0),Padding=new Padding(0)};}
 void StyleInput(Control c){c.BackColor=Color.FromArgb(16,21,29);c.ForeColor=text;c.Font=new Font(Font.FontFamily,10F);c.Margin=new Padding(0,3,12,8);c.Width=220;if(c is TextBox)((TextBox)c).BorderStyle=BorderStyle.FixedSingle;if(c is NumericUpDown){((NumericUpDown)c).BorderStyle=BorderStyle.FixedSingle;((NumericUpDown)c).TextAlign=HorizontalAlignment.Left;}if(c is ComboBox){var combo=(ComboBox)c;combo.FlatStyle=FlatStyle.Flat;combo.IntegralHeight=false;combo.DropDownHeight=280;}}
 void Number(NumericUpDown n,decimal min,decimal max,decimal value){StyleInput(n);n.Minimum=min;n.Maximum=max;n.Value=value;n.ThousandsSeparator=true;n.Width=160;}
 Control Field(string label,Control input){var p=new FlowLayoutPanel{FlowDirection=FlowDirection.TopDown,WrapContents=false,AutoSize=true,Margin=new Padding(0,0,6,0)};p.Controls.Add(Label(label));p.Controls.Add(input);return p;}
 void Grid(DataGridView g,params string[] cols){NativeTheme.Dark(g);g.Dock=DockStyle.Fill;g.BackgroundColor=panel;g.BorderStyle=BorderStyle.None;g.ForeColor=text;g.ReadOnly=true;g.AllowUserToAddRows=false;g.AllowUserToDeleteRows=false;g.RowHeadersVisible=false;g.SelectionMode=DataGridViewSelectionMode.FullRowSelect;g.MultiSelect=false;g.AutoSizeColumnsMode=DataGridViewAutoSizeColumnsMode.Fill;g.EnableHeadersVisualStyles=false;g.ColumnHeadersDefaultCellStyle.BackColor=Color.FromArgb(32,39,49);g.ColumnHeadersHeight=44;g.ColumnHeadersBorderStyle=DataGridViewHeaderBorderStyle.None;g.CellBorderStyle=DataGridViewCellBorderStyle.SingleHorizontal;g.ColumnHeadersDefaultCellStyle.ForeColor=text;g.DefaultCellStyle.BackColor=panel;g.DefaultCellStyle.ForeColor=text;g.DefaultCellStyle.SelectionBackColor=Color.FromArgb(48,43,33);g.DefaultCellStyle.SelectionForeColor=text;g.GridColor=Color.FromArgb(36,44,56);g.RowTemplate.Height=46;g.DefaultCellStyle.Padding=new Padding(9,4,9,4);g.ColumnHeadersDefaultCellStyle.Font=new Font(Font.FontFamily,9F,FontStyle.Bold);g.AlternatingRowsDefaultCellStyle.BackColor=Color.FromArgb(20,25,32);foreach(var col in cols)g.Columns.Add(col,col);}
 Panel Page(string title){return new Panel{Text=title,BackColor=bg,ForeColor=text,Padding=new Padding(0)};}
 async Task Run(Func<Task> action){if(busy||closing)return;busy=true;foreach(var b in actions)b.Enabled=false;accounts.Enabled=characters.Enabled=false;try{if(backgroundWork!=null){try{await backgroundWork;}catch{}}state.ForeColor=Color.FromArgb(104,220,197);await action();}catch(Exception e){state.ForeColor=Color.FromArgb(247,155,160);state.Text=!Program.Cloud&&e.Message.Contains("请先停止")?"游戏服务或残留租约阻止连接，请点击右上角“停止服务端”。":e.Message;output.Text=e.Message;}finally{busy=false;foreach(var b in actions)b.Enabled=!preview;accounts.Enabled=characters.Enabled=!preview;}}
 async Task<Dictionary<string,object>> Api(string path,object body=null){
  if(preview)throw new Exception("界面预览未连接真实存档。");if(String.IsNullOrEmpty(endpoint))throw new Exception("请先连接GM服务。");
  package.AssertUnchanged();
  var address=endpoint.TrimEnd('/')+path;var auth=token;var expectedPin=pin;
  return await Task.Run(()=>{
   var req=(HttpWebRequest)WebRequest.Create(address);req.Method=body==null?"GET":"POST";req.ContentType="application/json";req.Timeout=90000;req.ReadWriteTimeout=90000;req.AllowAutoRedirect=false;req.Proxy=null;
   if(Program.Cloud){req.Headers["Authorization"]="Bearer "+auth;req.ServerCertificateValidationCallback=(s,cert,chain,errors)=>{if(cert==null)return false;var x=new X509Certificate2(cert);return DateTime.UtcNow>=x.NotBefore.ToUniversalTime()&&DateTime.UtcNow<=x.NotAfter.ToUniversalTime()&&Program.Hash(cert.GetRawCertData())==expectedPin;};}
   else req.Headers["X-GM-Token"]=auth;
   if(body!=null){var bytes=Encoding.UTF8.GetBytes(Program.Json.Serialize(body));req.ContentLength=bytes.Length;using(var stream=req.GetRequestStream())stream.Write(bytes,0,bytes.Length);}
   HttpWebResponse res;try{res=(HttpWebResponse)req.GetResponse();}catch(WebException e){res=e.Response as HttpWebResponse;if(res==null)throw new Exception(Program.Cloud?"无法连接云端GM，请检查服务器管理服务与网络。":"本机GM无响应，请重新打开程序。");}
   using(res){using(var reader=new StreamReader(res.GetResponseStream(),Encoding.UTF8)){var d=Program.Parse(reader.ReadToEnd());if(d.ContainsKey("error"))throw new Exception(Program.Str(d,"error"));if((int)res.StatusCode>=400)throw new Exception("管理请求失败："+(int)res.StatusCode);return d;}}
  });
 }
 async Task Connect(){
  if(preview){state.Text="界面预览 · 未连接真实存档 · 所有写入已禁用";location.Text=Program.Cloud?"腾讯云远程管理 · 预览":"本机独立存档 · 预览";accounts.Items.Add(new Choice{Id=1,Text="演示账号"});characters.Items.Add(new Choice{Id=1,Text="演示角色 · Lv.115"});loading=true;accounts.SelectedIndex=characters.SelectedIndex=0;loading=false;target.Text="演示角色 · Lv.115";balances.Text="金币：100,000\n点券：5,000";inventoryRows.Add(new Dictionary<string,object>{{"container","items"},{"slot",121},{"template",3037},{"name","无色小晶块"},{"amount",100}});goldSet.Value=100000;ceraSet.Value=5000;coinSet.Value=30;spSet.Value=12000;spAlt.Value=12000;
  inventoryRows.Add(new Dictionary<string,object>{{"container","items"},{"slot",122},{"template",3036},{"name","材料示例 #3036"},{"amount",30}});
  inventoryRows.Add(new Dictionary<string,object>{{"container","items"},{"slot",123},{"template",3326},{"name","强烈的气息"},{"amount",500}});
  inventoryRows.Add(new Dictionary<string,object>{{"container","items"},{"slot",124},{"template",3038},{"name","材料示例 #3038"},{"amount",200}});
  RenderInventory();bag.Rows.Add("背包 9","3037","物品示例","3");if(Program.Cloud)await LoadPlayerDirectory(true);return;}
  if(Program.Cloud){
   ServicePointManager.SecurityProtocol=SecurityProtocolType.Tls12;
   if(String.IsNullOrEmpty(endpoint)){
    var d=OwnerProfile.Load();endpoint=Program.Str(d,"endpoint");token=Program.Str(d,"token");pin=Program.Str(d,"certificate_sha256");location.Text="联网存档\n"+endpoint;
   }
   await RefreshCloud();poll.Start();return;
  }
  if(backend==null||backend.HasExited){state.Text="正在启动本机GM，首次读取物品目录约一分钟…";if(String.IsNullOrEmpty(root))root=FindRoot();location.Text="单机存档\n"+root;if(backend!=null){backend.Dispose();backend=null;}await StartBackend(root);}
  await LoadAccounts();poll.Start();
 }
 string FindRoot(){return package.Root;}


 async Task RunBackground(Func<Task> action){
  if(busy||closing||backgroundWork!=null)return;
  var work=action();backgroundWork=work;
  try{await work;if(!closing&&!IsDisposed)state.ForeColor=Color.FromArgb(104,220,197);}
  catch(Exception e){if(!closing&&!IsDisposed){state.ForeColor=Color.FromArgb(247,155,160);state.Text=e.Message;output.Text=e.Message;}}
  finally{backgroundWork=null;}
 }
 async Task RefreshCloud(){
  var d=await Api("/control/status");if(closing)return;bool wasReady=gmReady;
  phase=Program.Str(d,"phase");gmReady=Program.Bool(d,"gm_ready");
  string detail=Program.Str(d,"detail");state.Text="云端："+(Program.Bool(d,"game_running")?"游戏服务运行":"游戏服务停止")+" · 游戏连接 "+Program.Str(d,"connections")+" · "+(String.IsNullOrEmpty(detail)?"玩家列表可查看 · 修改需进入维护":detail);
  if(gmReady&&!loaded)await LoadAccounts();
  if(!gmReady){if(loaded){loaded=false;loading=true;accounts.Items.Clear();characters.Items.Clear();loading=false;account=role=0;ClearTarget();bag.Rows.Clear();}}await LoadPlayerDirectory(wasReady&&!gmReady);
 }

 async Task Maintenance(){if(!Program.Cloud)return;if(MessageBox.Show("所有玩家退出后，将停止云端游戏服务并准备GM。\n维护期间无法登录游戏；修改完成后请点击“恢复开服”。\n\n现在进入维护？",Text,MessageBoxButtons.YesNo,MessageBoxIcon.Question)!=DialogResult.Yes)return;await Api("/control/maintenance",new{confirm=true});await RefreshCloud();}
 async Task Resume(){if(!Program.Cloud)return;if(MessageBox.Show("关闭云端GM并恢复游戏服务？\n请确认所需修改已经完成。",Text,MessageBoxButtons.YesNo,MessageBoxIcon.Question)!=DialogResult.Yes)return;await Api("/control/resume",new{confirm=true});await RefreshCloud();}
 async Task LoadAccounts(){var st=await Api("/api/gm/status");if(Program.Str(st,"edition")!=(Program.Cloud?"cloud":"local"))throw new Exception("版本隔离拒绝：管理目标不属于本程序。");package.AssertDatabase(Program.Str(st,"database"));levelMax=Math.Min(255,Convert.ToDecimal(st["level_max"]));level.Maximum=levelMax;var d=await Api("/api/overview");loading=true;accounts.Items.Clear();characters.Items.Clear();accounts.Items.Add(new Choice{Id=0,Text="请选择账号"});foreach(var a in Program.Rows(d,"accounts")){var row=Program.Dict(a);accounts.Items.Add(new Choice{Id=Convert.ToInt64(row["id"]),Text=Program.Str(row,"username")+" (#"+Program.Str(row,"id")+")"});}accounts.SelectedIndex=0;loading=false;account=role=0;loaded=true;gmReady=true;state.Text=Program.Cloud?"联网 GM 已就绪 · 云端存档 · 完成后请恢复开服":"单机 GM 已就绪 · 支持在线发物品、金币/点券和等级 · 修改前备份";}
 async Task LoadCharacters(){if(Program.Cloud&&!gmReady){account=accounts.SelectedItem is Choice?((Choice)accounts.SelectedItem).Id:0;FillDirectoryCharacters();ShowDirectoryCharacter();return;}account=accounts.SelectedItem is Choice?((Choice)accounts.SelectedItem).Id:0;role=0;loading=true;characters.Items.Clear();loading=false;bag.Rows.Clear();target.Text="请选择角色";if(account==0)return;var d=await Api("/api/characters?account="+account);loading=true;characters.Items.Add(new Choice{Id=0,Text="请选择角色"});foreach(var a in Program.Rows(d,"characters")){var row=Program.Dict(a);characters.Items.Add(new Choice{Id=Convert.ToInt64(row["id"]),Text=Program.Str(row,"name")+" · Lv."+Program.Str(row,"level")+" (#"+Program.Str(row,"id")+")"});}characters.SelectedIndex=0;loading=false;balances.Text="账号点券："+Program.Str(d,"cera");}
 async Task LoadCharacter(){if(Program.Cloud&&!gmReady){ShowDirectoryCharacter();return;}role=characters.SelectedItem is Choice?((Choice)characters.SelectedItem).Id:0;if(role==0)return;var d=await Api("/api/character?id="+role);var c=Program.Dict(d["character"]);target.Text=Program.Str(c,"name")+" · Lv."+Program.Str(c,"level");balances.Text="金币："+Program.Str(c,"gold")+"\n点券："+Program.Str(d,"cera");bag.Rows.Clear();foreach(var a in Program.Rows(d,"bag")){var row=Program.Dict(a);bag.Rows.Add((Program.Bool(row,"worn")?"穿戴 ":"背包 ")+Program.Str(row,"slot"),Program.Str(row,"template"),Program.Str(row,"name"),Program.Str(row,"count"));}await Snapshot();}
 async Task Search(){var d=await Api("/api/items?limit=60&q="+Uri.EscapeDataString(query.Text.Trim()));items.Rows.Clear();foreach(var a in Program.Rows(d,"items")){var row=Program.Dict(a);items.Rows.Add(Program.Str(row,"id"),Program.Str(row,"name"),Program.Str(row,"type_label"),Program.Bool(row,"grantable")?"是":"否");}state.Text="搜索结果："+Program.Str(d,"count")+" 条，点击物品选择编号。";}
 string Reason(TextBox t){if(String.IsNullOrWhiteSpace(t.Text))throw new Exception("请填写操作原因。");return t.Text.Trim();}
 Dictionary<string,object> Request(string why){if(account<=0||role<=0)throw new Exception("请先选择账号和角色。");return new Dictionary<string,object>{{"account",account},{"character",role},{"reason",why}};}
 async Task Mutate(string path,Dictionary<string,object> body,string description){
  if(Program.Cloud&&!gmReady)throw new Exception("当前为只读玩家列表。所有玩家退出后，点击进入维护才能修改存档。");
  var st=await Api("/api/gm/status");if(!Program.Bool(st,"write_ready"))throw new Exception("当前服务端没有提供在线GM入口，请使用本次配套启动器；旧服务需退出游戏后重启。");
  package.AssertDatabase(Program.Str(st,"database"));bool online=Program.Bool(st,"online");if(online&&path!="/api/grant"&&path!="/api/gm/level"&&!(path=="/api/gm/command"&&(Program.Str(body,"action")=="gold_set"||Program.Str(body,"action")=="cera_set")))throw new Exception("在线模式支持发物品、金币/点券和等级。此项其他修改请退出游戏并停止服务端后使用。");
  if(MessageBox.Show((Program.Cloud?"联网存档":"单机存档")+"\n账号："+accounts.SelectedItem+"\n角色："+characters.SelectedItem+"\n"+description+"\n\n修改前会自动备份，确认执行？",Text,MessageBoxButtons.YesNo,MessageBoxIcon.Question)!=DialogResult.Yes)return;
  var key=path+Program.Json.Serialize(body);string id;if(!pending.TryGetValue(key,out id)){id="gm-desktop-"+Guid.NewGuid().ToString("N");pending[key]=id;}body["grant_id"]=id;
  var result=await Api(path,body);pending.Remove(key);output.Text=Program.Json.Serialize(result);state.Text=Program.Bool(result,"applied")?(Program.Bool(result,"online")?"在线修改已保存并发送游戏刷新，备份与审计已保存。":"操作成功，备份与审计已保存。"):"操作回执已返回；相同操作不会重复发放。";await LoadCharacter();
 }
 async Task GrantCurrency(){if(goldAmount.Value==0&&ceraAmount.Value==0)throw new Exception("请填写非零金币或点券。");var d=Request(Reason(currencyReason));d["gold"]=goldAmount.Value;d["cera"]=ceraAmount.Value;await Mutate("/api/grant",d,"增加金币 "+goldAmount.Value+"，点券调整 "+ceraAmount.Value);}
 async Task SetLevel(){var d=Request(Reason(levelReason));d["level"]=level.Value;await Mutate("/api/gm/level",d,"目标等级 "+level.Value+"（不赠送技能点）");}
 async Task GrantItem(){var d=Request(Reason(itemReason));d["items"]=new[]{new{template=template.Value,amount=amount.Value}};await Mutate("/api/grant",d,"发放物品 "+template.Value+" × "+amount.Value);}
 async Task History(){if(account<=0)throw new Exception("请先选择账号。");output.Text=Program.Json.Serialize(await Api("/api/history?account="+account));state.Text="已读取当前账号操作记录。";}
 async Task Backup(){output.Text=Program.Json.Serialize(await Api("/api/gm/backup",new{}));state.Text="一致性备份与校验完成。";}
 async Task Backups(){output.Text=Program.Json.Serialize(await Api("/api/gm/backups"));state.Text="已读取备份列表。";}
 async void OnClosing(object s,FormClosingEventArgs e){if(shutdown)return;e.Cancel=true;try{await ShutdownAsync();shutdown=true;Close();}catch(Exception error){state.Text=error.Message;}}
}

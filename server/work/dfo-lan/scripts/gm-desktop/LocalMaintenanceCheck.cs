using System;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using System.Threading;
using System.Windows.Forms;

// Isolated process fixtures only: no real game, server, credential, player DB or API.
static class LocalMaintenanceCheck {
 static int passed;
 static void Assert(bool b,string name){if(!b)throw new Exception(name);passed++;}
 static void Refuses(Action a,string name){bool refused=false;try{a();}catch{refused=true;}Assert(refused,name);}
 static string Root(string basis,string name){string root=Path.Combine(basis,name),module=Path.Combine(root,"server","work","dfo-lan");Directory.CreateDirectory(Path.Combine(module,"runtime","storage"));string db=Path.Combine(module,"runtime","storage","dfolan.sqlite3");File.WriteAllText(db,"SAVE SENTINEL - NOT A DATABASE");File.WriteAllText(Path.Combine(module,"runtime","storage","local.json"),Program.Json.Serialize(new{driver="sqlite",sqlite_path=db}));return root;}
 static Process Child(string stub,string root,string name,string lease){string exe=Path.Combine(root,name+".exe");File.Copy(stub,exe,true);return Process.Start(new ProcessStartInfo(exe,"\""+lease+"\""){UseShellExecute=false,CreateNoWindow=true});}
 [STAThread] static int Main(string[] args){
  string basis=args[1];Directory.CreateDirectory(basis);Process a=null,b=null,game=null;
  try{
   string ra=Root(basis,"甲 整合包"),rb=Root(basis,"乙 整合包"),db=LocalMaintenance.Database(ra),lease=db+".admin-guard";
   Assert(LocalMaintenance.Inside(db,ra)&&!LocalMaintenance.Inside(db,ra+"extra"),"path boundary");
   Assert(!LocalMaintenance.Server(new LocalMaintenance.Entry{Name="wireprobe-isolated",Path=Path.Combine(ra,"analysis","candidate","wireprobe-isolated.exe")},ra),"nested independent test server excluded");
   Assert(LocalMaintenance.Server(new LocalMaintenance.Entry{Name="wireprobe-pvf",Path=Path.Combine(ra,"server","work","dfo-lan","bin","wireprobe-pvf.exe")},ra),"formal server recognized");
   Assert(LocalMaintenance.Server(new LocalMaintenance.Entry{Name="wireprobe-pvf",Path=Path.Combine(ra,"02_服务端","server","work","dfo-lan","bin","wireprobe-pvf.exe")},ra),"canonical server recognized");
   a=Child(args[0],ra,"wireprobe-pvf",lease);b=Child(args[0],rb,"wireprobe-pvf",LocalMaintenance.Database(rb)+".admin-guard");
   var deadline=DateTime.UtcNow.AddSeconds(5);while((!File.Exists(lease)||!File.Exists(LocalMaintenance.Database(rb)+".admin-guard"))&&DateTime.UtcNow<deadline)Thread.Sleep(10);
   Assert(File.ReadAllText(lease).Trim()==a.Id.ToString(),"isolated server ready");
   var plan=LocalMaintenance.Plan(ra,db,LocalMaintenance.Snapshot(),LocalMaintenance.Alive);Assert(plan.Count==1&&plan[0].Pid==a.Id,"only selected package planned");
   game=Child(args[0],ra,"DFO","");Thread.Sleep(100);
   Refuses(()=>LocalMaintenance.Stop(ra),"running game refuses stop");Assert(!a.HasExited&&File.Exists(lease),"game refusal preserves server/lease");game.Kill();game.WaitForExit();
   File.WriteAllText(lease,b.Id.ToString());Refuses(()=>LocalMaintenance.Stop(ra),"foreign active lease refuses stop");Assert(!a.HasExited&&!b.HasExited,"foreign lease stops nothing");
   File.WriteAllText(lease,"invalid");Refuses(()=>LocalMaintenance.Stop(ra),"invalid lease refuses stop");Assert(!a.HasExited&&File.Exists(lease),"invalid lease preserved");
   File.WriteAllText(lease,a.Id.ToString());string result=LocalMaintenance.Stop(ra);
   Assert(a.HasExited&&!b.HasExited,"selected server stopped; other package alive");Assert(!File.Exists(lease)&&File.Exists(LocalMaintenance.Database(rb)+".admin-guard"),"only dead selected lease removed");
   Assert(File.ReadAllText(db)=="SAVE SENTINEL - NOT A DATABASE","save bytes untouched");Assert(result.Contains("1 个进程"),"stop summary accurate");
   Assert(!LocalMaintenance.ClearDeadLease(db,LocalMaintenance.Alive),"missing lease is idempotent");
   File.WriteAllText(lease,a.Id.ToString());Assert(LocalMaintenance.ClearDeadLease(db,LocalMaintenance.Alive),"already exited stale lease cleared");
   File.WriteAllText(lease,Process.GetCurrentProcess().Id.ToString());Refuses(()=>LocalMaintenance.ClearDeadLease(db,LocalMaintenance.Alive),"live GM lease retained");Assert(File.Exists(lease),"live lease file retained");
   Refuses(()=>LocalMaintenance.ClearDeadLease(db,pid=>{throw new UnauthorizedAccessException();}),"unknown PID status retained");Assert(File.Exists(lease),"unknown status file retained");
   File.WriteAllText(lease,"42");int calls=0;Refuses(()=>LocalMaintenance.ClearDeadLease(db,pid=>{if(++calls==1)File.WriteAllText(lease,"43");return false;}),"changed lease retained");Assert(File.ReadAllText(lease)=="43","replacement lease retained");File.Delete(lease);
   var cfg=Path.Combine(ra,"server","work","dfo-lan","runtime","storage","local.json");File.WriteAllText(cfg,Program.Json.Serialize(new{driver="sqlite",sqlite_path=LocalMaintenance.Database(rb)}));Refuses(()=>LocalMaintenance.Database(ra),"external save refuses maintenance");
   Application.EnableVisualStyles();using(var f=new GMForm(true)){var actions=(System.Collections.Generic.List<Button>)typeof(GMForm).GetField("actions",BindingFlags.Instance|BindingFlags.NonPublic).GetValue(f);Assert(actions.Any(x=>x.Text=="停止服务端"),"stop button wired in local UI");Assert(actions.Any(x=>x.Text=="连接 / 刷新"),"reconnect remains available");}
   File.WriteAllText(args[2],Program.Json.Serialize(new{passed,real_player_data_accessed=false,real_game_started=false,real_server_started=false,other_package_process_preserved=true}));Console.WriteLine("Local maintenance checks passed: "+passed);return 0;
  }catch(Exception e){Console.Error.WriteLine(e);return 1;}
  finally{foreach(var p in new[]{a,b,game})if(p!=null){try{if(!p.HasExited){p.Kill();p.WaitForExit();}}catch{}p.Dispose();}}
 }
}

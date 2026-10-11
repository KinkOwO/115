using System;
using System.IO;
using System.Diagnostics;
using System.Reflection;
using System.Linq;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Windows.Forms;
using Local115US;

class LifecycleCheck {
 static readonly List<string> passed=new List<string>();
 static void Assert(bool b,string name){if(!b)throw new Exception(name);passed.Add(name);}
 static BindingFlags fields=BindingFlags.NonPublic|BindingFlags.Instance;
 static bool Alive(Process p){try{return !p.HasExited;}catch{return false;}}
 [STAThread] static int Main(string[] args){
  string evidence=Path.GetFullPath(args[0]);Directory.CreateDirectory(evidence);string sandbox=Path.Combine(evidence,"lifecycle "+Guid.NewGuid().ToString("N"));Directory.CreateDirectory(sandbox);
  Application.EnableVisualStyles();Application.SetCompatibleTextRenderingDefault(false);
  int exit=0;var host=new Form{ShowInTaskbar=false,StartPosition=FormStartPosition.Manual,Left=-20000,Top=-20000,Width=1200,Height=800};
  host.Shown+=async(s,e)=>{try{
   var assembly=typeof(Core).Assembly;var type=assembly.GetType("GMForm",true);var gm=(Form)Activator.CreateInstance(type,new object[]{true,new PackageContext(sandbox,sandbox,false)});host.Controls.Add(gm);gm.Show();
   gm.Hide();await Task.Delay(20);Assert(!gm.IsDisposed,"hiding embedded GM does not dispose session");gm.Show();
   var actions=(List<Button>)type.GetField("actions",fields).GetValue(gm);int transitions=0;foreach(var button in actions)button.EnabledChanged+=(a,b)=>transitions++;
   var readGate=new TaskCompletionSource<int>();var read=(Task)type.GetMethod("RunBackground",fields).Invoke(gm,new object[]{(Func<Task>)(()=>readGate.Task)});Assert(transitions==0,"status polling never disables or flickers controls");
   int writes=0;var writeGate=new TaskCompletionSource<int>();Func<Task> write=()=>{writes++;return writeGate.Task;};var foreground=(Task)type.GetMethod("Run",fields).Invoke(gm,new object[]{write});Assert(actions.All(b=>!b.Enabled)&&writes==0,"foreground locks controls and waits existing status read");
   await(Task)type.GetMethod("Run",fields).Invoke(gm,new object[]{write});Assert(writes==0,"duplicate foreground request is not queued");readGate.SetResult(0);await Task.Delay(20);Assert(writes==1,"one foreground operation proceeds after read completion");
   int newReads=0;await(Task)type.GetMethod("RunBackground",fields).Invoke(gm,new object[]{(Func<Task>)(()=>{newReads++;return Task.FromResult(0);})});Assert(newReads==0,"no status poll during foreground operation");
   writeGate.SetResult(0);await foreground;await read;Assert(writes==1&&!(bool)type.GetField("busy",fields).GetValue(gm),"foreground protection clears after completion");
   type.GetField("busy",fields).SetValue(gm,true);bool refused=false;try{await(Task)type.GetMethod("ShutdownAsync").Invoke(gm,null);}catch(Exception){refused=true;}Assert(refused,"shutdown refuses active write operation");Assert(!(bool)type.GetField("shutdown",fields).GetValue(gm),"refused shutdown leaves session available");type.GetField("busy",fields).SetValue(gm,false);
   var gate=new TaskCompletionSource<int>();type.GetField("backgroundWork",fields).SetValue(gm,gate.Task);var shutdown=(Task)type.GetMethod("ShutdownAsync").Invoke(gm,null);await Task.Delay(30);Assert(!shutdown.IsCompleted,"shutdown waits background request completion");gate.SetResult(0);await shutdown;Assert((bool)type.GetField("shutdown",fields).GetValue(gm),"shutdown completes after request receipt");gm.Dispose();
   string worker=Path.Combine(Path.GetDirectoryName(Assembly.GetExecutingAssembly().Location),"LifecycleWorker.exe"),marker=Path.Combine(sandbox,"child.pid");
   var unrelated=Process.Start(new ProcessStartInfo(worker,"sleep"){UseShellExecute=false,CreateNoWindow=true});
   var own=new Process{StartInfo=new ProcessStartInfo(worker,Core.Args("spawn",marker)){UseShellExecute=false,CreateNoWindow=true}};var owner=OwnedProcess.Start(own);
   try{for(int i=0;i<100&&!File.Exists(marker);i++)await Task.Delay(30);Assert(File.Exists(marker),"owned worker produced isolated child");using(var child=Process.GetProcessById(Int32.Parse(File.ReadAllText(marker)))){owner.Dispose();await Task.Delay(200);Assert(!Alive(own),"owned background process cleaned");Assert(!Alive(child),"owned child process cleaned by job");Assert(Alive(unrelated),"other package process remains running");}}
   finally{owner.Dispose();try{if(Alive(unrelated)){unrelated.Kill();unrelated.WaitForExit(5000);}}finally{unrelated.Dispose();own.Dispose();}}
   Console.WriteLine("PASS "+passed.Count+" lifecycle checks");
   File.WriteAllText(Path.Combine(evidence,"S18-lifecycle-result.json"),Core.Json.Serialize(new{passed=passed.Count,checks=passed,fixture=sandbox,game_started=false,real_player_data_accessed=false}));
  }catch(Exception ex){exit=1;File.WriteAllText(Path.Combine(evidence,"lifecycle-error.txt"),ex.ToString());Console.Error.WriteLine(ex);}host.Close();};Application.Run(host);return exit;
 }
}

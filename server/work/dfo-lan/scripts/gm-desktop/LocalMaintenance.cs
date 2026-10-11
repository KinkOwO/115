using System;
using System.Collections.Generic;
using System.ComponentModel;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Runtime.InteropServices;
using System.Text;
using System.Threading.Tasks;

// Local maintenance is scoped by executable path, never by a global image name/port.
static class LocalMaintenance {
 internal sealed class Entry { public int Pid; public string Name,Path; public long Started; }
 [DllImport("kernel32.dll",SetLastError=true)] static extern IntPtr OpenProcess(uint access,bool inherit,int pid);
 [DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
 [DllImport("kernel32.dll",SetLastError=true)] static extern uint WaitForSingleObject(IntPtr handle,uint milliseconds);
 internal static bool Inside(string path,string root) { return !String.IsNullOrEmpty(path)&&System.IO.Path.GetFullPath(path).StartsWith(System.IO.Path.GetFullPath(root).TrimEnd('\\')+"\\",StringComparison.OrdinalIgnoreCase); }
 internal static bool Server(Entry e,string root) {
  if(!e.Name.StartsWith("wireprobe",StringComparison.OrdinalIgnoreCase)||String.IsNullOrEmpty(e.Path))return false;
  string dir=Path.GetDirectoryName(Path.GetFullPath(e.Path));
  return new[]{root,Path.Combine(root,"server","work","dfo-lan","bin"),Path.Combine(root,"02_服务端","server","work","dfo-lan","bin")}.Any(p=>String.Equals(dir,Path.GetFullPath(p).TrimEnd('\\'),StringComparison.OrdinalIgnoreCase));
 }
 internal static bool Alive(int pid) {
  var handle=OpenProcess(0x101000,false,pid);if(handle!=IntPtr.Zero){try{uint state=WaitForSingleObject(handle,0);if(state==0)return false;if(state==258)return true;throw new Win32Exception(Marshal.GetLastWin32Error(),"无法确认管理租约持有进程状态，已保留租约。");}finally{CloseHandle(handle);}}
  int error=Marshal.GetLastWin32Error();if(error==87)return false;
  throw new Win32Exception(error,"无法确认管理租约持有进程是否已退出，已保留租约。");
 }
 internal static string Database(string root) {
  string module=Path.Combine(root,"server","work","dfo-lan");if(!Directory.Exists(module))module=Path.Combine(root,"02_服务端","server","work","dfo-lan");
  var config=Program.Parse(File.ReadAllText(Path.Combine(module,"runtime","storage","local.json"),Encoding.UTF8).TrimStart('\ufeff'));
  string driver=Program.Str(config,"driver"),value=Program.Str(config,"sqlite_path");
  if((driver!=""&&driver!="sqlite")||String.IsNullOrWhiteSpace(value))throw new Exception("单机GM需要有效的 SQLite 存档配置。");
  var db=Path.GetFullPath(Path.IsPathRooted(value)?value:Path.Combine(module,value));
  if(!Inside(db,root))throw new Exception("存档位置位于此整合包之外，请先核对启动器的整合包和存档目录。");
  return db;
 }
 internal static List<Entry> Snapshot() {
  var result=new List<Entry>();foreach(var p in Process.GetProcesses())using(p){
   string name;try{name=p.ProcessName;}catch{continue;}
   if(!name.StartsWith("wireprobe",StringComparison.OrdinalIgnoreCase)&&!name.Equals("DFO",StringComparison.OrdinalIgnoreCase))continue;
   try { result.Add(new Entry{Pid=p.Id,Name=name,Path=p.MainModule.FileName,Started=p.StartTime.ToUniversalTime().Ticks}); }
   catch(InvalidOperationException){}catch(Win32Exception){throw new Exception("无法读取游戏进程位置，请以同等权限运行GM后重试。");}
  }return result;
 }
 internal static List<Entry> Plan(string root,string db,IEnumerable<Entry> processes,Func<int,bool> alive) {
  var rows=processes.ToList();if(rows.Any(e=>e.Name.Equals("DFO",StringComparison.OrdinalIgnoreCase)&&Inside(e.Path,root)))throw new Exception("请先正常退出此整合包的游戏，再点击“停止服务端”。");
  var targets=rows.Where(e=>Server(e,root)).ToList();string lease=db+".admin-guard";
  if(File.Exists(lease)){
   int pid;if(!Int32.TryParse(File.ReadAllText(lease).Trim(),out pid)||pid<=0)throw new Exception("管理租约内容无效，已保留文件，请检查操作结果中的存档位置。");
   if(alive(pid)&&!targets.Any(e=>e.Pid==pid))throw new Exception("存档仍被其他GM或进程使用，请先关闭对应程序；不会强制清除活跃租约。");
  }return targets;
 }
 internal static bool ClearDeadLease(string db,Func<int,bool> alive) {
  string path=db+".admin-guard";if(!File.Exists(path))return false;
  string original=File.ReadAllText(path);var time=File.GetLastWriteTimeUtc(path);int pid;
  if(!Int32.TryParse(original.Trim(),out pid)||pid<=0||alive(pid))throw new Exception("管理租约仍在使用或无法确认，已保留文件。");
  if(!File.Exists(path))return false;
  if(File.ReadAllText(path)!=original||File.GetLastWriteTimeUtc(path)!=time||alive(pid))throw new Exception("管理租约发生变化，已停止清理，请重试。");
  File.Delete(path);return true;
 }
 internal static string Stop(string root) {
  string db=Database(root);var targets=Plan(root,db,Snapshot(),Alive);int stopped=0;
  foreach(var e in targets){
   Process p;try{p=Process.GetProcessById(e.Pid);}catch(ArgumentException){continue;}
   using(p){if(p.HasExited)continue;if(p.StartTime.ToUniversalTime().Ticks!=e.Started||!String.Equals(p.MainModule.FileName,e.Path,StringComparison.OrdinalIgnoreCase))throw new Exception("服务端进程已变化，请重试停止。");p.Kill();if(!p.WaitForExit(10000))throw new Exception("服务端未在时限内退出，请检查操作结果。");stopped++;}
  }
  if(Snapshot().Any(e=>Server(e,root)))throw new Exception("此整合包仍有服务端运行，请重试停止。");
  bool cleared=ClearDeadLease(db,Alive);
  return "已停止此整合包服务端 "+stopped+" 个进程"+(cleared?"；已清理退出进程的残留租约":"")+"。\r\n存档："+db;
 }
}

partial class GMForm {
 async Task StopLocalServer(){
  if(Program.Cloud||preview)return;
  if(backend!=null&&!backend.HasExited&&gmReady){var status=await Api("/api/gm/status");if(!Program.Bool(status,"online")){state.Text="当前GM处于离线模式；正在检查并停止对应服务端。";}}
  if(String.IsNullOrEmpty(root))root=FindRoot();location.Text="单机存档\n"+root;
  state.Text="正在停止此整合包服务端…";string note=await Task.Run(()=>LocalMaintenance.Stop(root));
  if(backend!=null){if(!backend.HasExited){backend.Kill();backend.WaitForExit(5000);}backend.Dispose();backend=null;}endpoint=token="";gmReady=false;
  output.Text=note+"\r\n正在连接GM…";state.Text="服务端已停止，正在连接GM…";
  await StartBackend(root);await LoadAccounts();output.Text=note+"\r\nGM已连接，可以管理存档。";
 }
}

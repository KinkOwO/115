using System;
using System.IO;
using System.Diagnostics;
using System.Text;
using System.Text.RegularExpressions;
using System.Threading.Tasks;
using System.Windows.Forms;
using Local115US;

partial class GMForm {
 readonly PackageContext package;
 bool shutdown;
 OwnedProcess ownedBackend;
 public bool IsBusy {get{return busy||backgroundWork!=null;}}
 public bool IsConnected {get{return backend!=null&&!backend.HasExited&&!String.IsNullOrEmpty(endpoint);}}
 public int BackendPID {get{return backend==null?0:backend.Id;}}
 public string StatusText {get{return state.Text;}}
 public async Task ActivateAsync(){if(shutdown)throw new ObjectDisposedException("GM");if(!loaded)await Run(Connect);}
 public async Task ShutdownAsync(){
  if(busy)throw new Exception("GM操作正在进行，请等待操作回执后再切换目录、启动或退出。");
  closing=true;poll.Stop();if(backgroundWork!=null){try{await backgroundWork;}catch{}}
  if(clientArt!=null){clientArt.Dispose();clientArt=null;}StopOwnedBackend();shutdown=true;
 }
 void StopOwnedBackend(){
  if(ownedBackend!=null){ownedBackend.Dispose();ownedBackend=null;}
  if(backend!=null){backend.Dispose();backend=null;}
  endpoint=token="";gmReady=false;loaded=false;
 }
 async Task StartBackend(string rootDir){
  package.AssertUnchanged();if(package.DatabaseIdentity=="")throw new Exception("请先进入游戏创建存档，再打开GM。");
  StopOwnedBackend();var ready=new TaskCompletionSource<string>();string last="";
  var exe=Program.ExtractBackend(rootDir);
  var psi=new ProcessStartInfo(exe,Core.Args("-root",rootDir,"-expected-database",package.Database,"-open=false")){UseShellExecute=false,CreateNoWindow=true,RedirectStandardError=true,RedirectStandardOutput=true,StandardErrorEncoding=Encoding.UTF8,StandardOutputEncoding=Encoding.UTF8,WorkingDirectory=rootDir};
  foreach(string key in new System.Collections.Generic.List<string>(psi.EnvironmentVariables.Keys.CastStrings()))if(key.StartsWith("DFO_",StringComparison.OrdinalIgnoreCase))psi.EnvironmentVariables.Remove(key);
  var process=new Process{StartInfo=psi,EnableRaisingEvents=true};backend=process;
  process.ErrorDataReceived+=(s,e)=>{if(e.Data==null)return;var m=Regex.Match(e.Data,@"http://127\.0\.0\.1:[0-9]+/#([a-f0-9]{32})");if(m.Success)ready.TrySetResult(m.Value);else last=Redact(e.Data);};
  process.Exited+=(s,e)=>ready.TrySetException(new Exception(String.IsNullOrEmpty(last)?"GM后台已退出，请检查配套文件和存档位置。":last));
  try{
   ownedBackend=OwnedProcess.Start(process);process.BeginErrorReadLine();process.BeginOutputReadLine();
   if(await Task.WhenAny(ready.Task,Task.Delay(180000))!=ready.Task)throw new Exception("GM物品目录准备超时，后台已清理，可重新连接。");
   var uri=new Uri(await ready.Task);endpoint=uri.GetLeftPart(UriPartial.Authority);token=uri.Fragment.Substring(1);
   var status=await Api("/api/gm/status");if(Program.Str(status,"edition")!="local")throw new Exception("拒绝非本地GM。");package.AssertDatabase(Program.Str(status,"database"));gmReady=true;
  }catch{StopOwnedBackend();throw;}
 }
 static string Redact(string line){return Regex.Replace(line,@"(?i)(token[=: ]+|#)[a-f0-9]{16,}","$1[已隐藏]");}
 async Task RefreshLocal(){var st=await Api("/api/gm/status");package.AssertDatabase(Program.Str(st,"database"));state.Text=Program.Bool(st,"online")?"在线 GM · 可发物品、金币/点券、等级 · 其他修改需退出游戏后停服":"维护 GM · 修改前自动备份 · 当前存档："+package.Database;}
}

static class StringCollectionExtension {public static System.Collections.Generic.IEnumerable<string> CastStrings(this System.Collections.ICollection keys){foreach(object key in keys)yield return (string)key;}}

namespace Local115US {
 // A dedicated job owns only a GM subprocess, never a game session or another package.
 public sealed class OwnedProcess:IDisposable {
  IntPtr job;Process process;
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)] struct Basic {public long ProcessTime,JobTime;public uint Flags;public UIntPtr Minimum,Maximum;public uint Active;public UIntPtr Affinity;public uint Priority,Scheduling;}
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)] struct IO {public ulong ReadOps,WriteOps,OtherOps,ReadBytes,WriteBytes,OtherBytes;}
  [System.Runtime.InteropServices.StructLayout(System.Runtime.InteropServices.LayoutKind.Sequential)] struct Extended {public Basic Basic;public IO IO;public UIntPtr ProcessMemory,JobMemory,PeakProcessMemory,PeakJobMemory;}
  [System.Runtime.InteropServices.DllImport("kernel32.dll",CharSet=System.Runtime.InteropServices.CharSet.Unicode)] static extern IntPtr CreateJobObject(IntPtr security,string name);
  [System.Runtime.InteropServices.DllImport("kernel32.dll",SetLastError=true)] static extern bool SetInformationJobObject(IntPtr job,int info,ref Extended data,uint length);
  [System.Runtime.InteropServices.DllImport("kernel32.dll",SetLastError=true)] static extern bool AssignProcessToJobObject(IntPtr job,IntPtr process);
  [System.Runtime.InteropServices.DllImport("kernel32.dll")] static extern bool CloseHandle(IntPtr handle);
  public static OwnedProcess Start(Process p){var owner=new OwnedProcess();owner.job=CreateJobObject(IntPtr.Zero,null);var limit=new Extended();limit.Basic.Flags=0x2000;
   if(owner.job==IntPtr.Zero||!SetInformationJobObject(owner.job,9,ref limit,(uint)System.Runtime.InteropServices.Marshal.SizeOf(limit))){owner.Dispose();throw new Exception("无法建立GM后台清理保护，未启动后台。");}
   try{p.Start();owner.process=p;if(!AssignProcessToJobObject(owner.job,p.Handle))throw new Exception("无法绑定GM后台清理保护。");return owner;}catch{owner.Dispose();throw;}
  }
  public void Dispose(){if(job!=IntPtr.Zero){CloseHandle(job);job=IntPtr.Zero;}if(process!=null){try{if(!process.HasExited){process.Kill();process.WaitForExit(5000);}}catch(InvalidOperationException){}process=null;}}
 }
}

using System;
using System.IO;
using System.Text;
using System.Runtime.InteropServices;
using Microsoft.Win32.SafeHandles;

namespace Local115US {
 public sealed class PackageContext {
  public readonly string Root,Client,Module,StorageConfig,Database,ConfigHash,DatabaseIdentity;
  public PackageContext(string root,string client,bool requireDatabase) {
   Root=Path.GetFullPath(root);if(Root.Length>Path.GetPathRoot(Root).Length)Root=Root.TrimEnd('\\');Client=Path.GetFullPath(client);
   Module=Core.Module(Root);StorageConfig=Path.Combine(Module,"runtime","storage","local.json");
   var d=Core.Read(StorageConfig);string driver=Core.Get(d,"driver",""),db=Core.Get(d,"sqlite_path","");
   if(driver!="sqlite"&&(driver!=""||Core.Get(d,"postgres_dsn","")!=""))throw new Exception("此启动器只管理本地 SQLite 存档，配置未更改。");
   Database=Core.Full(db==""?"runtime/storage/dfolan.sqlite3":db,Module);
   if(Database.StartsWith(@"\\")||Database.StartsWith(@"\\?\"))throw new Exception("本地存档不能位于网络共享。");
   ConfigHash=File.Exists(StorageConfig)?Core.Hash(StorageConfig):"";
   if(requireDatabase&&!File.Exists(Database))throw new Exception("未找到已有存档："+Database+"\nGM不会创建替代空库。新安装请先进入游戏创建角色。");
   DatabaseIdentity=File.Exists(Database)?FileIdentity(Database):"";
  }
  public void AssertUnchanged() {
   string current=File.Exists(StorageConfig)?Core.Hash(StorageConfig):"";
   if(current!=ConfigHash)throw new Exception("存档配置已变化，请重新打开 GM 或备份页；未执行本次操作。");
   if(DatabaseIdentity!=""&&(!File.Exists(Database)||FileIdentity(Database)!=DatabaseIdentity))throw new Exception("数据库文件已替换，请重新连接并核对角色；未执行本次操作。");
  }
  public void AssertDatabase(string path){AssertUnchanged();if(DatabaseIdentity==""||!File.Exists(path)||FileIdentity(path)!=DatabaseIdentity)throw new Exception("GM返回的数据库与当前存档不是同一文件，已拒绝连接。");}
  public string InstanceKey {get{return ProgramHash(DirectoryIdentity(Root));}}
  static string ProgramHash(string value){using(var h=System.Security.Cryptography.SHA256.Create())return BitConverter.ToString(h.ComputeHash(Encoding.UTF8.GetBytes(value))).Replace("-","");}
  [StructLayout(LayoutKind.Sequential)] struct Info {public uint Attributes;public System.Runtime.InteropServices.ComTypes.FILETIME Created,Accessed,Written;public uint Volume,High,Low,Links,IndexHigh,IndexLow;}
  [DllImport("kernel32.dll",CharSet=CharSet.Unicode,SetLastError=true)] static extern SafeFileHandle CreateFile(string name,uint access,uint share,IntPtr security,uint creation,uint flags,IntPtr template);
  [DllImport("kernel32.dll",SetLastError=true)] static extern bool GetFileInformationByHandle(SafeFileHandle file,out Info info);
  static string Identity(string path,bool directory){using(var handle=CreateFile(path,0,7,IntPtr.Zero,3,directory?0x02000000u:0u,IntPtr.Zero)){Info info;if(handle.IsInvalid||!GetFileInformationByHandle(handle,out info))throw new IOException("无法确认文件身份："+path);return info.Volume.ToString("x8")+":"+info.IndexHigh.ToString("x8")+info.IndexLow.ToString("x8");}}
  public static string FileIdentity(string path){return Identity(path,false);}
  public static string DirectoryIdentity(string path){return Identity(path,true);}
 }
}

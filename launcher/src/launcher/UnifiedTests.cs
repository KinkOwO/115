using System;
using System.IO;
using System.Collections.Generic;
using System.Runtime.InteropServices;

namespace Local115US {
 public static class UnifiedTests {
  [DllImport("kernel32.dll",CharSet=CharSet.Unicode,SetLastError=true)]static extern bool CreateHardLink(string link,string existing,IntPtr reserved);
  static readonly List<string> passed=new List<string>();
  static void Check(string name,Action test){test();passed.Add(name);}
  static void Assert(bool value){if(!value)throw new Exception("Assertion failed");}
  static void Refused(Action action){bool refused=false;try{action();}catch{refused=true;}Assert(refused);}
  static void Config(string path,string db){Core.Write(path,new Dictionary<string,object>{{"driver","sqlite"},{"sqlite_path",db},{"unrelated",31}});}
  public static int Run(string directory){
   Directory.CreateDirectory(directory);string sandbox=Path.Combine(directory,"fixture-"+Guid.NewGuid().ToString("N"));Directory.CreateDirectory(sandbox);
   string a=Path.Combine(sandbox,"整合包 A"),b=Path.Combine(sandbox,"整合包 B");Directory.CreateDirectory(a);Directory.CreateDirectory(b);
   string cfg=Path.Combine(Core.Module(a),"runtime","storage","local.json"),db=Path.Combine(a,"自定义 存档","玩家.sqlite3");Directory.CreateDirectory(Path.GetDirectoryName(db));File.WriteAllText(db,"isolated identity fixture only");Config(cfg,db);
   var context=new PackageContext(a,a,true);string before=Core.Hash(cfg);
   Check("custom sqlite_path preserved",()=>Assert(context.Database==db&&Core.Hash(cfg)==before));
   Check("different package has different instance identity",()=>Assert(context.InstanceKey!=new PackageContext(b,b,false).InstanceKey));
   Check("new package initializes only config, never an empty database",()=>{var fresh=new PackageContext(b,b,false);Assert(!File.Exists(fresh.StorageConfig)&&!File.Exists(fresh.Database));Core.InitializeMissingStorage(fresh);Assert(File.Exists(fresh.StorageConfig)&&!File.Exists(fresh.Database));Assert(Core.Get(Core.Read(fresh.StorageConfig),"sqlite_path","")=="runtime/storage/dfolan.sqlite3");});
   Check("existing custom config is never rewritten",()=>{Core.InitializeMissingStorage(context);Assert(Core.Hash(cfg)==before);});
   Check("concurrent config creation preserved",()=>{string c=Path.Combine(sandbox,"Package C");Directory.CreateDirectory(c);var fresh=new PackageContext(c,c,false);Config(fresh.StorageConfig,db);string fingerprint=Core.Hash(fresh.StorageConfig);Core.InitializeMissingStorage(fresh);Assert(Core.Hash(fresh.StorageConfig)==fingerprint);});
   Check("missing database rejected without creating",()=>{Refused(()=>new PackageContext(b,b,true));Assert(!File.Exists(new PackageContext(b,b,false).Database));});
   Check("same file identity accepted",()=>context.AssertDatabase(db));
   Check("hardlink alias same database",()=>{string alias=Path.Combine(a,"alias.sqlite3");Assert(CreateHardLink(alias,db,IntPtr.Zero));context.AssertDatabase(alias);});
   Check("foreign database rejected",()=>{string foreign=Path.Combine(b,"other.sqlite3");File.WriteAllText(foreign,"different file");Refused(()=>context.AssertDatabase(foreign));});
   Check("configuration change rejected",()=>{Config(cfg,Path.Combine(b,"other.sqlite3"));Refused(context.AssertUnchanged);Config(cfg,db);});
   Check("replaced database rejected",()=>{File.Move(db,db+".old");File.WriteAllText(db,"new identity");Refused(context.AssertUnchanged);});
   Check("network database rejected",()=>{Config(cfg,@"\\server\save\player.sqlite3");Refused(()=>new PackageContext(a,a,false));Config(cfg,db);});
   Check("foreign driver rejected",()=>{Core.Write(cfg,new Dictionary<string,object>{{"driver","postgres"}});Refused(()=>new PackageContext(a,a,false));Config(cfg,db);});
   foreach(string p in new[]{"../x","client/../x","C:/x","/x","client//x","client\\x"}){string value=p;Check("manifest path rejection "+value,()=>Refused(()=>ReleaseManifest.Resolve(a,b,value)));}
   Check("client mapping uses selected client",()=>Assert(ReleaseManifest.Resolve(a,b,"client/Script.pvf")==Path.Combine(b,"Script.pvf")));
   Check("missing release manifest allowed",()=>Assert(ReleaseManifest.Validate(a,b).Count>0));
   Check("mismatched release ID allowed",()=>{Core.Write(Path.Combine(a,ReleaseManifest.Name),new Dictionary<string,object>{{"schema",1},{"edition","local"},{"release_id","old"}});Assert(ReleaseManifest.Validate(a,b).Count>0);});
   Check("malformed release manifest allowed",()=>{File.WriteAllText(Path.Combine(a,ReleaseManifest.Name),"broken JSON");Assert(ReleaseManifest.Validate(a,b).Count>0);});
   string plugin=Path.Combine(b,".115us-mods","test.dll");Directory.CreateDirectory(Path.GetDirectoryName(plugin));File.WriteAllText(plugin,"before");
   var manifest=new Dictionary<string,object>{{"schema",1},{"edition","local"},{"release_id",ReleaseBuild.ID},{"files",new object[]{new Dictionary<string,object>{{"path","client/.115us-mods/test.dll"},{"role","plugin"},{"size",6},{"sha256",Core.Hash(plugin)}}}}};
   Check("same-size plugin update is nonblocking",()=>{Core.Write(Path.Combine(a,ReleaseManifest.Name),manifest);File.WriteAllText(plugin,"after!");Assert(ReleaseManifest.Validate(a,b,true).Exists(x=>x.Contains("哈希值不一致")&&x.Contains("继续启动")));});
   Check("missing optional plugin is nonblocking",()=>{File.Delete(plugin);Assert(ReleaseManifest.Validate(a,b).Exists(x=>x.Contains("不存在")));});
   Check("essential game files remain required",()=>Refused(()=>Core.Validate(a,b,true)));
   Check("startup hash warning reaches GUI log and preserves storage",()=>{
    foreach(string file in new[]{Core.Backend(a),Path.Combine(Core.Module(a),"bin","wireprobe-pvf.exe"),Path.Combine(a,"server","work","dfo_probe_tools","probe.exe"),Path.Combine(b,"DFO.exe"),Path.Combine(b,"Script.pvf"),Path.Combine(b,"sk.dat")}){Directory.CreateDirectory(Path.GetDirectoryName(file));File.WriteAllText(file,"before");}
    Core.Write(Path.Combine(Core.Module(a),"configs","pvf-default.json"),new Dictionary<string,object>{{"binary","bin/wireprobe-pvf.exe"}});
    Core.Write(Path.Combine(a,"server","launcher.local.json"),new Dictionary<string,object>{{"client_dir",b}});
    string gameFile=Path.Combine(b,"Script.pvf");string recorded=Core.Hash(gameFile);
    Core.Write(Path.Combine(a,ReleaseManifest.Name),new Dictionary<string,object>{{"schema",1},{"edition","local"},{"release_id",ReleaseBuild.ID},{"files",new object[]{new Dictionary<string,object>{{"path","client/Script.pvf"},{"role","pvf"},{"size",6},{"sha256",recorded}}}}});
    File.WriteAllText(gameFile,"after!");string storageHash=Core.Hash(cfg);var notices=new List<string>();Core.Prepare(a,b,false,notices.Add);
    Assert(notices.Exists(x=>x.Contains("哈希值不一致")&&x.Contains(recorded)&&x.Contains("继续启动"))&&Core.Hash(cfg)==storageHash&&File.ReadAllText(gameFile)=="after!");
   });
   Check("command argument quoting",()=>{Assert(Core.Quote(@"G:\中文 路径\")=="\"G:\\中文 路径\\\\\"");Assert(Core.Quote("a\"b")=="\"a\\\"b\"");});
   Check("client memory notice threshold, hysteresis and peak retained",()=>{var n=new ClientMemoryNotice();long cap=ClientMemoryNotice.LimitBytes;Assert(cap==16L*1024*1024*1024);n.Observe(9L*1024*1024*1024);Assert(!n.NearLimit&&n.ExitHint=="");n.Observe(cap*9/10);Assert(n.NearLimit&&n.Hint(cap*9/10).Contains("接近上限"));n.Observe(cap*86/100);Assert(n.NearLimit);n.Observe(cap*84/100);Assert(!n.NearLimit&&n.ExitHint.Contains("不能单独证明"));});
   Check("client memory notice resets for new session",()=>{var n=new ClientMemoryNotice();Assert(!n.NearLimit&&n.PeakBytes==0&&n.ExitHint=="");n.Observe(-1);Assert(n.PeakBytes==0);});
   File.WriteAllText(Path.Combine(directory,"S16-S17-guard-result.json"),Core.Json.Serialize(new{passed=passed.Count,checks=passed,fixture=sandbox,real_player_data_accessed=false,game_started=false}));Console.WriteLine("PASS "+passed.Count+" package/identity/version guards");return 0;
  }
 }
}

from pathlib import Path
import re
R=Path(__file__).resolve().parents[1]
def edit(rel,fn):
 p=R/rel;s=p.read_text('utf-8-sig');p.write_text(fn(s),encoding='utf-8')
def desktop(s):
 s=re.sub(r'^\[assembly:.*\]\n','',s,flags=re.M)
 a=s.index(' [STAThread] public static int Main(');b=s.index('\n}\n\nclass Choice',a)
 s=s[:a]+s[b:]
 a=s.index(' public static string ExtractBackend()');b=s.index('\n public static void Import',a)
 s=s[:a]+''' public static string ExtractBackend(string root){
  byte[] bytes;using(var stream=Assembly.GetExecutingAssembly().GetManifestResourceStream("backend.exe")){using(var m=new MemoryStream()){stream.CopyTo(m);bytes=m.ToArray();}}
  var hash=Hash(bytes);if(hash!=Local115US.ReleaseBuild.GMHash)throw new Exception("内嵌 GM 后台校验失败。");
  var cache=Path.Combine(Local115US.Core.Module(root),"runtime","tool-cache",hash);Directory.CreateDirectory(cache);var exe=Path.Combine(cache,"gm-local.exe");
  if(!File.Exists(exe)||Hash(File.ReadAllBytes(exe))!=hash){var temp=exe+"."+Guid.NewGuid().ToString("N")+".tmp";File.WriteAllBytes(temp,bytes);try{if(File.Exists(exe))File.Replace(temp,exe,null);else File.Move(temp,exe);}finally{if(File.Exists(temp))File.Delete(temp);}}
  return exe;
 }'''+s[b:]
 s=s.replace('public GMForm(bool preview) {','public GMForm(bool preview,Local115US.PackageContext package) {\n  this.package=package;root=package.Root;')
 s=s.replace('Shown+=async(s,e)=>await Run(Connect);FormClosing+=OnClosing;','TopLevel=false;ShowInTaskbar=false;FormClosing+=OnClosing;')
 s=s.replace('poll.Tick+=async(s,e)=>{await RunBackground(RefreshCloud);};','poll.Tick+=async(s,e)=>{if(!preview&&!String.IsNullOrEmpty(endpoint))await RunBackground(RefreshLocal);};')
 s=s.replace('var address=endpoint.TrimEnd', 'package.AssertUnchanged();\n  var address=endpoint.TrimEnd')
 s=s.replace('static string FindRoot(){', 'string FindRoot(){return package.Root;}\n static string RemovedLegacyRoot(){')
 a=s.index(' static string RemovedLegacyRoot()');b=s.index('\n async Task StartBackend',a);s=s[:a]+s[b:]
 a=s.index(' async Task StartBackend');b=s.index('\n async Task RunBackground',a);s=s[:a]+s[b:]
 s=s.replace('await LoadAccounts();\n }','await LoadAccounts();poll.Start();\n }',1)
 s=s.replace('levelMax=Math.Min(255,Convert.ToDecimal(st["level_max"]));','package.AssertDatabase(Program.Str(st,"database"));levelMax=Math.Min(255,Convert.ToDecimal(st["level_max"]));')
 s=s.replace('bool online=Program.Bool(st,"online");','package.AssertDatabase(Program.Str(st,"database"));bool online=Program.Bool(st,"online");')
 a=s.index(' async void OnClosing(');s=s[:a]+''' async void OnClosing(object s,FormClosingEventArgs e){if(shutdown)return;e.Cancel=true;try{await ShutdownAsync();shutdown=true;Close();}catch(Exception error){state.Text=error.Message;}}
}
'''
 return s
edit('src/gm/Desktop.cs',desktop)
def theme(s):
 s=s.replace('Program.ExtractBackend()','Program.ExtractBackend(root)')
 s=s.replace('Task<ArtResult> Art(uint id){','Task<ArtResult> Art(uint id){if(preview)return Task.FromResult(new ArtResult());')
 s=s.replace('layout.RowStyles.Add(new RowStyle(SizeType.Absolute,30));','layout.RowStyles.Add(new RowStyle(SizeType.Absolute,0));')
 s=s.replace('layout.Controls.Add(WindowChrome(),0,0);','')
 s=s.replace('115US GM 1.4.5','115US GM 1.5 · 统一启动器')
 return s
edit('src/gm/Theme.cs',theme)
def design(s):
 s=s.replace('if(m.Msg!=0x84||WindowState!=FormWindowState.Normal)return;','if(!TopLevel||m.Msg!=0x84||WindowState!=FormWindowState.Normal)return;')
 return s
edit('src/gm/Design.cs',design)
def maintenance(s):
 a=s.index(' internal static string Database(');b=s.index('\n internal static List<Entry> Snapshot()',a)
 s=s[:a]+''' internal static string Database(string root){return new Local115US.PackageContext(root,Local115US.Core.Client(root),true).Database;}
'''+s[b:]
 s=s.replace('if(Program.Cloud||preview)return;','if(Program.Cloud||preview)return;package.AssertUnchanged();')
 s=s.replace('if(backend!=null){if(!backend.HasExited){backend.Kill();backend.WaitForExit(5000);}backend.Dispose();backend=null;}endpoint=token="";gmReady=false;','StopOwnedBackend();')
 return s
edit('src/gm/LocalMaintenance.cs',maintenance)
def launcher(s):
 s=s.replace('public const string ServerHash = "8f2d8ba4884852de078a43102a01966e1cff5f49a0727fa5d146fe370161e2b9";','public const string ServerHash = ReleaseBuild.ServerHash;')
 s=s.replace('public static string Module(string root) { return Path.Combine(root,"server","work","dfo-lan"); }','public static string Module(string root) { string p=Path.Combine(root,"server","work","dfo-lan");return Directory.Exists(p)?p:Path.Combine(root,"02_服务端","server","work","dfo-lan"); }')
 s=s.replace('if(hash&&Hash(Path.Combine(Module(root),"bin","wireprobe-pvf.exe"))!=ServerHash)throw new Exception("服务端与本地版发布版本不一致，请使用完整配套版本。");','if(hash)ReleaseManifest.Validate(root,client);')
 a=s.index('  public static void Prepare(');b=s.index('  public static Process Spawn',a)
 s=s[:a]+'''  public static void Prepare(string root,string client,bool initialize=false) {
   Validate(root,client,true);var context=new PackageContext(root,client,false);
   if(!initialize)return;
   string launch=Path.Combine(root,"server","launcher.local.json");var cfg=Read(launch);
   cfg["client_dir"]=Path.GetFullPath(client);cfg["server_binary"]="work/dfo-lan/bin/wireprobe-pvf.exe";Write(launch,cfg);
   if(!File.Exists(context.StorageConfig))Write(context.StorageConfig,new Dictionary<string,object>{{"driver","sqlite"},{"sqlite_path",context.Database}});
  }
'''+s[b:]
 s=s.replace('public class Shell {','public partial class Shell {')
 s=s.replace('public Shell(string r,string c) {','public Shell(string r,string c,bool previewMode=false) {\n   this.previewMode=previewMode;')
 s=s.replace('Core.Validate(nr,nc,true);root=nr;client=nc;','Core.Validate(nr,nc,true);await DisconnectGM();root=nr;client=nc;')
 a=s.index('   w.Closing+=');b=s.index('\n   RefreshPaths();',a)
 s=s[:a]+'   w.Closing+=Closing;\n   InitializeUnified();'+s[b:]
 s=s.replace('new[]{"LibraryPage","SettingsPage","NotesPage"}','new[]{"LibraryPage","SettingsPage","NotesPage","GMPage","BackupPage","VersionPage"}')
 s=s.replace('new[]{"Library","Settings","Notes"}','new[]{"Library","Settings","Notes","GM","Backup","Version"}')
 s=s.replace('async Task Start(){if(busy||running)return;','async Task Start(){if(busy||running)return;await DisconnectGM();')
 s=s.replace('await Task.Run(()=>Core.Prepare(root,client));string lockPath','await Task.Run(()=>Core.Prepare(root,client,true));string lockPath')
 s=s.replace('async Task Stop(){if(!running||busy)return;','async Task Stop(){if(!running||busy)return;await DisconnectGM();')
 s=s.replace('if(args.Contains("--self-test"))return Tests.Run();','if(args.Contains("--self-test"))return UnifiedTests.Run(Core.Opt(args,"--evidence",Path.Combine(Core.Home,"self-test")));')
 s=s.replace('115US Local Launcher 1.0.9','115US Local + GM Launcher 2.0.0')
 a=s.index('   var app=new Application();');b=s.index('\n  }catch(Exception ex)',a)
 s=s[:a]+'''   Forms.Application.EnableVisualStyles();Forms.Application.SetCompatibleTextRenderingDefault(false);
   bool preview=args.Contains("--render-all");var context=new PackageContext(root,client,false);
   using(var instance=new System.Threading.Mutex(false,"Local\\\\115US.Unified."+context.InstanceKey+(preview?".Preview":""))){
    bool owned=false;try{try{owned=instance.WaitOne(args.Contains("--start")?10000:0);}catch(System.Threading.AbandonedMutexException){owned=true;}
     if(!owned)throw new Exception("这份整合包的启动器已打开，请使用现有窗口。");
     var app=new Application();app.ShutdownMode=ShutdownMode.OnLastWindowClose;var shell=new Shell(root,client,preview);
     if(preview)shell.RenderAll(Core.Opt(args,"--render-all",Path.Combine(Core.Home,"preview")));
     shell.Show(args.Contains("--start"),Core.Opt(args,"--render-preview",null));app.Run();return 0;
    }finally{if(owned)instance.ReleaseMutex();}
   }
'''+s[b:]
 return s
edit('src/launcher/Launcher.cs',launcher)
def xaml(s):
 s=s.replace('Width="1140" Height="760"','Width="1480" Height="940"')
 s=s.replace('LOCAL EDITION   /   1.0.5','LOCAL + GM   /   2.0.0')
 s=s.replace('<Button x:Name="Settings"','<Button x:Name="GM" Style="{StaticResource NavButton}" Content="GM 工作台" Margin="0,10,0,0"/>\n<Button x:Name="Backup" Style="{StaticResource NavButton}" Content="备份与恢复" Margin="0,5,0,0"/>\n<Button x:Name="Version" Style="{StaticResource NavButton}" Content="版本检查" Margin="0,5,0,0"/>\n<Button x:Name="Settings"',1)
 s=s.replace('<Grid x:Name="SettingsPage"','<Grid x:Name="GMPage" Visibility="Collapsed"/>\n<Grid x:Name="BackupPage" Visibility="Collapsed" Margin="28"/>\n<Grid x:Name="VersionPage" Visibility="Collapsed" Margin="28"/>\n<Grid x:Name="SettingsPage"',1)
 return s
edit('src/launcher/Shell.xaml',xaml)
print('Adapted isolated WPF and GM sources')

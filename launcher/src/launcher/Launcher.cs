using System;
using System.Collections.Generic;
using System.Diagnostics;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Security.Cryptography;
using System.Security.Principal;
using System.Text;
using System.Threading.Tasks;
using System.Web.Script.Serialization;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Input;
using System.Windows.Markup;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Threading;
using System.Windows.Shell;
using System.Runtime.InteropServices;
using System.Windows.Interop;
using Forms = System.Windows.Forms;

namespace Local115US {
 public static class Core {
  public const string QQGroup = "517631510";
  public const string ServerHash = ReleaseBuild.ServerHash;
  public static JavaScriptSerializer Json {get{return new JavaScriptSerializer { MaxJsonLength=64*1024*1024 };}}
  public static string Home = AppDomain.CurrentDomain.BaseDirectory.TrimEnd(Path.DirectorySeparatorChar);
  public static string Module(string root) { string p=Path.Combine(root,"server","work","dfo-lan");return Directory.Exists(p)?p:Path.Combine(root,"02_服务端","server","work","dfo-lan"); }
  public static string Backend(string root) { return Path.Combine(Module(root),"bin","dfolauncher.exe"); }
  public static string Full(string p,string basis) { return Path.GetFullPath(Path.IsPathRooted(p)?p:Path.Combine(basis,p)); }
  public static Dictionary<string,object> Read(string path) {
   return File.Exists(path)?Json.Deserialize<Dictionary<string,object>>(File.ReadAllText(path,Encoding.UTF8)):new Dictionary<string,object>();
  }
  public static string Get(Dictionary<string,object> d,string k,string def) { return d.ContainsKey(k)&&d[k]!=null?Convert.ToString(d[k]):def; }
  public static void Write(string p,Dictionary<string,object> d) {
   Directory.CreateDirectory(Path.GetDirectoryName(p)); string tmp=p+"."+Guid.NewGuid().ToString("N")+".tmp";
   File.WriteAllText(tmp,Json.Serialize(d),new UTF8Encoding(false));
   try { if(File.Exists(p)) File.Replace(tmp,p,null); else File.Move(tmp,p); }
   finally { if(File.Exists(tmp)) File.Delete(tmp); }
  }
  // CommandLineToArgvW quoting: preserve spaces, quotes and trailing backslashes.
  public static string Quote(string arg) {
   StringBuilder b=new StringBuilder("\""); int slashes=0;
   foreach(char c in arg) { if(c=='\\') { slashes++;continue; } if(c=='\"') { b.Append('\\',slashes*2+1);b.Append(c); } else { b.Append('\\',slashes);b.Append(c); } slashes=0; }
   b.Append('\\',slashes*2);return b.Append('"').ToString();
  }
  public static string Args(params string[] args) { return String.Join(" ",args.Select(Quote)); }
  public static bool Admin() { return new WindowsPrincipal(WindowsIdentity.GetCurrent()).IsInRole(WindowsBuiltInRole.Administrator); }
  public static string Hash(string p) { using(var f=File.OpenRead(p)) using(var h=SHA256.Create()) return BitConverter.ToString(h.ComputeHash(f)).Replace("-","").ToLowerInvariant(); }
  public static string Client(string root) {
   string p=Path.Combine(root,"server","launcher.local.json");
   if(File.Exists(p)) { var d=Read(p);string c=Get(d,"client_dir","");if(c!="") { string f=Full(c,Path.Combine(root,"server"));if(File.Exists(Path.Combine(f,"DFO.exe")))return f; } }
   foreach(string n in new[]{"client","DFO",@"01_客户端\DFO"}) if(File.Exists(Path.Combine(root,n,"DFO.exe")))return Path.Combine(root,n);
   return Path.Combine(root,"client");
  }
  public static void Validate(string root,string client,bool hash) {
   foreach(string p in new[]{Backend(root),Path.Combine(Module(root),"bin","wireprobe-pvf.exe"),Path.Combine(Module(root),"configs","pvf-default.json"),Path.Combine(root,"server","work","dfo_probe_tools","probe.exe"),Path.Combine(client,"DFO.exe"),Path.Combine(client,"Script.pvf"),Path.Combine(client,"sk.dat")})
    if(!File.Exists(p))throw new Exception("文件缺失："+p+"\n请完整解压压缩包，或在目录设置中选择正确位置。");
   var prof=Read(Path.Combine(Module(root),"configs","pvf-default.json"));
   if(Get(prof,"binary","").Replace('\\','/')!="bin/wireprobe-pvf.exe")throw new Exception("当前目录不是配套的本地版默认配置。");
   if(hash)foreach(string line in ReleaseManifest.Validate(root,client))Console.WriteLine(line);
  }
  public static void Prepare(string root,string client,bool initialize=false) {
   Validate(root,client,true);var context=new PackageContext(root,client,false);
   if(!initialize){AssertCheckClient(root,client);return;}
   string launch=Path.Combine(root,"server","launcher.local.json");var cfg=Read(launch);
   cfg["client_dir"]=PersistedClientPath(root,client);cfg["server_binary"]="work/dfo-lan/bin/wireprobe-pvf.exe";Write(launch,cfg);
   InitializeMissingStorage(context);
  }
  static void AssertCheckClient(string root,string client){
   string configured=Get(Read(Path.Combine(root,"server","launcher.local.json")),"client_dir","");
   if(configured=="")return;
   string actual=Full(configured,Path.Combine(root,"server"));
   if(File.Exists(Path.Combine(actual,"DFO.exe"))&&PackageContext.DirectoryIdentity(actual)!=PackageContext.DirectoryIdentity(client))
    throw new Exception("已更换客户端目录；开始游戏时会应用新目录并重新校验，当前环境检查未完成。");
  }
  static string PersistedClientPath(string root,string client){
   string fullRoot=Path.GetFullPath(root),fullClient=Path.GetFullPath(client);
   string rootPrefix=fullRoot.TrimEnd(Path.DirectorySeparatorChar)+Path.DirectorySeparatorChar;
   if(!fullClient.StartsWith(rootPrefix,StringComparison.OrdinalIgnoreCase)&&!String.Equals(fullClient.TrimEnd(Path.DirectorySeparatorChar),fullRoot.TrimEnd(Path.DirectorySeparatorChar),StringComparison.OrdinalIgnoreCase))return fullClient;
   var server=new Uri(Path.Combine(fullRoot,"server")+Path.DirectorySeparatorChar);
   string relative=Uri.UnescapeDataString(server.MakeRelativeUri(new Uri(fullClient.TrimEnd(Path.DirectorySeparatorChar)+Path.DirectorySeparatorChar)).ToString()).TrimEnd('/');
   return relative==""?".":relative;
  }
  public static void InitializeMissingStorage(PackageContext context){
   if(File.Exists(context.StorageConfig))return;
   Directory.CreateDirectory(Path.GetDirectoryName(context.StorageConfig));string temp=context.StorageConfig+"."+Guid.NewGuid().ToString("N")+".tmp";
   File.WriteAllText(temp,Json.Serialize(new Dictionary<string,object>{{"driver","sqlite"},{"sqlite_path","runtime/storage/dfolan.sqlite3"}}),new UTF8Encoding(false));
   try{if(!File.Exists(context.StorageConfig))File.Move(temp,context.StorageConfig);}finally{if(File.Exists(temp))File.Delete(temp);}
  }
  public static Process Spawn(string root,string verb,Action<string> line,params string[] extra) {
   var argv=new List<string>{verb,"--root",root};argv.AddRange(extra);
   var si=new ProcessStartInfo(Backend(root),Args(argv.ToArray()));si.WorkingDirectory=root;si.UseShellExecute=false;si.CreateNoWindow=true;si.RedirectStandardOutput=true;si.RedirectStandardError=true;si.StandardOutputEncoding=Encoding.UTF8;si.StandardErrorEncoding=Encoding.UTF8;
   // Do not inherit a cloud session's route/profile/account environment.
   RobotLaunch.ClearInherited(si);
   if(verb=="launch"&&!extra.Contains("--check")&&!extra.Contains("--dry-run"))RobotLaunch.Apply(si,root,Client(root));
   var p=new Process();p.StartInfo=si;p.OutputDataReceived+=(s,e)=>{if(e.Data!=null)line(e.Data);};p.ErrorDataReceived+=(s,e)=>{if(e.Data!=null)line(e.Data);};p.Start();p.BeginOutputReadLine();p.BeginErrorReadLine();return p;
  }
  public static int Run(string root,string verb,Action<string> line,params string[] extra) { using(var p=Spawn(root,verb,line,extra)) { p.WaitForExit();return p.ExitCode; } }
  public static string Opt(string[] args,string name,string def) { int i=Array.IndexOf(args,name);return i>=0&&i+1<args.Length?args[i+1]:def; }
 }

 public partial class Shell {
  [StructLayout(LayoutKind.Sequential)] struct NativePoint { public int X,Y; }
  [StructLayout(LayoutKind.Sequential)] struct NativeRect { public int Left,Top,Right,Bottom; }
  [StructLayout(LayoutKind.Sequential)] struct MinMaxInfo { public NativePoint Reserved,MaxSize,MaxPosition,MinTrackSize,MaxTrackSize; }
  [StructLayout(LayoutKind.Sequential)] struct MonitorInfo { public int Size; public NativeRect Monitor,Work; public int Flags; }
  [DllImport("user32.dll")] static extern IntPtr MonitorFromWindow(IntPtr window,uint flags);
  [DllImport("user32.dll",CharSet=CharSet.Auto)] static extern bool GetMonitorInfo(IntPtr monitor,ref MonitorInfo info);
  ClientMemoryNotice memoryNotice=new ClientMemoryNotice();
  Window w; string root,client,logFile; bool busy,running,closing;Process session;FileStream lease;
  TextBlock status,detail,rootLabel,clientLabel;TextBox log,rootBox,clientBox;Button start;ProgressBar progress;
  public Shell(string r,string c,bool previewMode=false) {
   this.previewMode=previewMode;
   root=Path.GetFullPath(r);client=Path.GetFullPath(c);
   using(var stream=Assembly.GetExecutingAssembly().GetManifestResourceStream("Shell.xaml"))w=(Window)XamlReader.Load(stream);
   w.SourceInitialized+=(s,e)=>{var handle=new WindowInteropHelper(w).Handle;HwndSource.FromHwnd(handle).AddHook(WindowMessage);SingleInstance.AllowOpenMessage(handle,root);};
   status=Find<TextBlock>("Status");detail=Find<TextBlock>("Detail");rootLabel=Find<TextBlock>("RootLabel");clientLabel=Find<TextBlock>("ClientLabel");log=Find<TextBox>("Log");rootBox=Find<TextBox>("RootBox");clientBox=Find<TextBox>("ClientBox");start=Find<Button>("Start");progress=Find<ProgressBar>("Progress");
   var image=new BitmapImage();image.BeginInit();image.StreamSource=Assembly.GetExecutingAssembly().GetManifestResourceStream("hero.jpg");image.CacheOption=BitmapCacheOption.OnLoad;image.EndInit();Find<Image>("HeroImage").Source=image;
   logFile=Path.Combine(Core.Home,"launcher-logs",DateTime.Now.ToString("yyyyMMdd-HHmmss")+".log");
   Bind("Start",async()=>await Start());Bind("Check",async()=>await Check());Bind("Stop",async()=>await Stop());
   Bind("Library",()=>{Page("LibraryPage");return Task.FromResult(0);});Bind("Settings",()=>{Page("SettingsPage");return Task.FromResult(0);});Bind("Notes",()=>{Page("NotesPage");return Task.FromResult(0);});
   Bind("BrowseRoot",()=>{using(var d=new Forms.FolderBrowserDialog()){d.Description="选择含 server 文件夹的本地版整合包目录";d.SelectedPath=rootBox.Text;if(d.ShowDialog()==Forms.DialogResult.OK)rootBox.Text=d.SelectedPath;}return Task.FromResult(0);});
   Bind("BrowseClient",()=>{var d=new Microsoft.Win32.OpenFileDialog{Title="选择客户端 DFO.exe",Filter="游戏客户端 (DFO.exe)|DFO.exe",InitialDirectory=Directory.Exists(clientBox.Text)?clientBox.Text:root};if(d.ShowDialog(w)==true)clientBox.Text=Path.GetDirectoryName(d.FileName);return Task.FromResult(0);});
   Bind("SavePaths",async()=>{if(busy||running)throw new Exception("游戏运行或检查时不能修改目录。");string nr=Core.Full(rootBox.Text,Core.Home);string nc=Core.Full(clientBox.Text,nr);Core.Validate(nr,nc,true);if(await SwitchPackage(nr,nc))return;await DisconnectGM();root=nr;client=nc;SaveChoice();RefreshPaths();await Check();});
   Bind("OpenRoot",()=>{Process.Start("explorer.exe",Core.Quote(root));return Task.FromResult(0);});
   Bind("ShowLog",()=>{var v=Find<Border>("LogPanel");v.Visibility=v.Visibility==Visibility.Visible?Visibility.Collapsed:Visibility.Visible;return Task.FromResult(0);});
   Bind("CloseLog",()=>{Find<Border>("LogPanel").Visibility=Visibility.Collapsed;return Task.FromResult(0);});
   Find<TextBlock>("QQGroupNumber").Text=Core.QQGroup;
   Bind("CopyQQGroup",async()=>{Clipboard.SetText(Core.QQGroup);Find<TextBlock>("QQGroupHint").Text="群号已复制";await Task.Delay(1800);Find<TextBlock>("QQGroupHint").Text="点击复制群号";});
   Bind("Min",()=>{SystemCommands.MinimizeWindow(w);return Task.FromResult(0);});Bind("Max",()=>{ToggleMaximize();return Task.FromResult(0);});Bind("Close",()=>{w.Close();return Task.FromResult(0);});
   w.StateChanged+=(s,e)=>{bool maximized=w.WindowState==WindowState.Maximized;Find<System.Windows.Shapes.Path>("MaxIcon").Data=Geometry.Parse(maximized?"M 3,3 L 3,1 12,1 12,10 10,10 M 1,4 L 10,4 10,13 1,13 Z":"M 1,1 L 12,1 12,12 1,12 Z");Find<Button>("Max").ToolTip=maximized?"还原":"最大化";};
   Find<Border>("TitleBar").MouseLeftButtonDown+=(s,e)=>{if(e.ClickCount==2)ToggleMaximize();else w.DragMove();};
   w.Closing+=Closing;
   InitializeUnified();
   Find<TextBlock>("ClientMemoryHint").Text=ClientMemoryNotice.DefaultHint;
   RefreshPaths();Page("LibraryPage");SetState("准备开始","点击开始游戏，登录器会自动检查并启动本地环境。",false);
  }
  T Find<T>(string name) where T:class { return (T)w.FindName(name); }
  IntPtr WindowMessage(IntPtr hwnd,int msg,IntPtr wp,IntPtr lp,ref bool handled){if(HandleOpenMessage(msg,wp)){handled=true;return IntPtr.Zero;}if(msg==0x24){var info=new MonitorInfo();info.Size=Marshal.SizeOf(typeof(MonitorInfo));if(GetMonitorInfo(MonitorFromWindow(hwnd,2),ref info)){var mm=(MinMaxInfo)Marshal.PtrToStructure(lp,typeof(MinMaxInfo));mm.MaxPosition.X=info.Work.Left-info.Monitor.Left;mm.MaxPosition.Y=info.Work.Top-info.Monitor.Top;mm.MaxSize.X=info.Work.Right-info.Work.Left;mm.MaxSize.Y=info.Work.Bottom-info.Work.Top;var source=HwndSource.FromHwnd(hwnd);var transform=source.CompositionTarget.TransformToDevice;mm.MinTrackSize.X=(int)Math.Ceiling(w.MinWidth*transform.M11);mm.MinTrackSize.Y=(int)Math.Ceiling(w.MinHeight*transform.M22);Marshal.StructureToPtr(mm,lp,false);handled=true;}}return IntPtr.Zero;}
  void Bind(string name,Func<Task> action) { Find<Button>(name).Click+=async(s,e)=>{try{await action();}catch(Exception ex){Error(ex);}}; }
  void ToggleMaximize(){if(w.WindowState==WindowState.Maximized)SystemCommands.RestoreWindow(w);else SystemCommands.MaximizeWindow(w);}
  void Page(string name) { foreach(string n in new[]{"LibraryPage","SettingsPage","NotesPage","GMPage","BackupPage","VersionPage"})Find<Grid>(n).Visibility=n==name?Visibility.Visible:Visibility.Collapsed;foreach(string n in new[]{"Library","Settings","Notes","GM","Backup","Version"})Find<Button>(n).Tag=n+"Page"==name?"selected":"";Find<Border>("LogPanel").Visibility=Visibility.Collapsed; }
  void RefreshPaths(){rootLabel.Text=root;clientLabel.Text=client;rootBox.Text=root;clientBox.Text=client;RefreshRobotToggle();}
  void SaveChoice(){Core.Write(Path.Combine(Core.Home,"local-launcher.settings.json"),new Dictionary<string,object>{{"root",root==Core.Home?".":root},{"client",client.StartsWith(root+"\\",StringComparison.OrdinalIgnoreCase)?client.Substring(root.Length+1):client}});}
  void SetState(string title,string message,bool active){status.Text=title;detail.Text=message;progress.Visibility=active?Visibility.Visible:Visibility.Collapsed;start.IsEnabled=!busy&&!running;start.Content=running?(active?"正在启动…":"游戏运行中"):(busy?"请稍候…":"开始游戏");Find<Button>("Check").IsEnabled=!busy&&!running;Find<Button>("Stop").IsEnabled=running&&!busy;RefreshRobotToggle();}
  public void Append(string line){w.Dispatcher.BeginInvoke(new Action(()=>{string txt=DateTime.Now.ToString("HH:mm:ss")+"  "+line+Environment.NewLine;log.AppendText(txt);if(log.Text.Length>80000)log.Text=log.Text.Substring(log.Text.Length-60000);log.ScrollToEnd();try{Directory.CreateDirectory(Path.GetDirectoryName(logFile));File.AppendAllText(logFile,txt,Encoding.UTF8);}catch{}}));}
  void Error(Exception ex){Append(ex.ToString());SetState("需要处理",ex.Message,false);MessageBox.Show(w,ex.Message,"本地版提示",MessageBoxButton.OK,MessageBoxImage.Information);}
  async Task Check(){if(busy||running)return;busy=true;SetState("正在检查","核对客户端、服务端与本地存档目录…",true);try{await Task.Run(()=>{Core.Prepare(root,client);int code=Core.Run(root,"launch",Append,"--check");if(code!=0)throw new Exception("环境检查未通过，请打开运行日志查看缺失文件或配置。");});SaveChoice();busy=false;SetState("可以开始游戏","本地版环境检查通过，存档保存在当前整合包内。",false);}finally{busy=false;start.IsEnabled=!running;Find<Button>("Check").IsEnabled=!running;progress.Visibility=Visibility.Collapsed;}}
  async Task Start(){if(busy||running)return;await DisconnectGM();if(!Core.Admin()){var si=new ProcessStartInfo(Assembly.GetExecutingAssembly().Location,Core.Args("--root",root,"--client",client,"--start"));si.WorkingDirectory=Core.Home;si.UseShellExecute=true;si.Verb="runas";try{Process.Start(si);closing=true;w.Close();}catch(System.ComponentModel.Win32Exception){SetState("等待开始","游戏网络隔离需要管理员权限，请在系统提示中允许。",false);}return;}
   memoryNotice=new ClientMemoryNotice();Find<TextBlock>("ClientMemoryHint").Text=ClientMemoryNotice.DefaultHint;
   busy=true;SetState("正在启动","正在加载本地服务端，首次启动需要一些时间…",true);
   try{await Task.Run(()=>Core.Prepare(root,client,true));string lockPath=Path.Combine(Core.Module(root),"runtime","local-launcher.session.lock");Directory.CreateDirectory(Path.GetDirectoryName(lockPath));lease=new FileStream(lockPath,FileMode.OpenOrCreate,FileAccess.ReadWrite,FileShare.None);SaveChoice();running=true;busy=false;SetState("正在启动游戏","正在加载资源并等待游戏窗口，请勿重复启动。",true);
    session=Core.Spawn(root,"launch",Append);var timer=new DispatcherTimer{Interval=TimeSpan.FromSeconds(2)};timer.Tick+=(s,e)=>{if(!running){timer.Stop();return;}try{foreach(var game in Process.GetProcessesByName("DFO")){using(game){try{if(!String.Equals(game.MainModule.FileName,Path.Combine(client,"DFO.exe"),StringComparison.OrdinalIgnoreCase))continue;long bytes=game.PrivateMemorySize64;bool wasNear=memoryNotice.NearLimit;memoryNotice.Observe(bytes);Find<TextBlock>("ClientMemoryHint").Text=memoryNotice.Hint(bytes);if(memoryNotice.NearLimit&&!wasNear)Append(memoryNotice.Hint(bytes));SetState(memoryNotice.NearLimit?"客户端内存接近上限":"游戏运行中",memoryNotice.NearLimit?"建议完成当前操作后正常退出并重新进入游戏。":"退出游戏后，本地服务端会自动停止。",false);}catch{}}}}catch{}};timer.Start();
    int exit=await Task.Run(()=>{session.WaitForExit();return session.ExitCode;});timer.Stop();running=false;session.Dispose();session=null;if(lease!=null){lease.Dispose();lease=null;}string memoryExit=memoryNotice.ExitHint;if(memoryExit!=""){Find<TextBlock>("ClientMemoryHint").Text=memoryExit;Append(memoryExit);}SetState(exit==0?"游戏已结束":"启动未完成",memoryExit!=""?memoryExit:(exit==0?"本地游戏环境已停止，可以再次开始。":"请打开运行日志查看原因；不会删除或重置存档。"),false);
   }catch{running=false;busy=false;if(lease!=null){lease.Dispose();lease=null;}throw;}
  }
  async Task Stop(){if(!running||busy)return;await DisconnectGM();busy=true;SetState("正在停止","正在安全关闭本地游戏环境…",true);try{int code=await Task.Run(()=>Core.Run(root,"stop",Append));if(code!=0)throw new Exception("停止环境未成功，请查看运行日志。");if(session!=null&&!session.HasExited)await Task.Run(()=>session.WaitForExit(15000));}finally{busy=false;SetState(running?"等待退出":"游戏已停止",running?"请等待游戏进程退出。":"可以再次开始游戏。",false);}}
  public void Show(bool autoStart,string preview){w.ContentRendered+=async(s,e)=>{if(preview!=null){try{w.UpdateLayout();var bitmap=new RenderTargetBitmap((int)w.ActualWidth,(int)w.ActualHeight,96,96,PixelFormats.Pbgra32);bitmap.Render(w);var enc=new PngBitmapEncoder();enc.Frames.Add(BitmapFrame.Create(bitmap));using(var f=File.Create(preview))enc.Save(f);}finally{closing=true;w.Close();}}else if(autoStart){try{await Start();}catch(Exception ex){Error(ex);}}};w.Show();}
 }
 public class Program {
  [STAThread] public static int Main(string[] args){try{
   try { Console.SetOut(new StreamWriter(Console.OpenStandardOutput(),new UTF8Encoding(false)){AutoFlush=true}); Console.SetError(new StreamWriter(Console.OpenStandardError(),new UTF8Encoding(false)){AutoFlush=true}); } catch { }
   if(args.Contains("--self-test"))return UnifiedTests.Run(Core.Opt(args,"--evidence",Path.Combine(Core.Home,"self-test")));
   var settings=Core.Read(Path.Combine(Core.Home,"local-launcher.settings.json"));string root=Core.Full(Core.Opt(args,"--root","."),Core.Home);
   bool settingsHere=String.Equals(Core.Full(Core.Get(settings,"root","."),Core.Home),root,StringComparison.OrdinalIgnoreCase);
   string client=Core.Opt(args,"--client",settingsHere?Core.Get(settings,"client",""):"");client=client==""?Core.Client(root):Core.Full(client,root);
   if(args.Contains("--verify-release")){Console.WriteLine(Core.Json.Serialize(new{ok=true,release_id=ReleaseBuild.ID,files=ReleaseManifest.Validate(root,client)}));return 0;}
   if(args.Contains("--check")||args.Contains("--plan")){Core.Prepare(root,client);int code=Core.Run(root,"launch",Console.WriteLine,args.Contains("--plan")?"--dry-run":"--check");return code;}
   if(args.Contains("--help")){Console.WriteLine("115US Local + GM Launcher 2.0.5 | --root <package> --client <client> --check | --plan | --self-test");return 0;}
   Forms.Application.EnableVisualStyles();Forms.Application.SetCompatibleTextRenderingDefault(false);
   bool preview=args.Contains("--render-all");var context=new PackageContext(root,client,false);
   using(var instance=new System.Threading.Mutex(false,"Local\\115US.Unified."+context.InstanceKey+(preview?".Preview":""))){
    bool owned=false;try{try{owned=instance.WaitOne(args.Contains("--start")?10000:0);}catch(System.Threading.AbandonedMutexException){owned=true;}
     if(!owned){SingleInstance.Notify(root,args.Contains("--gm"));return 0;}
     var app=new Application();app.ShutdownMode=ShutdownMode.OnLastWindowClose;var shell=new Shell(root,client,preview);
     if(args.Contains("--gm"))shell.OpenGMOnShow();
     if(preview)shell.RenderAll(Core.Opt(args,"--render-all",Path.Combine(Core.Home,"preview")));
     shell.Show(args.Contains("--start"),Core.Opt(args,"--render-preview",null));app.Run();return 0;
    }finally{if(owned)instance.ReleaseMutex();}
   }

  }catch(Exception ex){if(args.Length>0)Console.Error.WriteLine(ex);else MessageBox.Show(ex.Message,"115US 本地版登录器");return 1;}}
 }
 public static class Tests {
  static void Assert(bool ok,string msg){if(!ok)throw new Exception("SELFTEST: "+msg);}
  public static int Run(){string tmp=Path.Combine(Path.GetTempPath(),"115us launcher test "+Guid.NewGuid().ToString("N"));Directory.CreateDirectory(tmp);
   string path=Path.Combine(tmp,"中文目录","settings.json");Core.Write(path,new Dictionary<string,object>{{"unknown",42},{"client_dir",@"E:\游戏 目录\client"}});var d=Core.Read(path);d["server_binary"]="local";Core.Write(path,d);var r=Core.Read(path);Assert(Convert.ToInt32(r["unknown"])==42,"unknown fields retained");Assert(Core.Get(r,"client_dir","")==@"E:\游戏 目录\client","unicode paths retained");
   Assert(Core.Quote(@"C:\游戏 目录\")=="\"C:\\游戏 目录\\\\\"","trailing slash quoting");Assert(Core.Quote("a\"b")=="\"a\\\"b\"","quote escaping");Assert(Core.Full("client",tmp)==Path.Combine(tmp,"client"),"relative client relocates");
   bool refused=false;try{Core.Validate(tmp,tmp,false);}catch{refused=true;}Assert(refused,"incomplete packages rejected");Console.WriteLine("PASS: Unicode paths, atomic settings, unknown fields, argument quoting, relocation, missing-file rejection.");return 0;
  }
 }
}

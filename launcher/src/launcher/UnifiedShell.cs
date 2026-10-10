using System;
using System.IO;
using System.Linq;
using System.Collections.Generic;
using System.Diagnostics;
using System.Threading.Tasks;
using System.Windows;
using System.Windows.Controls;
using System.Windows.Media;
using System.Windows.Media.Imaging;
using System.Windows.Forms.Integration;
using Forms=System.Windows.Forms;

namespace Local115US {
 public partial class Shell {
  [System.Runtime.InteropServices.DllImport("user32.dll")]static extern IntPtr GetParent(IntPtr window);
  [System.Runtime.InteropServices.DllImport("user32.dll")]static extern IntPtr GetAncestor(IntPtr window,uint flags);
  bool previewMode;
  GMForm gm;
  Task gmOpening;
  WindowsFormsHost gmHost;
  Forms.Panel gmViewport;
  TextBox versionReport,backupReport;
  ListBox backupList;
  ComboBox backupSource;
  uint openMessage;
  TextBlock saveLabel;
  CheckBox townRobots;
  TextBlock robotHint;
  bool robotChoice;
  readonly List<Button> backupActions=new List<Button>();
  void InitializeUnified(){
   openMessage=SingleInstance.Message(root);
   gmHost=new WindowsFormsHost();gmViewport=new Forms.Panel{AutoScroll=true};gmHost.Child=gmViewport;Find<Grid>("GMPage").Children.Add(gmHost);
   Bind("GM",async()=>{Page("GMPage");await OpenGM();});
   Bind("Backup",async()=>{Page("BackupPage");if(!previewMode)await BackupAction(LoadBackups);});
   Bind("Version",async()=>{Page("VersionPage");await VersionCheck();});
   MakeVersionPage();MakeBackupPage();InitializeRobotToggle();
  }
  void InitializeRobotToggle(){
   townRobots=Find<CheckBox>("TownRobots");robotHint=Find<TextBlock>("RobotHint");RefreshRobotToggle();
   townRobots.Click+=(s,e)=>{
    if(busy||running){townRobots.IsChecked=robotChoice;return;}
    bool selected=townRobots.IsChecked==true;
    try{if(!previewMode)RobotLaunch.SetEnabled(root,selected);robotChoice=selected;RobotToggleText();}
    catch(Exception ex){townRobots.IsChecked=robotChoice;MessageBox.Show(w,"假人开关未保存："+ex.Message,"本地版提示",MessageBoxButton.OK,MessageBoxImage.Information);}
   };
  }
  void RefreshRobotToggle(){
   if(townRobots==null)return;
   try{robotChoice=previewMode||RobotLaunch.Enabled(root);townRobots.IsChecked=robotChoice;townRobots.IsEnabled=!busy&&!running;RobotToggleText();}
   catch(Exception ex){townRobots.IsChecked=false;townRobots.IsEnabled=false;robotHint.Text="假人开关配置无法读取："+ex.Message;}
  }
  void RobotToggleText(){
   townRobots.Content=robotChoice?"已开启":"已关闭";
   robotHint.Text="不影响精锐小队 · 下次启动游戏生效 · 保留假人养成进度";
  }
  async Task OpenGM(){if(gmOpening!=null){await gmOpening;return;}gmOpening=OpenGMCore();try{await gmOpening;}finally{gmOpening=null;}}
  async Task OpenGMCore(){
   if(busy&&!running)throw new Exception("正在处理启动或备份，请稍后打开GM。");
   if(gm==null){
    // GM validates its backend/save independently of client resources.
    var context=new PackageContext(root,client,!previewMode);gm=new GMForm(previewMode,context);
    gmViewport.Controls.Add(gm);gm.Location=new System.Drawing.Point(0,0);gm.Show();
    Action resize=()=>{if(gm!=null){gm.Size=new System.Drawing.Size(Math.Max(1120,gmViewport.ClientSize.Width-2),Math.Max(680,gmViewport.ClientSize.Height-2));gmViewport.AutoScrollMinSize=gm.Size;}};
    gmViewport.Resize+=(s,e)=>resize();resize();await gm.ActivateAsync();
   }
  }
  bool HandleOpenMessage(int message,IntPtr parameter){if((uint)message!=openMessage)return false;w.Dispatcher.BeginInvoke(new Action(async()=>{if(w.WindowState==WindowState.Minimized)w.WindowState=WindowState.Normal;w.Activate();if(parameter.ToInt64()==1){try{Page("GMPage");await OpenGM();}catch(Exception ex){Error(ex);}}}));return true;}
  public void OpenGMOnShow(){w.ContentRendered+=async(s,e)=>{try{Page("GMPage");await OpenGM();}catch(Exception ex){Error(ex);}};}
  async Task<bool> SwitchPackage(string nextRoot,string nextClient){
   if(new PackageContext(root,client,false).InstanceKey==new PackageContext(nextRoot,nextClient,false).InstanceKey)return false;
   await DisconnectGM();string exe=Path.Combine(nextRoot,Path.GetFileName(System.Reflection.Assembly.GetExecutingAssembly().Location));
   Process.Start(new ProcessStartInfo(exe,Core.Args("--root",nextRoot,"--client",nextClient)){UseShellExecute=false,WorkingDirectory=nextRoot});closing=true;w.Close();return true;
  }
  async Task DisconnectGM(){
   if(gmOpening!=null)throw new Exception("GM正在连接，请等待连接完成后再继续。");
   if(gm==null)return;await gm.ShutdownAsync();gmViewport.Controls.Remove(gm);gm.Dispose();gm=null;
  }
  async void Closing(object sender,System.ComponentModel.CancelEventArgs e){
   if(closing)return;e.Cancel=true;
   try{
    if(busy)throw new Exception("当前操作尚未完成，请等待结果后再退出。");
    if(gm!=null&&gm.IsBusy)throw new Exception("GM操作尚未完成，请等待回执后再退出。");
    if(running){MessageBox.Show(w,"请先在游戏中正常退出，服务端停止后再关闭启动器。GM页和首页可以自由切换。","游戏仍在运行",MessageBoxButton.OK,MessageBoxImage.Information);return;}
    await DisconnectGM();closing=true;w.Close();
   }catch(Exception ex){MessageBox.Show(w,ex.Message,"暂时无法关闭",MessageBoxButton.OK,MessageBoxImage.Information);}
  }
  TextBox ReportBox(){return new TextBox{IsReadOnly=true,TextWrapping=TextWrapping.Wrap,VerticalScrollBarVisibility=ScrollBarVisibility.Auto,FontSize=13,Margin=new Thickness(0,16,0,0)};}
  TextBlock Heading(string text){return new TextBlock{Text=text,FontSize=25,FontWeight=FontWeights.Bold,Margin=new Thickness(0,0,0,14)};}
  Button ActionButton(string text,Func<Task> action){var b=new Button{Content=text,Margin=new Thickness(0,0,10,0),Padding=new Thickness(16,10,16,10)};b.Click+=async(s,e)=>{try{await action();}catch(Exception ex){MessageBox.Show(w,ex.Message,"本地版提示",MessageBoxButton.OK,MessageBoxImage.Information);}};return b;}
  void MakeVersionPage(){
   var grid=Find<Grid>("VersionPage");grid.RowDefinitions.Add(new RowDefinition{Height=GridLength.Auto});grid.RowDefinitions.Add(new RowDefinition{Height=GridLength.Auto});grid.RowDefinitions.Add(new RowDefinition());
   grid.Children.Add(Heading("版本与配套检查"));
   var row=new StackPanel{Orientation=Orientation.Horizontal};row.Children.Add(ActionButton("重新核对文件",VersionCheck));row.Children.Add(new TextBlock{Text="本地版 + GM  2.0.6  /  "+ReleaseBuild.ID,VerticalAlignment=VerticalAlignment.Center,Foreground=Brushes.Goldenrod});Grid.SetRow(row,1);grid.Children.Add(row);
   versionReport=ReportBox();Grid.SetRow(versionReport,2);grid.Children.Add(versionReport);
  }
  async Task VersionCheck(){
   if(previewMode){versionReport.Text="预览模式：不读取游戏或存档。\n统一启动器、GM、网关和插件版本将在此按配套清单核对。";return;}
   versionReport.Text="正在核对配套文件，请稍候…";
   try{var lines=await Task.Run(()=>ReleaseManifest.Validate(root,client,true));var context=new PackageContext(root,client,false);versionReport.Text="组件检查完成（差异仅提示）\n发行版："+ReleaseBuild.ID+"\n当前存档："+context.Database+"\n\n"+String.Join("\n",lines);}
   catch(Exception ex){versionReport.Text="检查未通过\n"+ex.Message;}
  }
  void MakeBackupPage(){
   var grid=Find<Grid>("BackupPage");foreach(var h in new[]{GridLength.Auto,GridLength.Auto,GridLength.Auto,new GridLength(1,GridUnitType.Star),new GridLength(1,GridUnitType.Star)})grid.RowDefinitions.Add(new RowDefinition{Height=h});
   grid.Children.Add(Heading("存档备份与恢复"));saveLabel=new TextBlock{Text="备份恢复只针对当前整合包选择的存档。",TextWrapping=TextWrapping.Wrap,Margin=new Thickness(0,0,0,14)};Grid.SetRow(saveLabel,1);grid.Children.Add(saveLabel);
   var row=new WrapPanel();backupSource=new ComboBox{Width=200,Margin=new Thickness(0,0,12,0),FontSize=13};backupSource.Items.Add("启动器备份");backupSource.Items.Add("GM 自动备份");backupSource.SelectedIndex=0;row.Children.Add(backupSource);
   foreach(var pair in new[]{new KeyValuePair<string,Func<Task>>("刷新列表",LoadBackups),new KeyValuePair<string,Func<Task>>("立即备份",CreateBackup),new KeyValuePair<string,Func<Task>>("恢复选中备份",RestoreBackup)}){var action=pair.Value;var button=ActionButton(pair.Key,()=>BackupAction(action));backupActions.Add(button);row.Children.Add(button);}
   Grid.SetRow(row,2);grid.Children.Add(row);backupList=new ListBox{Margin=new Thickness(0,16,0,0),Background=new SolidColorBrush(Color.FromRgb(29,33,41)),Foreground=Brushes.White,FontSize=13};Grid.SetRow(backupList,3);grid.Children.Add(backupList);
   backupReport=ReportBox();Grid.SetRow(backupReport,4);grid.Children.Add(backupReport);
   backupSource.SelectionChanged+=async(s,e)=>{backupList.Items.Clear();if(!previewMode&&backupReport!=null)await BackupAction(LoadBackups);};
  }
  string Source {get{return backupSource.SelectedIndex==0?"storage":"gm-local";}}
  async Task BackupAction(Func<Task> action){
   if(previewMode)return;if(busy){backupReport.Text="启动或备份操作正在进行，请稍候。";return;}busy=true;foreach(var b in backupActions)b.IsEnabled=false;backupSource.IsEnabled=false;
   try{await action();}catch(Exception ex){backupReport.Text=ex.Message;}finally{busy=false;foreach(var b in backupActions)b.IsEnabled=true;backupSource.IsEnabled=true;start.IsEnabled=!running;}
  }
  sealed class BackupRow {public string Source,Name,Directory,ManifestHash;public PackageContext Context;public override string ToString(){return (Source=="storage"?"启动器":"GM自动")+"  |  "+Name+"\n"+Directory;}}
  async Task LoadBackups(){
   var context=new PackageContext(root,client,true);saveLabel.Text="当前存档："+context.Database+"\n恢复前会验证备份，并自动保存当前进度；恢复操作需先退出游戏。";
   var data=await StorageTool(context,"--list",Source,null,false);context.AssertDatabase(Core.Get(data,"database",""));
   if(Core.Get(data,"source","")!=Source)throw new Exception("备份来源返回不一致，已拒绝显示。");
   backupList.Items.Clear();object rows;if(data.TryGetValue("backups",out rows)&&rows!=null)foreach(var item in (System.Collections.IEnumerable)rows){var d=(Dictionary<string,object>)item;string dir=Core.Get(d,"directory",""),name=Core.Get(d,"name","");
    var baseDir=Source=="storage"?Path.Combine(Path.GetDirectoryName(context.Database),"backups"):Path.Combine(Path.GetDirectoryName(context.Database),"gm-backups","local");
    var expected=Path.Combine(baseDir,name);if(Path.GetFileName(name)!=name||String.IsNullOrEmpty(name)||!String.Equals(Path.GetFullPath(dir),Path.GetFullPath(expected),StringComparison.OrdinalIgnoreCase))throw new Exception("备份目录不属于所选来源，拒绝恢复。");
    backupList.Items.Add(new BackupRow{Source=Source,Name=name,Directory=dir,Context=context,ManifestHash=Core.Hash(Path.Combine(dir,"manifest.json"))});
   }
   backupReport.Text=backupList.Items.Count+" 份备份。请选择需要恢复的时间；GM 自动备份与启动器备份分别列出。";
  }
  async Task CreateBackup(){await DisconnectGM();var context=new PackageContext(root,client,true);var result=await StorageTool(context,"--backup",Source,null,false);await LoadBackups();backupReport.Text="备份已完成。\n"+Core.Json.Serialize(result);}
  async Task RestoreBackup(){
   var row=backupList.SelectedItem as BackupRow;if(row==null)throw new Exception("请先选中一份备份。");if(row.Source!=Source)throw new Exception("备份来源已变化，请重新刷新列表。");row.Context.AssertUnchanged();
   if(Core.Hash(Path.Combine(row.Directory,"manifest.json"))!=row.ManifestHash)throw new Exception("备份清单已变化，请刷新列表后核对。");
   await DisconnectGM();var plan=await StorageTool(row.Context,"--restore",row.Source,row.Name,true);
   backupReport.Text=Core.Json.Serialize(plan);
   if(MessageBox.Show(w,"恢复将用所选备份替换当前存档。\n\n存档："+row.Context.Database+"\n来源："+(row.Source=="storage"?"启动器备份":"GM 自动备份")+"\n备份："+row.Name+"\n\n恢复前会自动备份当前进度。确认恢复？","恢复存档",MessageBoxButton.YesNo,MessageBoxImage.Warning)!=MessageBoxResult.Yes)return;
   row.Context.AssertUnchanged();if(Core.Hash(Path.Combine(row.Directory,"manifest.json"))!=row.ManifestHash)throw new Exception("备份清单已变化，取消恢复。");
   var result=await StorageTool(row.Context,"--restore",row.Source,row.Name,false);backupList.Items.Clear();backupReport.Text=Core.Json.Serialize(result);saveLabel.Text="恢复操作已返回。请核对结果后重新打开GM，重新读取角色。";
  }
  async Task<Dictionary<string,object>> StorageTool(PackageContext context,string action,string source,string name,bool dry){
   context.AssertUnchanged();await Task.Run(()=>Core.Validate(root,client,true));
   var args=new List<string>{"--source",source,"--expected-database",context.Database,action};if(name!=null)args.Add(name);if(dry)args.Add("--dry-run");
   return await Task.Run(()=>{var lines=new List<string>();var gate=new object();int code=Core.Run(root,"storage-sync",s=>{lock(gate)lines.Add(s);},args.ToArray());
    Dictionary<string,object> result=null;foreach(string line in lines){if(!line.TrimStart().StartsWith("{"))continue;try{var parsed=Core.Json.Deserialize<Dictionary<string,object>>(line);if(parsed.ContainsKey("ok"))result=parsed;}catch{}}
    if(code!=0||result==null||!result.ContainsKey("ok")||!Convert.ToBoolean(result["ok"]))throw new Exception(result!=null?Core.Get(result,"error","备份工具拒绝操作"):"备份工具未返回有效结果："+String.Join("\n",lines));return result;});
  }
  public void RenderAll(string directory){
   w.ShowInTaskbar=false;w.WindowStartupLocation=WindowStartupLocation.Manual;w.Left=-20000;w.Top=-20000;
   w.ContentRendered+=async(s,e)=>{try{
    Directory.CreateDirectory(directory);await OpenGM();await Task.Delay(150);
    Page("LibraryPage");var homeSizes=new[]{new System.Drawing.Size(1480,940),new System.Drawing.Size(1000,680)};
    foreach(var size in homeSizes){
     w.Width=size.Width;w.Height=size.Height;
     foreach(bool enabled in new[]{true,false}){
      townRobots.IsChecked=enabled;townRobots.RaiseEvent(new RoutedEventArgs(System.Windows.Controls.Primitives.ButtonBase.ClickEvent));w.UpdateLayout();await Task.Delay(40);
      var togglePoint=townRobots.TranslatePoint(new Point(0,0),w);var startPoint=start.TranslatePoint(new Point(0,0),w);
      if(togglePoint.Y<0||togglePoint.Y+townRobots.ActualHeight>w.ActualHeight||startPoint.Y+start.ActualHeight>w.ActualHeight||robotChoice!=enabled)throw new Exception("Robot toggle layout or interaction failed");
      var home=new RenderTargetBitmap((int)w.ActualWidth,(int)w.ActualHeight,96,96,PixelFormats.Pbgra32);home.Render(w);var png=new PngBitmapEncoder();png.Frames.Add(BitmapFrame.Create(home));using(var f=File.Create(Path.Combine(directory,"home-"+size.Width+"-robots-"+(enabled?"on":"off")+".png")))png.Save(f);
     }
    }
    townRobots.IsChecked=true;townRobots.RaiseEvent(new RoutedEventArgs(System.Windows.Controls.Primitives.ButtonBase.ClickEvent));w.Width=1480;w.Height=940;
    File.WriteAllText(Path.Combine(directory,"robot-toggle-ui.json"),Core.Json.Serialize(new{passed=true,sizes=new[]{"1480x940","1000x680"},states=new[]{"on","off"},toggle_and_start_visible=true,elite_squad_exclusion_shown=true,game_started=false,preview_writes_preferences=false}));
    foreach(string name in new[]{"LibraryPage","GMPage","BackupPage","VersionPage"}){Page(name);if(name=="VersionPage")await VersionCheck();w.UpdateLayout();await Task.Delay(60);
     var bitmap=new RenderTargetBitmap((int)w.ActualWidth,(int)w.ActualHeight,96,96,PixelFormats.Pbgra32);bitmap.Render(w);var enc=new PngBitmapEncoder();enc.Frames.Add(BitmapFrame.Create(bitmap));using(var f=File.Create(Path.Combine(directory,name+".png")))enc.Save(f);
    }
    foreach(var size in new[]{new System.Drawing.Size(1120,680),new System.Drawing.Size(1248,820)}){gm.Size=size;gm.PerformLayout();using(var b=new System.Drawing.Bitmap(gm.Width,gm.Height)){gm.DrawToBitmap(b,new System.Drawing.Rectangle(0,0,gm.Width,gm.Height));b.Save(Path.Combine(directory,"GM-content-"+size.Width+".png"));}}
    foreach(double scale in new[]{1.0,1.25,1.5,2.0}){
     w.Width=1480;w.Height=940;gmHost.LayoutTransform=new ScaleTransform(scale,scale);Page("GMPage");w.UpdateLayout();await Task.Delay(120);
     gm.PreviewFontScale(scale);gm.VerifyPropertiesLayout();
     for(int page=0;page<10;page++){gm.PreviewPage(page);await Task.Delay(20);using(var b=new System.Drawing.Bitmap(gm.Width,gm.Height)){gm.DrawToBitmap(b,new System.Drawing.Rectangle(0,0,gm.Width,gm.Height));b.Save(Path.Combine(directory,"scaled-"+scale.ToString("0.00",System.Globalization.CultureInfo.InvariantCulture)+"-"+page+".png"));}}
    }
    gm.PreviewFontScale(1);gmHost.LayoutTransform=Transform.Identity;
    Page("GMPage");w.UpdateLayout();await Task.Delay(80);
    var mainHandle=new System.Windows.Interop.WindowInteropHelper(w).Handle;var childHandle=gm.Handle;
    bool sameRoot=GetAncestor(childHandle,2)==mainHandle,childParent=GetParent(childHandle)==gmViewport.Handle;
    bool focusTraversal=gm.SelectNextControl(null,true,true,true,true);
    w.Width=1000;w.Height=680;w.UpdateLayout();await Task.Delay(100);gmViewport.PerformLayout();
    gmViewport.AutoScrollPosition=new System.Drawing.Point(10000,10000);var scroll=gmViewport.AutoScrollPosition;
    bool rightReach=gm.Right<=gmViewport.ClientSize.Width+2,bottomReach=gm.Bottom<=gmViewport.ClientSize.Height+2;
    bool hiddenStayedAlive;Page("LibraryPage");await Task.Delay(50);hiddenStayedAlive=gm!=null&&!gm.IsDisposed;Page("GMPage");

    File.WriteAllText(Path.Combine(directory,"render-result.json"),Core.Json.Serialize(new{single_window=sameRoot,embedded=childParent&&!gm.TopLevel,top_level=gm.TopLevel,wpf_window=mainHandle.ToInt64(),winforms_child=childHandle.ToInt64(),winforms_parent=gmViewport.Handle.ToInt64(),host_handle=gmHost.Handle.ToInt64(),host_width=gmHost.ActualWidth,host_height=gmHost.ActualHeight,focus_traversal=focusTraversal,viewport_autoscroll=gmViewport.AutoScroll,scroll_x=scroll.X,scroll_y=scroll.Y,right_edge_reachable=rightReach,bottom_edge_reachable=bottomReach,hidden_stayed_alive=hiddenStayedAlive,forms=Forms.Application.OpenForms.Count,api_requests=0,player_data_accessed=false}));
    if(!sameRoot||!childParent||gm.TopLevel||!focusTraversal||!rightReach||!bottomReach||!hiddenStayedAlive||scroll.X>=0||scroll.Y>=0)throw new Exception("Embedded HWND/focus verification failed");
   }catch(Exception ex){File.WriteAllText(Path.Combine(directory,"render-error.txt"),ex.ToString());}await DisconnectGM();closing=true;w.Close();};
  }
 }
}

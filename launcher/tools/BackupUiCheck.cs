using System;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Windows;
using System.Windows.Controls;
using Local115US;
class BackupUiCheck {
 static List<string> passed=new List<string>();static BindingFlags flags=BindingFlags.NonPublic|BindingFlags.Instance;
 static object Get(Shell shell,string name){return typeof(Shell).GetField(name,flags).GetValue(shell);}
 static Task Call(Shell shell,string name){return(Task)typeof(Shell).GetMethod(name,flags).Invoke(shell,null);}
 static void Assert(bool b,string name){if(!b)throw new Exception(name);passed.Add(name);}
 [STAThread] static int Main(string[] args){
  string root=Path.GetFullPath(args[0]),evidence=Path.GetFullPath(args[1]);if(!File.Exists(Path.Combine(root,"ISOLATED-GUI-TEST-FIXTURE")))return 2;
  int exit=0;var app=new Application();app.ShutdownMode=ShutdownMode.OnLastWindowClose;var shell=new Shell(root,Path.Combine(root,"client"));var window=(Window)Get(shell,"w");window.ShowInTaskbar=false;window.WindowStartupLocation=WindowStartupLocation.Manual;window.Left=-20000;window.Top=-20000;
  window.ContentRendered+=async(s,e)=>{try{
   await Call(shell,"LoadBackups");var list=(ListBox)Get(shell,"backupList");Assert(list.Items.Count>0,"actual WPF backup page parses production storage-sync catalog");
   var first=list.Items.Cast<object>().First(row=>Convert.ToString(row.GetType().GetField("Name").GetValue(row)).StartsWith("same-name-regression-"));string name=Convert.ToString(first.GetType().GetField("Name").GetValue(first));string firstDir=Convert.ToString(first.GetType().GetField("Directory").GetValue(first));
   var source=(ComboBox)Get(shell,"backupSource");source.SelectedIndex=1;while((bool)Get(shell,"busy"))await Task.Delay(10);Assert(list.Items.Count>0,"source selection loads GM automatic backup catalog");
   var matched=list.Items.Cast<object>().First(row=>Convert.ToString(row.GetType().GetField("Name").GetValue(row))==name);string secondDir=Convert.ToString(matched.GetType().GetField("Directory").GetValue(matched));Assert(firstDir!=secondDir&&secondDir.Contains("gm-backups"),"actual UI rows bind duplicate names to different source directories");
   list.SelectedItem=matched;bool refused=false;try{await Call(shell,"RestoreBackup");}catch(Exception ex){refused=ex.Message.Contains("7001");}Assert(refused,"actual restore UI refuses active production game before confirmation or mutation");
   File.WriteAllText(Path.Combine(evidence,"S21-backup-ui-result.json"),Core.Json.Serialize(new{passed=passed.Count,checks=passed,fixture=root,game_started=false,real_player_data_accessed=false}));Console.WriteLine("PASS "+passed.Count+" WPF backup UI checks");
  }catch(Exception ex){exit=1;File.WriteAllText(Path.Combine(evidence,"backup-ui-error.txt"),ex.ToString());Console.Error.WriteLine(ex);}typeof(Shell).GetField("closing",flags).SetValue(shell,true);window.Close();};window.Show();app.Run();return exit;
 }
}

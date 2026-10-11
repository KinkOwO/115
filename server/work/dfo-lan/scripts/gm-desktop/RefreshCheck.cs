using System;
using System.Collections.Generic;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Threading;
using System.Threading.Tasks;
using System.Windows.Forms;

// Finite regression harness. No shown window, real credential, database or API.
static class RefreshCheck {
 static int passed;static readonly BindingFlags Flags=BindingFlags.Instance|BindingFlags.NonPublic;
 static object Get(GMForm f,string name){return typeof(GMForm).GetField(name,Flags).GetValue(f);}
 static void Set(GMForm f,string name,object value){typeof(GMForm).GetField(name,Flags).SetValue(f,value);}
 static object Call(GMForm f,string method,params object[] args){return typeof(GMForm).GetMethod(method,Flags).Invoke(f,args);}
 static void Assert(bool ok,string name){if(!ok)throw new Exception(name);passed++;}
 static void Pump(Func<bool> done){var deadline=DateTime.UtcNow.AddSeconds(3);while(!done()&&DateTime.UtcNow<deadline){Application.DoEvents();Thread.Sleep(2);}if(!done())throw new Exception("UI task timeout");}
 [STAThread] public static int Main(string[] args){try{Application.EnableVisualStyles();using(var f=new GMForm(false)){
  var buttons=(List<Button>)Get(f,"actions");int transitions=0;foreach(var b in buttons)b.EnabledChanged+=(s,e)=>transitions++;
  for(int i=0;i<4;i++){var gate=new TaskCompletionSource<int>();var poll=(Task)Call(f,"RunBackground",(Func<Task>)(()=>gate.Task));Assert(buttons.All(b=>b.Enabled),"poll disabled buttons");gate.SetResult(0);Pump(()=>poll.IsCompleted);poll.GetAwaiter().GetResult();}
  Assert(transitions==0,"refresh changed button state");
  var bgGate=new TaskCompletionSource<int>();var bg=(Task)Call(f,"RunBackground",(Func<Task>)(()=>bgGate.Task));var fgGate=new TaskCompletionSource<int>();int writes=0;
  Func<Task> action=()=>{writes++;return fgGate.Task;};var foreground=(Task)Call(f,"Run",action);
  Assert(buttons.All(b=>!b.Enabled)&&writes==0,"foreground must wait for refresh with controls locked");
  ((Task)Call(f,"Run",action)).GetAwaiter().GetResult();Assert(writes==0,"duplicate foreground action was queued");bgGate.SetResult(0);Pump(()=>writes==1);
  int extraPolls=0;((Task)Call(f,"RunBackground",(Func<Task>)(()=>{extraPolls++;return Task.FromResult(0);}))).GetAwaiter().GetResult();Assert(extraPolls==0,"poll ran during a write");
  Assert(buttons.All(b=>!b.Enabled),"write unlocked controls early");fgGate.SetResult(0);Pump(()=>foreground.IsCompleted&&bg.IsCompleted);foreground.GetAwaiter().GetResult();Assert(writes==1&&buttons.All(b=>b.Enabled),"foreground protection not restored");
  Set(f,"playerDirectory",Program.Parse("{\"accounts\":[{\"id\":7,\"username\":\"alice\",\"characters\":[{\"id\":9,\"name\":\"Hero\",\"level\":115,\"profession\":2},{\"id\":10,\"name\":\"Other\",\"level\":20,\"profession\":1}]},{\"id\":8,\"username\":\"empty\",\"characters\":[]}]}") );
  Call(f,"RenderPlayers");var grid=(DataGridView)Get(f,"playerGrid");Assert(grid.Rows.Count==3,"all accounts/characters including empty account not rendered");
  ((TextBox)Get(f,"playerSearch")).Text="hero";Assert(grid.Rows.Count==1&&Convert.ToString(grid.Rows[0].Cells[2].Value)=="Hero","character search failed");
  Set(f,"loading",true);var accounts=(ComboBox)Get(f,"accounts");accounts.Items.Add(new Choice{Id=7,Text="alice"});accounts.SelectedIndex=0;Set(f,"loading",false);
  var load=(Task)Call(f,"LoadCharacters");Pump(()=>load.IsCompleted);load.GetAwaiter().GetResult();var characters=(ComboBox)Get(f,"characters");Assert(characters.Items.Count==3,"read-only account selection did not list saved characters");
  Set(f,"loading",true);characters.SelectedIndex=1;Set(f,"loading",false);var character=(Task)Call(f,"LoadCharacter");Pump(()=>character.IsCompleted);character.GetAwaiter().GetResult();Assert((long)Get(f,"role")==9&&((Label)Get(f,"balances")).Text.Contains("最近保存"),"read-only character selection failed");
 }
 File.WriteAllText(args[0],Program.Json.Serialize(new{passed,network_requests=0,player_data_accessed=false,shown_windows=0}));Console.WriteLine("Refresh/player directory checks passed: "+passed);return 0;
 }catch(Exception e){Console.Error.WriteLine(e);return 1;}}
}

using System;
using System.IO;
using System.Linq;
using System.Diagnostics;
using System.Reflection;
using System.Threading.Tasks;
using System.Collections.Generic;
using System.Windows.Forms;
using Local115US;

class GMBackendCheck {
 static readonly List<string> passed=new List<string>();
 static readonly BindingFlags flags=BindingFlags.NonPublic|BindingFlags.Instance;
 static Type gmType;static Form gm;
 static void Assert(bool b,string name){if(!b)throw new Exception(name);passed.Add(name);}
 static Task<Dictionary<string,object>> Api(string path,object body=null){return(Task<Dictionary<string,object>>)gmType.GetMethod("Api",flags).Invoke(gm,new[]{(object)path,body});}
 static IEnumerable<Dictionary<string,object>> Rows(Dictionary<string,object> d,string key){return((System.Collections.IEnumerable)d[key]).Cast<object>().Select(o=>(Dictionary<string,object>)o);}
 static long Number(Dictionary<string,object> d,string key){return Convert.ToInt64(d[key]);}
 static Dictionary<string,object> Grant(long account,long role,string id){return new Dictionary<string,object>{{"account",account},{"character",role},{"grant_id",id},{"reason","isolated unified-launcher regression"},{"gold",1},{"cera",1}};}
 [STAThread] static int Main(string[] args){
  string root=Path.GetFullPath(args[0]),evidence=Path.GetFullPath(args[1]);Directory.CreateDirectory(evidence);int exit=0;
  // Explicit test-fixture marker is mandatory. This harness never accepts an ordinary installation.
  if(!File.Exists(Path.Combine(root,"ISOLATED-GUI-TEST-FIXTURE")))throw new Exception("Missing explicit isolated fixture marker");
  var context=new PackageContext(root,Core.Client(root),true);string configHash=Core.Hash(context.StorageConfig);
  Application.EnableVisualStyles();Application.SetCompatibleTextRenderingDefault(false);var host=new Form{ShowInTaskbar=false,StartPosition=FormStartPosition.Manual,Left=-20000,Top=-20000,Width=1200,Height=800};
  host.Shown+=async(s,e)=>{try{
   gmType=typeof(Core).Assembly.GetType("GMForm",true);gm=(Form)Activator.CreateInstance(gmType,new object[]{false,context});host.Controls.Add(gm);gm.Show();await(Task)gmType.GetMethod("ActivateAsync").Invoke(gm,null);
   Assert((bool)gmType.GetProperty("IsConnected").GetValue(gm,null),"embedded GUI connected to its packaged backend");
   int pid=(int)gmType.GetProperty("BackendPID").GetValue(gm,null);var status=await Api("/api/gm/status");context.AssertDatabase(Convert.ToString(status["database"]));Assert(Convert.ToString(status["edition"])=="local","backend confirms local edition and exact database identity");
   gm.Hide();await Task.Delay(50);gm.Show();Assert((int)gmType.GetProperty("BackendPID").GetValue(gm,null)==pid,"page hide and show retains same live backend");await(Task)gmType.GetMethod("ActivateAsync").Invoke(gm,null);Assert((int)gmType.GetProperty("BackendPID").GetValue(gm,null)==pid,"repeated opening reuses one backend");
   var overview=await Api("/api/overview");long account=Number(Rows(overview,"accounts").First(),"id");var chars=await Api("/api/characters?account="+account);long role=Number(Rows(chars,"characters").First(),"id");var before=await Api("/api/character?id="+role);long gold=Number((Dictionary<string,object>)before["character"],"gold"),cera=Number(before,"cera");
   string id="unified-regression-"+Guid.NewGuid().ToString("N");var grant=Grant(account,role,id);var first=await Api("/api/grant",grant);var replay=await Api("/api/grant",grant);var after=await Api("/api/character?id="+role);
   Assert(Convert.ToBoolean(first["applied"])&&!Convert.ToBoolean(replay["applied"])&&Number((Dictionary<string,object>)after["character"],"gold")==gold+1&&Number(after,"cera")==cera+1,"lost-receipt replay uses grant_id and changes balances only once");
   bool refused=false;try{await Api("/api/grant",Grant(account+999999,role,"foreign-"+id));}catch{refused=true;}Assert(refused,"foreign account and character pairing rejected");
   var invalid=Grant(account,role,"invalid-"+id);invalid["items"]=new[]{new{template=4294967295L,amount=1}};refused=false;try{await Api("/api/grant",invalid);}catch{refused=true;}after=await Api("/api/character?id="+role);Assert(refused&&Number((Dictionary<string,object>)after["character"],"gold")==gold+1&&Number(after,"cera")==cera+1,"invalid grant leaves existing balance unchanged");
   var backup=await Api("/api/gm/backup",new{});var backups=await Api("/api/gm/backups");Assert(Rows(backups,"backups").Any(row=>File.Exists(Path.Combine(Convert.ToString(row["directory"]),"manifest.json"))),"GM automatic backup catalog returned with real manifests");
   await(Task)gmType.GetMethod("ShutdownAsync").Invoke(gm,null);await Task.Delay(100);bool alive;try{using(var process=Process.GetProcessById(pid))alive=!process.HasExited;}catch(ArgumentException){alive=false;}Assert(!alive,"closing embedded session removes its backend process");Assert(!File.Exists(context.Database+".admin-guard"),"no administration lease remains after session close");Assert(Core.Hash(context.StorageConfig)==configHash,"custom storage configuration unchanged by GM operations");gm.Dispose();
   File.WriteAllText(Path.Combine(evidence,"S20-gm-operation-result.json"),Core.Json.Serialize(new{passed=passed.Count,checks=passed,root,database=context.Database,account,role,game_started=false,real_player_data_accessed=false,online_game_flow="covered by Go unit suite; no real game started"}));Console.WriteLine("PASS "+passed.Count+" embedded GM backend checks");
  }catch(Exception ex){exit=1;File.WriteAllText(Path.Combine(evidence,"gm-backend-error.txt"),ex.ToString());Console.Error.WriteLine(ex);}if(gm!=null&&!gm.IsDisposed){try{await(Task)gmType.GetMethod("ShutdownAsync").Invoke(gm,null);}catch{}gm.Dispose();}host.Close();};Application.Run(host);return exit;
 }
}

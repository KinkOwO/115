using System;
using System.IO;
using System.Linq;
using System.Diagnostics;
using System.Collections.Generic;
using Local115US;

class FeatureCheck {
 static List<string> passed=new List<string>();
 static void Assert(bool b,string name){if(!b)throw new Exception(name);passed.Add(name);}
 static ProcessStartInfo StartInfo(){var p=new ProcessStartInfo();p.EnvironmentVariables["DFO_UNAPPROVED"]="danger";p.EnvironmentVariables["DFO_TOWN_ROBOTS"]="old";p.EnvironmentVariables["FEATURE_TEST_RETAIN"]="yes";RobotLaunch.ClearInherited(p);return p;}
 static int Main(string[] args){try{
  string root=args[0],client=Path.Combine(root,"client"),evidence=args[1],cfg=Path.Combine(Core.Module(root),"runtime/storage/local.json");
  Assert(!File.Exists(cfg),"fresh package has no active storage configuration");Core.Prepare(root,client);Assert(!File.Exists(cfg),"validation does not initialize storage");
  Core.Prepare(root,client,true);var context=new PackageContext(root,client,false);Assert(File.Exists(cfg)&&!File.Exists(context.Database),"first-run prepare creates only configuration");
  string custom=Path.Combine(root,"custom saves","private.sqlite3");Directory.CreateDirectory(Path.GetDirectoryName(custom));File.WriteAllText(custom,"identity-only fixture");Core.Write(cfg,new Dictionary<string,object>{{"driver","sqlite"},{"sqlite_path",custom},{"custom_field","retained"}});string hash=Core.Hash(cfg);Core.Prepare(root,client,true);Assert(Core.Hash(cfg)==hash,"full prepare preserves custom save configuration bytes");
  string preference=Path.Combine(root,RobotLaunch.PreferenceFile);
  Assert(RobotLaunch.Enabled(root)&&!File.Exists(preference),"default follows r1 enabled state without writing preferences");
  Core.Write(preference,new Dictionary<string,object>{{"unrelated","retained"}});RobotLaunch.SetEnabled(root,false);
  Assert(!RobotLaunch.Enabled(root)&&Core.Get(Core.Read(preference),"unrelated","")=="retained","off choice persists and preserves unrelated settings");
  string elite=Path.Combine(client,".115us-mods/EliteCompanionPilot.dll");File.WriteAllText(elite,"inert elite squad fixture");string eliteHash=Core.Hash(elite),saveHash=Core.Hash(cfg);
  var off=StartInfo();off.EnvironmentVariables["DFO_ELITE_COMPANION_PILOT"]="attunement";off.EnvironmentVariables["DFO_LOCAL_ROBOT_AI"]="stale";off.EnvironmentVariables["DFO_ROBOT_AI_OFFER"]="stale";
  RobotLaunch.Apply(off,root,client);
  Assert(off.EnvironmentVariables["DFO_TOWN_ROBOTS"]=="0"&&off.EnvironmentVariables["DFO_LOCAL_ROBOT_AI"]==null&&off.EnvironmentVariables["DFO_ROBOT_AI_OFFER"]==null&&off.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"]==null&&off.EnvironmentVariables["DFO_TOWN_ROBOT_STORAGE"]==null,"off disables fleet and clears stale ordinary robot capabilities");
  Assert(off.EnvironmentVariables["DFO_ELITE_COMPANION_PILOT"]=="attunement"&&Core.Hash(elite)==eliteHash,"off preserves elite squad environment and plugin bytes");
  Assert(off.EnvironmentVariables["DFO_LOCAL_DUNGEON_EXIT"]=="r13-local-exit-v1","off keeps shared local dungeon exit support");
  Assert(!Directory.Exists(Path.Combine(Core.Module(root),"runtime/robot-authority"))&&!File.Exists(Path.Combine(Core.Module(root),"runtime/storage/town-robots-integrated.json"))&&Core.Hash(cfg)==saveHash,"off does not create robot runtime data or edit player storage");
  RobotLaunch.SetEnabled(root,true);Assert(RobotLaunch.Enabled(root),"on choice persists for the next launch");
  var p=StartInfo();Assert(!p.EnvironmentVariables.Keys.Cast<string>().Any(k=>k.StartsWith("DFO_",StringComparison.OrdinalIgnoreCase)),"clears all inherited DFO settings");Assert(p.EnvironmentVariables["FEATURE_TEST_RETAIN"]=="yes","preserves unrelated environment");
  RobotLaunch.Apply(p,root,client);var expected=new[]{"DFO_TOWN_ROBOT_UDP_OBSERVE","DFO_LOCAL_DUNGEON_EXIT","DFO_LOCAL_ROBOT_AUTHORITY","DFO_LOCAL_ROBOT_AI","DFO_ROBOT_AUTHORITY_LEASE","DFO_ROBOT_AI_OFFER","DFO_ROBOT_AI_ACK","DFO_TOWN_ROBOTS","DFO_TOWN_ROBOT_STORAGE"};
  Assert(p.EnvironmentVariables.Keys.Cast<string>().Where(k=>k.StartsWith("DFO_",StringComparison.OrdinalIgnoreCase)).OrderBy(x=>x).SequenceEqual(expected.OrderBy(x=>x)),"only the nine fixed local robot environment keys injected");
  Assert(p.EnvironmentVariables["DFO_TOWN_ROBOTS"]=="69"&&p.EnvironmentVariables["DFO_LOCAL_ROBOT_AI"]=="r23-native-input-v1","local count and AI protocol values bound");
  string storage=p.EnvironmentVariables["DFO_TOWN_ROBOT_STORAGE"],session=Path.GetDirectoryName(p.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"]);Assert(File.Exists(storage)&&Path.GetDirectoryName(p.EnvironmentVariables["DFO_ROBOT_AI_ACK"])==session,"template initialized and IPC uses one session directory");
  File.WriteAllText(storage,"retained player-owned robot archive");var next=StartInfo();RobotLaunch.Apply(next,root,client);Assert(File.ReadAllText(storage)=="retained player-owned robot archive","existing robot progression archive never replaced");Assert(session!=Path.GetDirectoryName(next.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"]),"each launch has a new authority directory");Assert(!File.Exists(next.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"]),"launcher does not fabricate authority lease");
  int sessions=Directory.GetDirectories(Path.GetDirectoryName(session)).Length;RobotLaunch.SetEnabled(root,false);RobotLaunch.Apply(next,root,client);
  Assert(File.ReadAllText(storage)=="retained player-owned robot archive"&&Directory.GetDirectories(Path.GetDirectoryName(session)).Length==sessions,"turning off preserves progression and creates no new IPC session");
  RobotLaunch.SetEnabled(root,true);var enabledAgain=StartInfo();enabledAgain.EnvironmentVariables["DFO_ELITE_COMPANION_PILOT"]="attunement";RobotLaunch.Apply(enabledAgain,root,client);
  Assert(enabledAgain.EnvironmentVariables["DFO_TOWN_ROBOTS"]=="69"&&File.ReadAllText(storage)=="retained player-owned robot archive"&&enabledAgain.EnvironmentVariables["DFO_ELITE_COMPANION_PILOT"]=="attunement"&&Core.Hash(elite)==eliteHash,"re-enabling resumes ordinary robots and leaves elite squad unchanged");
  string alternate=Path.Combine(root,"separate installation");Directory.CreateDirectory(alternate);Assert(!RobotLaunch.Enabled(alternate),"preference is scoped to the selected installation");
  var savedPreference=File.ReadAllBytes(preference);File.WriteAllText(preference,"malformed preferences");bool badRefused=false;try{RobotLaunch.SetEnabled(root,false);}catch{badRefused=true;}
  Assert(badRefused&&File.ReadAllText(preference)=="malformed preferences","malformed preference is never silently replaced");File.WriteAllBytes(preference,savedPreference);
  string plugin=Path.Combine(client,".115us-mods/test.dll");var bytes=File.ReadAllBytes(plugin);try{File.WriteAllText(plugin,"corrupt");var rejected=StartInfo();bool refused=false;try{RobotLaunch.Apply(rejected,root,client);}catch{refused=true;}Assert(!refused&&rejected.EnvironmentVariables["DFO_LOCAL_ROBOT_AI"]=="r23-native-input-v1","updated plugin fingerprint does not block robot activation");}finally{File.WriteAllBytes(plugin,bytes);}
  File.WriteAllText(Path.Combine(evidence,"S19-first-run-robot-result.json"),Core.Json.Serialize(new{passed=passed.Count,checks=passed,fixture=root,game_started=false,real_player_data_accessed=false}));Console.WriteLine("PASS "+passed.Count+" first-run and robot contract checks");return 0;
 }catch(Exception ex){Console.Error.WriteLine(ex);return 1;}}
}

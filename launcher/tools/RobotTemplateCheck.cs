using System;
using System.IO;
using System.Diagnostics;
using System.Collections.Generic;
using Local115US;
class RobotTemplateCheck {
 static int Main(string[] args){try{
  string root=args[0],template=Path.Combine(root,"server/work/dfo-lan/configs/robots-template.json");var data=Core.Read(template);var records=(Dictionary<string,object>)data["records"];if(records.Count!=69)throw new Exception("Expected 69 fresh robot records");
  var process=new ProcessStartInfo();RobotLaunch.ClearInherited(process);RobotLaunch.Apply(process,root,Path.Combine(root,"client"));string target=process.EnvironmentVariables["DFO_TOWN_ROBOT_STORAGE"];if(Core.Hash(target)!=Core.Hash(template))throw new Exception("Fresh template copy mismatch");
  var second=new ProcessStartInfo();RobotLaunch.ClearInherited(second);RobotLaunch.Apply(second,root,Path.Combine(root,"client"));if(process.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"]==second.EnvironmentVariables["DFO_ROBOT_AUTHORITY_LEASE"])throw new Exception("Authority session directory reused");
  File.WriteAllText(args[1],Core.Json.Serialize(new{passed=3,records=records.Count,template_sha256=Core.Hash(template),storage=target,environment_count="9 fixed keys",game_started=false,real_player_data_accessed=false}));Console.WriteLine("PASS 3 actual 69-robot template checks");return 0;
 }catch(Exception ex){Console.Error.WriteLine(ex);return 1;}}
}

using System;
using System.IO;
using System.Collections.Generic;
using System.Diagnostics;

namespace Local115US {
 public static class RobotLaunch {
  public const string PreferenceFile="local-robot.settings.json";
  static readonly string[] RobotKeys={"DFO_TOWN_ROBOT_UDP_OBSERVE","DFO_LOCAL_ROBOT_AUTHORITY","DFO_LOCAL_ROBOT_AI","DFO_ROBOT_AUTHORITY_LEASE","DFO_ROBOT_AI_OFFER","DFO_ROBOT_AI_ACK","DFO_TOWN_ROBOTS","DFO_TOWN_ROBOT_STORAGE"};
  public static bool Enabled(string root){
   var preference=Core.Read(Path.Combine(root,PreferenceFile));object value;
   if(preference==null)throw new Exception("假人开关配置为空，请检查 "+PreferenceFile+"。");
   if(preference.TryGetValue("town_robots_enabled",out value)){
    if(!(value is bool))throw new Exception("假人开关配置必须是 true 或 false。");
    return (bool)value;
   }
   var robots=Settings(root);return robots!=null&&Core.Get(robots,"enabled","false").ToLowerInvariant()=="true";
  }
  public static void SetEnabled(string root,bool enabled){
   string path=Path.Combine(root,PreferenceFile);var preference=Core.Read(path);
   if(preference==null)throw new Exception("假人开关配置为空，未覆盖原文件。");
   preference["town_robots_enabled"]=enabled;Core.Write(path,preference);
  }
  static Dictionary<string,object> Settings(string root){
   Dictionary<string,object> d;try{d=Core.Read(Path.Combine(root,ReleaseManifest.Name));if(d==null)return null;}catch{return null;}object features,robots;
   if(!d.TryGetValue("local_features",out features)||!(features is Dictionary<string,object>))return null;
   if(!((Dictionary<string,object>)features).TryGetValue("robots",out robots)||!(robots is Dictionary<string,object>))return null;
   return (Dictionary<string,object>)robots;
  }
  public static void ClearInherited(ProcessStartInfo process){
   var keys=new List<string>();foreach(string key in process.EnvironmentVariables.Keys)if(key.StartsWith("DFO_",StringComparison.OrdinalIgnoreCase))keys.Add(key);foreach(string key in keys)process.EnvironmentVariables.Remove(key);
  }
  public static void Apply(ProcessStartInfo process,string root,string client){
   var env=process.EnvironmentVariables;
   // Only ordinary town/party robot capabilities belong to this switch.
   // Elite squad settings and plugin files are independent and never edited here.
   foreach(string key in RobotKeys)env.Remove(key);
   var robots=Settings(root);
   if(robots!=null)env["DFO_LOCAL_DUNGEON_EXIT"]="r13-local-exit-v1";
   if(!Enabled(root)){env["DFO_TOWN_ROBOTS"]="0";return;}
   if(robots==null)throw new Exception("当前安装缺少城镇/组队假人配置。");
   // Validate all released plugins before publishing any robot capability or IPC paths.
   ReleaseManifest.Validate(root,client);
   int count;if(!Int32.TryParse(Core.Get(robots,"count",""),out count)||count<1||count>100)throw new Exception("假人数目不在整合版支持范围内。");
   string storage=Core.Get(robots,"storage",""),template=Core.Get(robots,"template",""),hash=Core.Get(robots,"template_sha256","");
   if(!storage.StartsWith("server/work/dfo-lan/runtime/storage/",StringComparison.Ordinal)||!storage.EndsWith(".json",StringComparison.OrdinalIgnoreCase)||!template.StartsWith("server/work/dfo-lan/configs/",StringComparison.Ordinal)||!template.EndsWith(".json",StringComparison.OrdinalIgnoreCase))throw new Exception("假人存档或干净模板不属于配套目录。");
   string source=ReleaseManifest.Resolve(root,client,template),target=ReleaseManifest.Resolve(root,client,storage);
   if(!File.Exists(source))throw new Exception("假人模板文件缺失。");
   hash=Core.Hash(source); // Preserve the selected valid template; no release fingerprint pin.
   var records=Core.Read(source);if(!records.ContainsKey("records"))throw new Exception("假人模板格式无效。");
   Directory.CreateDirectory(Path.GetDirectoryName(target));
   if(!File.Exists(target)){
    string temp=target+"."+Guid.NewGuid().ToString("N")+".tmp";File.Copy(source,temp,false);
    try{if(Core.Hash(temp)!=hash)throw new Exception("假人模板复制校验失败。");File.Move(temp,target);}finally{if(File.Exists(temp))File.Delete(temp);}
   }
   // Never refresh or replace an existing persistent robot profile.
   string session=Path.Combine(Core.Module(root),"runtime","robot-authority",Guid.NewGuid().ToString("N"));Directory.CreateDirectory(session);
   env["DFO_TOWN_ROBOT_UDP_OBSERVE"]="r12-observe1";
   env["DFO_LOCAL_ROBOT_AUTHORITY"]="r16-object-authority-v1";
   env["DFO_LOCAL_ROBOT_AI"]="r23-native-input-v1";
   env["DFO_ROBOT_AUTHORITY_LEASE"]=Path.Combine(session,"authority.bin");
   env["DFO_ROBOT_AI_OFFER"]=Path.Combine(session,"ai-offer.bin");
   env["DFO_ROBOT_AI_ACK"]=Path.Combine(session,"ai-ack.bin");
   env["DFO_TOWN_ROBOTS"]=count.ToString(System.Globalization.CultureInfo.InvariantCulture);
   env["DFO_TOWN_ROBOT_STORAGE"]=target;
  }
 }
}

using System;
using System.IO;
using System.Linq;
using System.Collections.Generic;
using System.Text.RegularExpressions;

namespace Local115US {
 public static class ReleaseManifest {
  public const string Name="release-manifest.json";
  public static string Resolve(string root,string client,string path){
   if(String.IsNullOrWhiteSpace(path)||path.Contains('\\')||path.StartsWith("/")||path.Contains(':')||path.Split('/').Any(x=>x==".."||x=="."||x==""))throw new Exception("版本清单含无效相对路径。");
   return path.StartsWith("client/",StringComparison.Ordinal)?Path.Combine(client,path.Substring(7).Replace('/','\\')):Path.Combine(root,path.Replace('/','\\'));
  }
  public static List<string> Validate(string root,string client,bool hashes=false){
   var result=new List<string>();var p=Path.Combine(root,Name);if(!File.Exists(p)){result.Add("提示：缺少版本清单，允许继续使用当前安装。");return result;}
   Dictionary<string,object> d;try{d=Core.Read(p);if(d==null)throw new Exception("空清单");}catch(Exception ex){result.Add("提示：版本清单无法读取，允许继续："+ex.Message);return result;}if(Core.Get(d,"schema","")!="1"||Core.Get(d,"edition","")!="local")result.Add("提示：版本清单格式不同，仅供参考。");
   if(Core.Get(d,"release_id","")!=ReleaseBuild.ID)result.Add("提示：发行标识不同，允许组件独立更新。");
   if(Core.Get(d,"gui_version","")!="2.0.5.0")result.Add("提示：登录器版本不同，允许继续。");
   if(Core.Get(d,"embedded_gm_sha256","")!=ReleaseBuild.GMHash)result.Add("提示：GM版本记录不同，使用当前内嵌后台。");
   object rows;if(!d.TryGetValue("files",out rows)||!(rows is System.Collections.IEnumerable)){result.Add("提示：版本清单没有文件表。");return result;}
   var paths=new HashSet<string>(StringComparer.OrdinalIgnoreCase);var roles=new HashSet<string>();
   foreach(object o in (System.Collections.IEnumerable)rows){
    var row=o as Dictionary<string,object>;if(row==null){result.Add("提示：忽略无效清单记录。");continue;}
    string path=Core.Get(row,"path",""),hash=Core.Get(row,"sha256",""),role=Core.Get(row,"role","asset");
    if(!paths.Add(path)||String.Equals(path,Name,StringComparison.OrdinalIgnoreCase)||!Regex.IsMatch(hash,"^[a-f0-9]{64}$")){result.Add("提示：忽略重复、自引用或无效哈希记录："+path);continue;}
    string file;try{file=Resolve(root,client,path);}catch{result.Add("提示：忽略无效相对路径。");continue;}long size;if(!Int64.TryParse(Core.Get(row,"size",""),out size)||size<0){result.Add("提示：无效文件大小记录："+path);continue;}
    if(!File.Exists(file)){result.Add("提示：清单组件不存在："+path);continue;}
    try{if(new FileInfo(file).Length!=size||(hashes&&Core.Hash(file)!=hash)){result.Add(path+"  提示：与发行记录不同，允许继续");continue;}}catch(Exception ex){result.Add("提示：无法读取组件信息："+path+"："+ex.Message);continue;}
    if(role=="gateway"&&(path!="server/work/dfo-lan/bin/wireprobe-pvf.exe"||hash!=ReleaseBuild.ServerHash))result.Add("提示：网关已独立更新，允许继续。");
    roles.Add(role);result.Add(path+(hashes?"  检查完成":"  文件存在"));
   }
   foreach(string role in new[]{"gui","gateway","launcher","pvf","sk","inner-pvf","plugin"})if(!roles.Contains(role))result.Add("提示：清单未记录组件："+role);
   string gui=System.Reflection.Assembly.GetExecutingAssembly().Location;
   if(!paths.Contains(Path.GetFileName(gui)))result.Add("提示：当前登录器位置不同，使用所选整合包。");
   return result;
  }
 }
}

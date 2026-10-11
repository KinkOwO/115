using System;
using System.IO;
using System.Collections.Generic;
using Local115US;

class PortabilityCheck {
 static int Main(string[] args){try{
  Console.OutputEncoding=new System.Text.UTF8Encoding(false);
  string root=Path.GetFullPath(args[0]),client=args.Length>2?Path.GetFullPath(args[2]):Core.Client(root);
  if(!File.Exists(Path.Combine(root,"ISOLATED-PORTABILITY-FIXTURE")))throw new Exception("Isolated fixture marker required");
  if(args[1]=="prepare")Core.Prepare(root,client,true);
  if(args[1]=="check")Core.Prepare(root,client);
  var context=new PackageContext(root,client,false);string foreign=args.Length>3?args[3]:null;bool foreignRefused=false;
  if(File.Exists(context.Database)){
   context.AssertDatabase(context.Database);
   if(foreign!=null){try{context.AssertDatabase(foreign);}catch{foreignRefused=true;}}
  }
  Console.WriteLine(Core.Json.Serialize(new{root,client,database=context.Database,storage_config=context.StorageConfig,
   database_exists=File.Exists(context.Database),sqlite_path=Core.Get(Core.Read(context.StorageConfig),"sqlite_path",""),
   client_dir=Core.Get(Core.Read(Path.Combine(root,"server/launcher.local.json")),"client_dir",""),foreign_database_refused=foreignRefused}));return 0;
 }catch(Exception ex){Console.Error.WriteLine(ex);return 1;}}
}

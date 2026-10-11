param([Parameter(Mandatory=$true)][string]$ClientDir,[Parameter(Mandatory=$true)][string]$CandidateDir,[Parameter(Mandatory=$true)][string]$EditorDll)
$ErrorActionPreference='Stop'
Set-Location -LiteralPath (Split-Path -Parent $PSScriptRoot)
[Reflection.Assembly]::LoadFrom($editorDll)|Out-Null
Add-Type -ReferencedAssemblies @($editorDll,'System.Runtime','System.Collections','System.IO','System.Console','System.Memory','System.Text.Json','System.Security.Cryptography','System.Text.Encoding','System.Text.Encoding.Extensions','System.Text.RegularExpressions') -TypeDefinition @'
using System;using System.IO;using System.Text;using System.Text.Json;using System.Text.RegularExpressions;using System.Collections.Generic;using System.Security.Cryptography;
public static class BuildLoginMaterials {
 static string Hash(string p){using var f=File.OpenRead(p);return Convert.ToHexString(SHA256.HashData(f)).ToLowerInvariant();}
 static byte[] Compile(A.m e,JsonElement ts){using var m=new MemoryStream();using var w=new BinaryWriter(m);foreach(var t in ts.EnumerateArray()){byte k=t[0].GetByte();w.Write(k);w.Write(k==3||k==6||k==8?e.B(t[1].GetString()):t[1].GetInt32());}return m.ToArray();}
 public static void Run(string client,string output){
  using var doc=JsonDocument.Parse(File.ReadAllText(Path.Combine(output,"pvf-plan.json")));var plan=doc.RootElement;
  string script=Path.Combine(client,"Script.pvf"),sk=Path.Combine(client,"sk.dat");
  string scriptHash=plan.GetProperty("client_script_sha256").GetString(),skHash=plan.GetProperty("client_sk_sha256").GetString();
  if(Hash(script)!=scriptHash||Hash(sk)!=skHash)throw new Exception("Client source identity changed");
  if(File.Exists(Path.Combine(output,"Script.pvf"))||File.Exists(Path.Combine(output,"sk.dat")))throw new Exception("Candidate already built");
  var e=new A.m(A.M.A(script,Path.Combine(output,"DFO.exe"),sk));
  var changes=new List<string>();
  foreach(var f in plan.GetProperty("files").EnumerateArray()){
   string p=f.GetProperty("path").GetString();int kind=f.GetProperty("kind").GetInt32();
   byte[] raw=kind==1?Compile(e,f.GetProperty("tokens")):Convert.FromBase64String(f.GetProperty("raw").GetString());
   e.A(p,kind,raw);changes.Add(p);
  }
  int n=e.c("list/n_string.lst");if(n<0)throw new Exception("Native string index missing");
  byte[] idx=e.Image.A(e.Image.A(n));var paths=new Dictionary<int,string>();
  for(int at=0;at<idx.Length;at+=10){if(at+10>idx.Length||idx[at]!=0||idx[at+5]!=6)throw new Exception("Native string index grammar");paths.Add(BitConverter.ToInt32(idx,at+1),e.A(BitConverter.ToInt32(idx,at+6)));}
  var tables=new Dictionary<int,string>();
  foreach(var row in plan.GetProperty("localized").EnumerateArray()){
   int id=row.GetProperty("table").GetInt32();string key=row.GetProperty("key").GetString(),value=row.GetProperty("value").GetString();
   if(key.Length==0||key.Length>40||key.Contains(">")||key.Contains("\r")||key.Contains("\n"))throw new Exception("Native string tag grammar: "+key);
   if(!tables.TryGetValue(id,out string text)){if(!paths.TryGetValue(id,out string p))throw new Exception("String table ID absent "+id);int ix=e.c(p);if(ix<0)throw new Exception("String table absent "+p);text=Encoding.Unicode.GetString(e.Image.A(e.Image.A(ix)));}
   var rx=new Regex("(?m)^(?:\\uFEFF)?"+Regex.Escape(key)+">[^\\r\\n\\0]*");var matches=rx.Matches(text);
   if(matches.Count>1)throw new Exception("Duplicate target string key "+key);
   bool preserve=row.TryGetProperty("preserve_existing",out var keep)&&keep.GetBoolean();
   if(matches.Count==1){if(!preserve)text=rx.Replace(text,m=>(m.Value.StartsWith("\uFEFF",StringComparison.Ordinal)?"\uFEFF":"")+key+">"+value);}
   else {string tail=text.Substring(text.TrimEnd('\0').Length);text=text.TrimEnd('\0');if(!text.EndsWith("\n"))text+="\r\n";text+=key+">"+value+"\r\n"+tail;}
   tables[id]=text;
  }
  foreach(var row in tables){if(row.Value.IndexOf("\uFEFF",1,StringComparison.Ordinal)>=0)throw new Exception("Embedded BOM in string table "+paths[row.Key]);e.A(paths[row.Key],3,Encoding.Unicode.GetBytes(row.Value));changes.Add(paths[row.Key]);}
  e.A(Path.Combine(output,"Script.pvf"),Path.Combine(output,"sk.dat"),s=>Console.WriteLine(s));
  if(Hash(script)!=scriptHash||Hash(sk)!=skHash)throw new Exception("Original client unexpectedly changed");
  File.WriteAllText(Path.Combine(output,"build-report.json"),JsonSerializer.Serialize(new {changed=changes,script_sha256=Hash(Path.Combine(output,"Script.pvf")),sk_sha256=Hash(Path.Combine(output,"sk.dat")),original_client_unchanged=true},new JsonSerializerOptions{WriteIndented=true}));
 }
}
'@
[BuildLoginMaterials]::Run([IO.Path]::GetFullPath($ClientDir),[IO.Path]::GetFullPath($CandidateDir))

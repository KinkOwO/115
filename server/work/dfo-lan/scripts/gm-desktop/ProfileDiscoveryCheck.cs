using System;
using System.IO;
using System.Security.Cryptography;
using System.Text;

// Compile with /main:ProfileDiscoveryCheck and the production UI sources.
// Uses generated credentials in a caller-supplied test directory, no network.
static class ProfileDiscoveryCheck {
 static int passed;
 static void Assert(bool value,string name){if(!value)throw new Exception(name);passed++;}
 public static int Main(string[] args){try{string root=Path.GetFullPath(args[0]);Directory.CreateDirectory(root);string primary=Path.Combine(root,"primary.dpapi"),backup=Path.Combine(root,"backup.dpapi");foreach(var path in new[]{primary,backup})if(File.Exists(path))throw new Exception("Test directory must be empty");
  var json=Program.Json.Serialize(new{endpoint="https://owner-check.invalid:10443",token=new string('a',64),certificate_sha256=new string('b',64)});
  var valid=ProtectedData.Protect(Encoding.UTF8.GetBytes(json),null,DataProtectionScope.CurrentUser);
  File.WriteAllBytes(primary,valid);Assert(Program.Str(OwnerProfile.LoadFromPaths(primary,backup),"endpoint")=="https://owner-check.invalid:10443","existing profile selected");Assert(File.Exists(backup),"backup created");
  File.Delete(primary);OwnerProfile.LoadFromPaths(primary,backup);Assert(Program.Hash(File.ReadAllBytes(primary))==Program.Hash(valid),"missing primary restored");
  File.WriteAllText(primary,"broken");OwnerProfile.LoadFromPaths(primary,backup);Assert(Program.Hash(File.ReadAllBytes(primary))==Program.Hash(valid),"corrupt primary restored");
  File.WriteAllBytes(primary,ProtectedData.Protect(Encoding.UTF8.GetBytes("{}"),null,DataProtectionScope.CurrentUser));OwnerProfile.LoadFromPaths(primary,backup);Assert(Program.Hash(File.ReadAllBytes(primary))==Program.Hash(valid),"invalid schema skipped");
  File.WriteAllBytes(primary,ProtectedData.Protect(Encoding.UTF8.GetBytes("null"),null,DataProtectionScope.CurrentUser));OwnerProfile.LoadFromPaths(primary,backup);Assert(Program.Hash(File.ReadAllBytes(primary))==Program.Hash(valid),"null schema skipped");
  File.WriteAllText(primary,json);File.Delete(backup);bool rejected=false;try{OwnerProfile.LoadFromPaths(primary,backup);}catch(InvalidOperationException){rejected=true;}Assert(rejected,"plaintext never treated as protected credential");
  File.Delete(primary);rejected=false;try{OwnerProfile.LoadFromPaths(primary,backup);}catch(InvalidOperationException e){rejected=e.Message.Contains("尚未配置");}Assert(rejected,"missing profile explained without picker");
  File.WriteAllText(Path.Combine(root,"result.json"),Program.Json.Serialize(new{passed,network_requests=0,real_profile_accessed=false}),new UTF8Encoding(false));Console.WriteLine("Profile discovery checks passed: "+passed);return 0;
 }catch(Exception e){Console.Error.WriteLine(e.Message);return 1;}}
}

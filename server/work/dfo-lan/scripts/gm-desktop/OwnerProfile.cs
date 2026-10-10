using System;
using System.Collections.Generic;
using System.IO;
using System.Security.Cryptography;
using System.Text;

// Owner credentials stay outside the game package and remain bound to the
// current Windows account. Never scan arbitrary server JSON or other users.
static class OwnerProfile {
 public static string Primary {get{return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.LocalApplicationData),"115US","GM","owner.dpapi");}}
 public static string Backup {get{return Path.Combine(Environment.GetFolderPath(Environment.SpecialFolder.ApplicationData),"115US","GM","owner.dpapi");}}
 static Dictionary<string,object> Read(byte[] bytes){var data=Program.Parse(Encoding.UTF8.GetString(ProtectedData.Unprotect(bytes,null,DataProtectionScope.CurrentUser)));Program.ValidateProfile(data);return data;}
 static void Save(string path,byte[] bytes){Directory.CreateDirectory(Path.GetDirectoryName(path));string stage=path+"."+Guid.NewGuid().ToString("N")+".tmp";try{File.WriteAllBytes(stage,bytes);if(File.Exists(path))File.Replace(stage,path,null);else File.Move(stage,path);}finally{if(File.Exists(stage))File.Delete(stage);}}
 public static void Import(string path){var bytes=File.ReadAllBytes(path);Program.ValidateProfile(Program.Parse(Encoding.UTF8.GetString(bytes)));var encrypted=ProtectedData.Protect(bytes,null,DataProtectionScope.CurrentUser);Save(Primary,encrypted);Save(Backup,encrypted);}
 public static Dictionary<string,object> Load(){return LoadFromPaths(Primary,Backup);}
 internal static Dictionary<string,object> LoadFromPaths(string primary,string backup){bool found=false;foreach(var path in new[]{primary,backup}){if(!File.Exists(path))continue;found=true;byte[] bytes;Dictionary<string,object> data;try{bytes=File.ReadAllBytes(path);data=Read(bytes);}catch(IOException){continue;}catch(UnauthorizedAccessException){continue;}catch(CryptographicException){continue;}catch(ArgumentException){continue;}catch(InvalidOperationException){continue;}catch(FormatException){continue;}
   // Repair a missing primary or create the same-user recovery copy. A backup
   // write failure must not prevent use of an already verified credential.
   try{if(!String.Equals(path,primary,StringComparison.OrdinalIgnoreCase))Save(primary,bytes);if(!File.Exists(backup)||Program.Hash(File.ReadAllBytes(backup))!=Program.Hash(bytes))Save(backup,bytes);}catch(IOException){}catch(UnauthorizedAccessException){}
   return data;
  }
  throw new InvalidOperationException(found?"服主凭据无法读取或解密。请使用最初配置 GM 的 Windows 账号启动；无需选择服务端 JSON 文件。":"本机尚未配置服主连接凭据。请恢复本机服主凭据后点击连接 / 刷新；无需选择服务端 JSON 文件。");
 }
}

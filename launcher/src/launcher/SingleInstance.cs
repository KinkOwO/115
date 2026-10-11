using System;
using System.Runtime.InteropServices;
namespace Local115US {
 public static class SingleInstance {
  [DllImport("user32.dll",CharSet=CharSet.Unicode)]static extern uint RegisterWindowMessage(string value);
  [DllImport("user32.dll",SetLastError=true)]static extern IntPtr SendMessageTimeout(IntPtr window,uint message,UIntPtr w,IntPtr l,uint flags,uint timeout,out UIntPtr result);
  [DllImport("user32.dll",SetLastError=true)]static extern bool ChangeWindowMessageFilterEx(IntPtr window,uint message,uint action,IntPtr change);
  public static void AllowOpenMessage(IntPtr window,string root){ChangeWindowMessageFilterEx(window,Message(root),1,IntPtr.Zero);}
  public static uint Message(string root){return RegisterWindowMessage("115US.Unified.Open."+new PackageContext(root,Core.Client(root),false).InstanceKey);}
  public static void Notify(string root,bool gm){UIntPtr result;SendMessageTimeout(new IntPtr(0xffff),Message(root),new UIntPtr(gm?1u:0u),IntPtr.Zero,2,1000,out result);}
 }
}

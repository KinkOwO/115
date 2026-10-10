using System;
using System.IO;
using System.Diagnostics;
using System.Threading;
class LifecycleWorker {
 static int Main(string[] args){if(args.Length>0&&args[0]=="spawn"){using(var child=Process.Start(new ProcessStartInfo(System.Reflection.Assembly.GetExecutingAssembly().Location,"sleep"){UseShellExecute=false,CreateNoWindow=true}))File.WriteAllText(args[1],child.Id.ToString());}Thread.Sleep(60000);return 0;}
}

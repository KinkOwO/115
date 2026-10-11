using System;
using System.IO;
using System.Diagnostics;
using System.Windows.Forms;
[assembly: System.Reflection.AssemblyTitle("115US Unified GM Entry")]
[assembly: System.Reflection.AssemblyVersion("2.0.1.0")]
[assembly: System.Reflection.AssemblyFileVersion("2.0.1.0")]
class LegacyGMRedirect {
 [STAThread] static int Main(){try{
  string root=AppDomain.CurrentDomain.BaseDirectory.TrimEnd('\\');
  if(String.Equals(Path.GetFileName(root),"05_操作入口",StringComparison.OrdinalIgnoreCase))root=Path.GetDirectoryName(root);
  string exe=Path.Combine(root,"DFO-115US本地版登录器.exe");
  if(!File.Exists(exe))throw new Exception("未找到同目录的配套统一启动器，请完整安装本次整合包。");
  Process.Start(new ProcessStartInfo(exe,"--gm"){UseShellExecute=false,WorkingDirectory=root});return 0;
 }catch(Exception ex){MessageBox.Show(ex.Message,"115US GM入口",MessageBoxButtons.OK,MessageBoxIcon.Information);return 1;}}
}

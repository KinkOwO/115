using System;
using System.Globalization;

namespace Local115US {
 public sealed class ClientMemoryNotice {
  public const long LimitBytes = 16L * 1024 * 1024 * 1024;
  public const string DefaultHint = "客户端内存上限 16 GiB（原 8 GiB）；请保持 Windows 分页文件为系统管理。";
  public long PeakBytes { get; private set; }
  public bool NearLimit { get; private set; }
  public void Observe(long bytes) {
   if(bytes<0)return;
   PeakBytes=Math.Max(PeakBytes,bytes);
   if(bytes>=LimitBytes*9/10)NearLimit=true;
   else if(bytes<LimitBytes*85/100)NearLimit=false;
  }
  public string Hint(long bytes) {
   string amount=(bytes/(double)(1L<<30)).ToString("0.00",CultureInfo.InvariantCulture);
   return NearLimit ? "客户端内存接近上限："+amount+" / 16 GiB。建议完成当前操作后正常退出并重新进入游戏，避免继续占用。"
                    : "客户端内存 "+amount+" / 16 GiB（原上限 8 GiB）；请保持 Windows 分页文件为系统管理。";
  }
  public string ExitHint {
   get { return PeakBytes>=LimitBytes*9/10 ? "本次客户端内存曾接近 16 GiB 上限。如遇闪退，请保留运行日志；内存读数不能单独证明退出原因。可关闭其他占内存程序并重启游戏。" : ""; }
  }
 }
}

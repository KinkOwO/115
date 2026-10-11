// 重建 ImagePacks2\NpkIndex.etc。
//
// 为什么是独立 exe 而不是继续在 pwsh 里反射：
//   索引构建器 USLocalServer.Pvf.dll / USLocalServer.Launcher.Core.dll 的目标框架是
//   net10.0（依赖 System.Runtime 10.0.0.0）。包内那个 pwsh 便携版是**自包含 net8**
//   的（自包含应用按设计忽略 roll-forward），跑不到 net10，加载必然失败。
//   所以这里编一个自包含 net10 的 exe：目标机不用装任何 .NET 运行时，
//   rebuild-index.ps1 只是把它复制到 %TEMP% 后调用（115\ 树下加载 exe 会被系统拦）。
//
// 自检判据（与 pwsh 版一致）：
//   1) 构建器报告的 NpkFiles == 目录里实际 *.NPK 个数；
//   2) 第二次重建必须零改动（Changes 为 null）。
// 退出码：0 通过；1 失败。

using System;
using System.Collections.Generic;
using System.Globalization;
using System.IO;
using System.Linq;
using System.Reflection;
using System.Text;
using System.Threading;

internal static class Program
{
    private const int Ok = 0;
    private const int Fail = 1;

    private static int Main(string[] args)
    {
        try
        {
            Console.OutputEncoding = new UTF8Encoding(false);
        }
        catch
        {
            // 输出编码改不了也不影响判定
        }

        string ip = null;
        string tools = null;
        for (int i = 0; i < args.Length; i++)
        {
            switch (args[i])
            {
                case "--ip":
                    ip = Next(args, ref i);
                    break;
                case "--tools":
                    tools = Next(args, ref i);
                    break;
            }
        }

        Say("运行时     : .NET " + Environment.Version);
        Say("构建器目录 : " + tools);
        Say("图像包目录 : " + ip);
        Say("");

        if (string.IsNullOrEmpty(ip) || !Directory.Exists(ip))
        {
            Say("[FAIL] 图像包目录不存在: " + ip);
            return Fail;
        }
        if (string.IsNullOrEmpty(tools) || !Directory.Exists(tools))
        {
            Say("[FAIL] 构建器目录不存在: " + tools);
            return Fail;
        }

        try
        {
            var asmPvf = Assembly.LoadFrom(Path.Combine(tools, "USLocalServer.Pvf.dll"));
            var asmCore = Assembly.LoadFrom(Path.Combine(tools, "USLocalServer.Launcher.Core.dll"));
            Say("已加载     : " + asmPvf.GetName().Name + " / " + asmCore.GetName().Name);

            var method = FindBuilder(asmCore);
            if (method == null)
            {
                Say("[FAIL] 没找到索引构建方法（静态 A，3 参数，末位 CancellationToken）");
                return Fail;
            }
            Say("构建方法   : " + method.DeclaringType.FullName + "." + method.Name +
                "(" + string.Join(", ", method.GetParameters().Select(p => p.ParameterType.Name)) + ")");

            int real = Directory.GetFiles(ip, "*.NPK").Length;
            Say("NPK 总数   : " + real.ToString(CultureInfo.InvariantCulture));
            Say("");

            object second = null;
            for (int i = 1; i <= 2; i++)
            {
                Say("--- 第 " + i.ToString(CultureInfo.InvariantCulture) +
                    " 次重建（第 2 次用来验证索引已收敛）---");
                var t0 = DateTime.Now;
                object r = Invoke(method, ip);
                int sec = (int)(DateTime.Now - t0).TotalSeconds;
                Say(Describe(r));
                Say("--- 第 " + i.ToString(CultureInfo.InvariantCulture) + " 次完成，用时 " +
                    sec.ToString(CultureInfo.InvariantCulture) + " 秒 ---");
                Say("");
                second = r;
            }

            var idx = Path.Combine(ip, "NpkIndex.etc");
            if (!File.Exists(idx))
            {
                Say("[FAIL] 没有生成 NpkIndex.etc");
            }
            else
            {
                var fi = new FileInfo(idx);
                Say("索引文件   : " + idx);
                Say("索引大小   : " + fi.Length.ToString(CultureInfo.InvariantCulture) + " 字节");
            }

            bool c1 = true;
            int reported = GetInt(second, "NpkFiles", -1);
            if (reported != real)
            {
                Say("[FAIL] 构建器报告 NpkFiles=" + reported.ToString(CultureInfo.InvariantCulture) +
                    "，但目录里实际有 " + real.ToString(CultureInfo.InvariantCulture) + " 个 NPK，不一致");
                c1 = false;
            }
            else
            {
                Say("[OK]   NPK 总数一致：" + real.ToString(CultureInfo.InvariantCulture));
            }

            bool c2 = true;
            object changes = Get(second, "Changes");
            if (changes != null)
            {
                Say("[FAIL] 第二次重建仍有改动（Changes 不为 null），索引没有收敛");
                c2 = false;
            }
            else
            {
                Say("[OK]   第二次重建零改动（Changes = null），索引已收敛");
            }

            Say("");
            if (c1 && c2)
            {
                Say("[OK] 索引重建通过，可以启动游戏了。");
                return Ok;
            }
            Say("[FAIL] 索引重建没有通过，请不要启动游戏，先看上面的原因。");
            return Fail;
        }
        catch (Exception ex)
        {
            Say("[FAIL] 出错中断：" + ex.Message);
            if (ex is ReflectionTypeLoadException rtle && rtle.LoaderExceptions != null)
            {
                foreach (var le in rtle.LoaderExceptions.Where(x => x != null).Take(5))
                {
                    Say("       " + le.Message);
                }
            }
            if (ex.InnerException != null)
            {
                Say("       内部：" + ex.InnerException.Message);
            }
            return Fail;
        }
    }

    private static string Next(string[] args, ref int i)
    {
        return i + 1 < args.Length ? args[++i] : null;
    }

    /// <summary>在 Core 里找 A.h.A(string, ?, CancellationToken)；名字拿不到时全程序集扫。 </summary>
    private static MethodInfo FindBuilder(Assembly asm)
    {
        var flags = BindingFlags.Static | BindingFlags.Public | BindingFlags.NonPublic;
        bool Matches(MethodInfo m)
        {
            var ps = m.GetParameters();
            return m.Name == "A" && ps.Length == 3 &&
                   ps[0].ParameterType == typeof(string) &&
                   ps[2].ParameterType == typeof(CancellationToken);
        }

        Type t = null;
        try
        {
            t = asm.GetType("A.h", false, false);
        }
        catch
        {
            t = null;
        }

        if (t != null)
        {
            var hit = t.GetMethods(flags).Where(Matches).ToArray();
            if (hit.Length == 1)
            {
                return hit[0];
            }
        }

        try
        {
            foreach (var type in asm.GetTypes())
            {
                var hit = type.GetMethods(flags).Where(Matches).ToArray();
                if (hit.Length == 1)
                {
                    return hit[0];
                }
            }
        }
        catch (ReflectionTypeLoadException rtle)
        {
            foreach (var type in rtle.Types.Where(x => x != null))
            {
                var hit = type.GetMethods(flags).Where(Matches).ToArray();
                if (hit.Length == 1)
                {
                    return hit[0];
                }
            }
            throw;
        }
        return null;
    }

    private static object Invoke(MethodInfo method, string ip)
    {
        var ps = method.GetParameters();
        var argv = new object[3];
        argv[0] = ip;
        argv[1] = ps[1].ParameterType.IsValueType ? Activator.CreateInstance(ps[1].ParameterType) : null;
        argv[2] = CancellationToken.None;
        return method.Invoke(null, argv);
    }

    private static object Get(object o, string name)
    {
        if (o == null)
        {
            return null;
        }
        var t = o.GetType();
        var p = t.GetProperty(name, BindingFlags.Public | BindingFlags.Instance);
        if (p != null)
        {
            return p.GetValue(o);
        }
        var f = t.GetField(name, BindingFlags.Public | BindingFlags.Instance);
        return f != null ? f.GetValue(o) : null;
    }

    private static int GetInt(object o, string name, int fallback)
    {
        object v = Get(o, name);
        if (v == null)
        {
            return fallback;
        }
        try
        {
            return Convert.ToInt32(v, CultureInfo.InvariantCulture);
        }
        catch
        {
            return fallback;
        }
    }

    private static string Describe(object o)
    {
        if (o == null)
        {
            return "(返回 null)";
        }
        var t = o.GetType();
        var sb = new StringBuilder();
        sb.Append(t.Name).Append(" { ");
        var parts = new List<string>();
        foreach (var p in t.GetProperties(BindingFlags.Public | BindingFlags.Instance))
        {
            object v;
            try
            {
                v = p.GetValue(o);
            }
            catch
            {
                v = "<err>";
            }
            parts.Add(p.Name + "=" + Short(v));
        }
        sb.Append(string.Join(", ", parts));
        sb.Append(" }");
        return sb.ToString();
    }

    private static string Short(object v)
    {
        if (v == null)
        {
            return "null";
        }
        var s = v.ToString();
        if (s.Length > 60)
        {
            s = s.Substring(0, 60) + "...(共 " + s.Length + " 字符)";
        }
        return s;
    }

    private static void Say(string msg)
    {
        Console.WriteLine(msg);
    }
}

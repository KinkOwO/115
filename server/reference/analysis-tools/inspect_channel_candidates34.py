"""Read only the selected-actor map in the owned, isolated DFO probe process."""
import ctypes as c, ctypes.wintypes as w, pathlib, sys, json, struct, datetime
pid=int(sys.argv[1]); ident=int(sys.argv[2]); dest=pathlib.Path(sys.argv[3])
if not 0<ident<65535: raise ValueError('invalid actor ID')
k=c.WinDLL('kernel32',use_last_error=True); ps=c.WinDLL('psapi',use_last_error=True)
k.OpenProcess.argtypes=[w.DWORD,w.BOOL,w.DWORD]; k.OpenProcess.restype=w.HANDLE
k.QueryFullProcessImageNameW.argtypes=[w.HANDLE,w.DWORD,w.LPWSTR,c.POINTER(w.DWORD)]
k.ReadProcessMemory.argtypes=[w.HANDLE,c.c_void_p,c.c_void_p,c.c_size_t,c.POINTER(c.c_size_t)]
k.CloseHandle.argtypes=[w.HANDLE]
ps.EnumProcessModules.argtypes=[w.HANDLE,c.c_void_p,w.DWORD,c.POINTER(w.DWORD)]
h=k.OpenProcess(0x410,False,pid)
if not h: raise c.WinError(c.get_last_error())
try:
 name=c.create_unicode_buffer(32768); size=w.DWORD(len(name))
 if not k.QueryFullProcessImageNameW(h,0,name,c.byref(size)):raise c.WinError(c.get_last_error())
 expected=pathlib.Path(__file__).resolve().parent.parent/'dfo_probe_client/DFO.exe'
 if pathlib.Path(name.value).resolve()!=expected.resolve():raise ValueError('process is not the owned isolated DFO client')
 modules=(c.c_void_p*1024)(); needed=w.DWORD()
 if not ps.EnumProcessModules(h,modules,c.sizeof(modules),c.byref(needed)):raise c.WinError(c.get_last_error())
 base=modules[0];delta=base-0x140000000
 def read(addr,n):
  if not 0<n<=2048 or addr<65536:raise ValueError('invalid bounded read')
  buf=c.create_string_buffer(n); got=c.c_size_t()
  if not k.ReadProcessMemory(h,addr,buf,n,c.byref(got)) or got.value!=n:raise c.WinError(c.get_last_error())
  return buf.raw
 def u(addr,n=8):return int.from_bytes(read(addr,n),'little')
 def globalu(addr,n=8):return u(addr+delta,n)
 def protected(addr):return ((globalu(addr,4)^0x1f2a025c)-4)&0xffffffff

 vector=0x144d9aa90+7+0x98cda91+delta
 begin=u(vector); end=u(vector+8); rows=[]
 if (end-begin)//8>128:raise ValueError("unexpected channel vector")
 for i in range((end-begin)//8):
  obj=u(begin+i*8); vt=u(obj); rows.append({"pointer":hex(obj),"vtable":hex(vt),"raw_hex":read(obj,0x180).hex(),"head_words":[u(obj+j,4) for j in range(8,0xa4,4)],"getters":{hex(off):hex(u(vt+off)) for off in (8,16,0x118,0x288)}})
 module=globalu(0x14e6839f0)
 result={"channels":rows,"module":hex(module),"auto_tries":u(module+0x90,4),"connect_globals":[globalu(0x145503865+7+0x918018c,4),globalu(0x14550386e+7+0x8763eef,4)]}
 dest.write_text(json.dumps(result,indent=2));print(json.dumps(result))
finally:k.CloseHandle(h)

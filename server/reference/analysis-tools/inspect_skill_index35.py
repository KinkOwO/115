"""Bounded read-only snapshot of job skill indexes in the isolated client."""
import ctypes as c,ctypes.wintypes as w,struct,json,sys
from pathlib import Path
pid=int(sys.argv[1]);dest=Path(sys.argv[2])
k=c.WinDLL('kernel32',use_last_error=True);ps=c.WinDLL('psapi',use_last_error=True)
k.OpenProcess.argtypes=[w.DWORD,w.BOOL,w.DWORD];k.OpenProcess.restype=w.HANDLE
k.QueryFullProcessImageNameW.argtypes=[w.HANDLE,w.DWORD,w.LPWSTR,c.POINTER(w.DWORD)]
k.ReadProcessMemory.argtypes=[w.HANDLE,c.c_void_p,c.c_void_p,c.c_size_t,c.POINTER(c.c_size_t)]
k.CloseHandle.argtypes=[w.HANDLE]
ps.EnumProcessModules.argtypes=[w.HANDLE,c.c_void_p,w.DWORD,c.POINTER(w.DWORD)]
h=k.OpenProcess(0x410,False,pid)
if not h:raise c.WinError(c.get_last_error())
try:
 name=c.create_unicode_buffer(32768);size=w.DWORD(len(name))
 if not k.QueryFullProcessImageNameW(h,0,name,c.byref(size)):raise c.WinError(c.get_last_error())
 expected=Path(__file__).resolve().parent.parent/'dfo_probe_client/DFO.exe'
 if Path(name.value).resolve()!=expected.resolve():raise ValueError('not isolated DFO client')
 modules=(c.c_void_p*1024)();needed=w.DWORD()
 if not ps.EnumProcessModules(h,modules,c.sizeof(modules),c.byref(needed)):raise c.WinError(c.get_last_error())
 delta=modules[0]-0x140000000
 def read(addr,n):
  if addr<65536 or not 0<n<=4096:raise ValueError('bounded read rejected')
  b=c.create_string_buffer(n);got=c.c_size_t()
  if not k.ReadProcessMemory(h,addr,b,n,c.byref(got)) or got.value!=n:raise c.WinError(c.get_last_error())
  return b.raw
 def u(addr,n=8):return int.from_bytes(read(addr,n),'little')
 def text(addr):
  chars=[]
  for i in range(512):
   b=read(addr+i*2,2)
   if b==b'\0\0':return b''.join(chars).decode('utf-16le','replace')
   chars.append(b)
  raise ValueError('unterminated index string')
 rows=[]
 for job in (0,16):
  table=0x14f36e640+delta+job*0x70
  head=u(table);row={'job':job,'head':hex(head),'count':u(table+8),'skills':[]}
  if head:
   todo=[u(head+8)];seen=set()
   while todo:
    node=todo.pop()
    if node==head or node in seen:continue
    if len(seen)>=1024:raise ValueError('index node bound')
    seen.add(node)
    if u(node+0x19,1):continue
    idx=u(node+0x20,4)
    if idx in (3,7,46,169,174,179,190,452):row['skills'].append({'id':idx,'path':text(u(node+0x28))})
    todo.extend((u(node),u(node+0x10)))
   row['visited']=len(seen)
  rows.append(row)
 result={'pid':pid,'module_base':hex(modules[0]),'scope':'read-only index definitions; no UI actions or live writes','jobs':rows}
 dest.write_text(json.dumps(result,indent=2));print(json.dumps(result,indent=2))
finally:k.CloseHandle(h)

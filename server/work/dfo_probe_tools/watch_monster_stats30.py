"""Read-only owned-client stat snapshots, without debugger stops or game writes.

Exact-build lookup: scene+110 entity manager, type3 FNV map at+78;
145df2df0 resolves shared node+28 minus30. HP descriptor is selected by
145c14140(actor,3), used by145c199c0/145c19a40. Preserve raw bytes so the
original getter can be replayed offline; do not guess effective damage.
"""
import ctypes as c
import ctypes.wintypes as w
import datetime,json,pathlib,re,sys,time

def watch(out):
 k=c.WinDLL('kernel32',use_last_error=True);ps=c.WinDLL('psapi',use_last_error=True)
 k.OpenProcess.argtypes=[w.DWORD,w.BOOL,w.DWORD];k.OpenProcess.restype=w.HANDLE
 k.ReadProcessMemory.argtypes=[w.HANDLE,c.c_void_p,c.c_void_p,c.c_size_t,c.POINTER(c.c_size_t)]
 k.QueryFullProcessImageNameW.argtypes=[w.HANDLE,w.DWORD,w.LPWSTR,c.POINTER(w.DWORD)]
 k.GetExitCodeProcess.argtypes=[w.HANDLE,c.POINTER(w.DWORD)]
 k.CloseHandle.argtypes=[w.HANDLE]
 ps.EnumProcessModules.argtypes=[w.HANDLE,c.c_void_p,w.DWORD,c.POINTER(w.DWORD)]
 pid=None
 for _ in range(120):
  log=out/'client.log'
  if log.exists():
   match=re.search(r'ROOT_PID (\d+)',log.read_text(errors='replace'))
   if match:pid=int(match[1]);break
  time.sleep(.5)
 if not pid:raise RuntimeError('owned client PID not recorded')
 h=k.OpenProcess(0x410,False,pid)
 if not h:raise c.WinError(c.get_last_error())
 try:
  path=c.create_unicode_buffer(32768);size=w.DWORD(len(path))
  if not k.QueryFullProcessImageNameW(h,0,path,c.byref(size)):raise c.WinError(c.get_last_error())
  expected=pathlib.Path(__file__).resolve().parent.parent/'dfo_probe_client/DFO.exe'
  if pathlib.Path(path.value).resolve()!=expected.resolve():raise RuntimeError('not the owned isolated client')
  modules=(c.c_void_p*1024)();needed=w.DWORD()
  # ROOT_PID is logged before ResumeThread; the loader may not have mapped
  # the module list yet. Retry the documented startup race (WinError299).
  for _ in range(120):
   if ps.EnumProcessModules(h,modules,c.sizeof(modules),c.byref(needed)) and needed.value and modules[0]:break
   time.sleep(.5)
  else:raise RuntimeError('owned client module list unavailable after startup')
  delta=modules[0]-0x140000000
  def read(addr,n):
   if addr<65536 or not 0<n<=2048:raise ValueError('invalid bounded read')
   b=c.create_string_buffer(n);got=c.c_size_t()
   if not k.ReadProcessMemory(h,addr,b,n,c.byref(got)) or got.value!=n:raise c.WinError(c.get_last_error())
   return b.raw
  def u(addr,n=8):return int.from_bytes(read(addr,n),'little')
  seen={};errors=set()
  with (out/'monster-stats.jsonl').open('a',encoding='utf-8') as log:
   def emit(row):
    row.update(time=datetime.datetime.now(datetime.timezone.utc).isoformat(),pid=pid)
    log.write(json.dumps(row)+'\n');log.flush()
   emit({'kind':'observer_started','read_only':True})
   while True:
    status=w.DWORD()
    if not k.GetExitCodeProcess(h,c.byref(status)) or status.value!=259:break
    try:
     root=u(0x14e6839f0+delta)
     if root and u(u(root)+0xd8)==0x144ed9c60+delta:
      scene=u(root+0x90)
      control=u(scene+0x108) if scene else 0
      manager=u(scene+0x110) if control and u(control+8,4) else 0
      if manager:
       sentinel=u(manager+0x78);node=u(sentinel);visited=set()
       while node!=sentinel and node not in visited and len(visited)<512:
        visited.add(node)
        key=u(node+0x10,4)
        if key>>16==3:
         control=u(node+0x20);ref=u(node+0x28)
         if control and u(control+8,4) and ref:
          actor=ref-0x30;vt=u(actor)
          if u(vt+0x12c8)==0x145c14140+delta:
           raw=read(actor+0x2368+3*0xc38,0xa0)
           identity=(scene,key,actor)
           if seen.get(identity)!=raw:
            seen[identity]=raw
            emit({'kind':'monster_hp_descriptor','entity':key&65535,'actor':hex(actor),'vtable':hex(vt),'team':u(actor+0xf10,4),'raw_hex':raw.hex(),'getter':'145c199c0/147220e40'})
        node=u(node)
       if len(seen)>4096:seen.clear()
    except (OSError,ValueError) as error:
     # Objects can disappear between reads during map transitions.
     name=type(error).__name__+': '+str(error)
     if name not in errors and len(errors)<16:errors.add(name);emit({'kind':'observer_read_retry','reason':name})
    time.sleep(.5)
   emit({'kind':'observer_finished'})
 finally:k.CloseHandle(h)

if __name__=='__main__':watch(pathlib.Path(sys.argv[1]).resolve())

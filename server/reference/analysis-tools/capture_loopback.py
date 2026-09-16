import socket,subprocess,time,pathlib,json,hashlib,sys
p=pathlib.Path(__file__).parent
normal='--normal' in sys.argv;prefix='normal' if normal else 'loopback'
record={'purpose':'Passive loopback capture with synthetic account fields; no protocol response is sent.','chunks':[]}
with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as listener:
 listener.bind(('127.0.0.1',0));listener.listen(1);listener.settimeout(1)
 port=listener.getsockname()[1];record['listen_host']='127.0.0.1';record['listen_port']=port
 payload=f'13?127.0.0.1?{port}?probe?00000000000000000000000000000000?0?0?30?0?0?0'
 record['synthetic_launch_payload']=payload
 proc=subprocess.Popen([str(p/'probe.exe'),str(p.parent/'dfo_probe_client'),str(p/(prefix+'_ui.log')),'55','normal-ui' if normal else 'trace-ui',str(p/'argument_breakpoints.txt'),payload],cwd=p,creationflags=subprocess.CREATE_NO_WINDOW)
 print('STARTED',proc.pid,'LISTEN',port,flush=True);start=time.monotonic();conn=None;blob=bytearray()
 try:
  while time.monotonic()-start<58 and proc.poll() is None:
   if conn is None:
    try:
     conn,peer=listener.accept();conn.settimeout(1);record['accepted_after_seconds']=round(time.monotonic()-start,3);record['peer']=peer;print('ACCEPTED',peer,flush=True)
    except socket.timeout:continue
   try:
    data=conn.recv(4096)
    if not data:
     record['peer_closed_after_seconds']=round(time.monotonic()-start,3);conn.close();conn=None;continue
    blob.extend(data);record['chunks'].append({'after_seconds':round(time.monotonic()-start,3),'size':len(data),'hex':data.hex()});print('RECEIVED',len(data),'bytes',flush=True)
    if len(blob)>65536:break
   except socket.timeout:pass
   except ConnectionResetError:
    record['peer_reset_after_seconds']=round(time.monotonic()-start,3);conn.close();conn=None
 finally:
  if conn:conn.close()
  try:record['probe_exit_code']=proc.wait(timeout=5)
  except subprocess.TimeoutExpired:proc.terminate();record['probe_exit_code']=proc.wait(timeout=5)
 record['elapsed_seconds']=round(time.monotonic()-start,3);record['received_bytes']=len(blob);record['received_sha256']=hashlib.sha256(blob).hexdigest();record['response_sent_bytes']=0
 (p/(prefix+'_capture.bin')).write_bytes(blob);(p/(prefix+'_capture.json')).write_text(json.dumps(record,indent=2),encoding='utf-8');print(json.dumps(record,indent=2),flush=True)

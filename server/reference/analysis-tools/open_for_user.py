"""Keep the authorized client and passive local listener open until the user exits."""
import socket,subprocess,time,pathlib,json,os
p=pathlib.Path(__file__).parent
state={'purpose':'User-requested interactive preview; synthetic credentials, no server responses.', 'helper_pid':os.getpid(),'status':'starting','received_bytes':0}
def save():
 (p/'interactive_state.json').write_text(json.dumps(state,indent=2),encoding='utf-8')
with socket.socket(socket.AF_INET,socket.SOCK_STREAM) as listener:
 listener.bind(('127.0.0.1',0));listener.listen(1);listener.settimeout(1)
 port=listener.getsockname()[1];state['listen_host']='127.0.0.1';state['listen_port']=port
 payload=f'13?127.0.0.1?{port}?probe?00000000000000000000000000000000?0?0?30?0?0?0'
 proc=subprocess.Popen([str(p/'probe.exe'),str(p.parent/'dfo_probe_client'),str(p/'interactive_ui.log'),'55','interactive-ui',str(p/'argument_breakpoints.txt'),payload],cwd=p,creationflags=subprocess.CREATE_NO_WINDOW)
 state['probe_pid']=proc.pid;state['status']='waiting_for_client';save();start=time.monotonic();conn=None
 try:
  while proc.poll() is None:
   if conn is None:
    try:
     conn,peer=listener.accept();conn.settimeout(1);state['status']='client_connected';state['peer']=peer;state['accepted_after_seconds']=round(time.monotonic()-start,3);save()
    except socket.timeout:continue
   try:
    data=conn.recv(4096)
    if not data:
     conn.close();conn=None;state['status']='client_disconnected';save();continue
    state['received_bytes']+=len(data);save()
   except socket.timeout:pass
   except ConnectionResetError:
    conn.close();conn=None;state['status']='client_disconnected';save()
 finally:
  if conn:conn.close()
  if proc.poll() is None:proc.terminate()
  state['probe_exit_code']=proc.wait(timeout=5);state['status']='closed';state['elapsed_seconds']=round(time.monotonic()-start,3);save()

"""Initialize isolated development storage; never use another project's data."""
import argparse,pathlib,json,secrets,subprocess,os,socket

FLAGS=getattr(subprocess,'CREATE_NO_WINDOW',0)

def initialize(postgres_bin,postgres_port,runtime):
 runtime=pathlib.Path(runtime)
 runtime.mkdir(parents=True,exist_ok=True)
 configfile=runtime/'local.json'
 if configfile.exists() or (runtime/'pgdata').exists():
  raise SystemExit('Storage configuration already exists; use it to inspect/restart rather than reinitialize')
 with socket.socket() as s:s.bind(('127.0.0.1',postgres_port))
 flags=FLAGS
 pgbin=pathlib.Path(postgres_bin).resolve()
 for dependency in [pgbin/'initdb.exe',pgbin/'pg_ctl.exe',pgbin/'createdb.exe']:
  if not dependency.is_file():raise SystemExit('Missing dependency: '+str(dependency))
 pgpass=secrets.token_urlsafe(32)
 passwordfile=runtime/'initdb-password.tmp'
 passwordfile.write_text(pgpass,encoding='utf-8')
 data=runtime/'pgdata'
 try:
  result=subprocess.run([str(pgbin/'initdb.exe'),'-D',str(data),'-U','dfo_owner','--pwfile',str(passwordfile),'--auth-host=scram-sha-256','--auth-local=scram-sha-256','--encoding=UTF8','--locale=C'],capture_output=True,creationflags=flags)
  (runtime/'initdb.log').write_bytes(result.stdout+result.stderr)
  if result.returncode:raise RuntimeError('initdb failed; see runtime/storage/initdb.log')
 finally:passwordfile.unlink(missing_ok=True)
 with (data/'postgresql.conf').open('a') as f:
  f.write(f"\nlisten_addresses = '127.0.0.1'\nport = {postgres_port}\nmax_connections = 30\nshared_buffers = '64MB'\n")
 with (runtime/'pg-control.log').open('ab') as control_log:
  subprocess.run([str(pgbin/'pg_ctl.exe'),'-D',str(data),'-l',str(runtime/'postgres.log'),'-w','start'],check=True,stdout=control_log,stderr=control_log,creationflags=flags)
 env=os.environ.copy();env['PGPASSWORD']=pgpass
 subprocess.run([str(pgbin/'createdb.exe'),'-h','127.0.0.1','-p',str(postgres_port),'-U','dfo_owner','dfo_lan'],env=env,check=True,capture_output=True,creationflags=flags)
 config={'postgres_dsn':f'postgres://dfo_owner:{pgpass}@127.0.0.1:{postgres_port}/dfo_lan?sslmode=disable','max_connections':12,'postgres_bin':str(pgbin),'postgres_data':str(data)}
 configfile.write_text(json.dumps(config,indent=2),encoding='utf-8')
 return {'config_path':str(configfile),'postgres_port':postgres_port}

def main():
 parser=argparse.ArgumentParser()
 parser.add_argument('--postgres-bin',required=True)
 parser.add_argument('--postgres-port',type=int,default=25438)
 args=parser.parse_args()
 root=pathlib.Path(__file__).resolve().parent.parent
 print(json.dumps(initialize(args.postgres_bin,args.postgres_port,root/'runtime/storage')))

if __name__ == '__main__':
 main()

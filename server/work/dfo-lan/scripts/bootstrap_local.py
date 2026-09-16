"""Initialize isolated development storage; never use another project's data."""
import argparse,pathlib,json,secrets,subprocess,os,socket,time

parser=argparse.ArgumentParser()
parser.add_argument('--postgres-bin',required=True)
parser.add_argument('--redis-bin',required=True)
parser.add_argument('--postgres-port',type=int,default=25438)
parser.add_argument('--redis-port',type=int,default=26388)
args=parser.parse_args()
root=pathlib.Path(__file__).resolve().parent.parent
runtime=root/'runtime/storage'
runtime.mkdir(parents=True,exist_ok=True)
configfile=runtime/'local.json'
if configfile.exists() or (runtime/'pgdata').exists():
 raise SystemExit('Storage configuration already exists; use it to inspect/restart rather than reinitialize')
for port in [args.postgres_port,args.redis_port]:
 with socket.socket() as s:s.bind(('127.0.0.1',port))
flags=subprocess.CREATE_NO_WINDOW
pgbin=pathlib.Path(args.postgres_bin).resolve()
redisbin=pathlib.Path(args.redis_bin).resolve()
for dependency in [pgbin/'initdb.exe',pgbin/'pg_ctl.exe',pgbin/'createdb.exe',redisbin/'redis-server.exe']:
 if not dependency.is_file():raise SystemExit('Missing dependency: '+str(dependency))
pgpass=secrets.token_urlsafe(32);redispass=secrets.token_urlsafe(32)
passwordfile=runtime/'initdb-password.tmp'
passwordfile.write_text(pgpass,encoding='utf-8')
data=runtime/'pgdata'
try:
 result=subprocess.run([str(pgbin/'initdb.exe'),'-D',str(data),'-U','dfo_owner','--pwfile',str(passwordfile),'--auth-host=scram-sha-256','--auth-local=scram-sha-256','--encoding=UTF8','--locale=C'],capture_output=True,creationflags=flags)
 (runtime/'initdb.log').write_bytes(result.stdout+result.stderr)
 if result.returncode:raise RuntimeError('initdb failed; see runtime/storage/initdb.log')
finally:passwordfile.unlink(missing_ok=True)
with (data/'postgresql.conf').open('a') as f:
 f.write(f"\nlisten_addresses = '127.0.0.1'\nport = {args.postgres_port}\nmax_connections = 30\nshared_buffers = '64MB'\n")
with (runtime/'pg-control.log').open('ab') as control_log:
 subprocess.run([str(pgbin/'pg_ctl.exe'),'-D',str(data),'-l',str(runtime/'postgres.log'),'-w','start'],check=True,stdout=control_log,stderr=control_log,creationflags=flags)
env=os.environ.copy();env['PGPASSWORD']=pgpass
subprocess.run([str(pgbin/'createdb.exe'),'-h','127.0.0.1','-p',str(args.postgres_port),'-U','dfo_owner','dfo_lan'],env=env,check=True,capture_output=True,creationflags=flags)
rediscfg=runtime/'redis.conf'
rediscfg.write_text(f'''bind 127.0.0.1
protected-mode yes
port {args.redis_port}
requirepass {redispass}
maxmemory 64mb
maxmemory-policy allkeys-lru
save ""
appendonly no
''',encoding='utf-8')
with (runtime/'redis.log').open('ab') as log:
 redis=subprocess.Popen([str(redisbin/'redis-server.exe'),rediscfg.name],cwd=runtime,stdout=log,stderr=log,creationflags=flags)
config={'postgres_dsn':f'postgres://dfo_owner:{pgpass}@127.0.0.1:{args.postgres_port}/dfo_lan?sslmode=disable','redis_address':f'127.0.0.1:{args.redis_port}','redis_password':redispass,'redis_prefix':'dfo-lan:','redis_bin':str(redisbin),'max_connections':12,'postgres_bin':str(pgbin),'redis_pid':redis.pid,'postgres_data':str(data)}
configfile.write_text(json.dumps(config,indent=2),encoding='utf-8')
print(json.dumps({'config_path':str(configfile),'postgres_port':args.postgres_port,'redis_port':args.redis_port,'redis_pid':redis.pid}))

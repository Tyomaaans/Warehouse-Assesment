#!/usr/bin/env python3
"""Task 3D/3E local production gate. Creates a unique stack; never deletes volumes.

Requires Docker/Compose, the documented Go/Node/npm toolchain, installed frontend
packages, and Chrome. Runtime secrets, logs, captures and state stay in /tmp.
"""
import ipaddress
import json
import os
import re
from pathlib import Path
import secrets
import socket
import subprocess
import tempfile
import time
import urllib.error
import urllib.request

ROOT = Path(__file__).resolve().parents[1]
STATE = Path(tempfile.mkdtemp(prefix='warehouse-production-'))
os.chmod(STATE, 0o700)
project = 'wh-proof-' + secrets.token_hex(4)
def port():
    with socket.socket() as sock:
        sock.bind(('127.0.0.1', 0))
        return str(sock.getsockname()[1])
network_ids = subprocess.check_output(['docker','network','ls','-q'], text=True).split()
networks = json.loads(subprocess.check_output(['docker','network','inspect',*network_ids], text=True))
used = [ipaddress.ip_network(item['Subnet']) for network in networks for item in (network['IPAM'].get('Config') or []) if item.get('Subnet')]
subnet = next(f'172.30.{number}.0/24' for number in range(94, 240) if not any(ipaddress.ip_network(f'172.30.{number}.0/24').overlaps(other) for other in used if other.version == 4))
env = dict(os.environ, API_IMAGE='warehouse-api:task3de-local', FRONTEND_IMAGE='warehouse-frontend:task3de-local', BINARY_SOURCE='build', ASSET_SOURCE='build', APP_ENV='production', TRUSTED_PROXIES='', API_PORT=port(), FRONTEND_PORT=port(), POSTGRES_PASSWORD=secrets.token_hex(24), REDIS_PASSWORD=secrets.token_hex(24), RABBITMQ_PASSWORD=secrets.token_hex(24), JWT_SECRET_KEY=secrets.token_hex(32), WAREHOUSE_SUBNET=subnet, OPERATOR_EMAIL='operator@warehouse.test', OPERATOR_PASSWORD=secrets.token_hex(24), SUPERVISOR_EMAIL='supervisor@warehouse.test', SUPERVISOR_PASSWORD=secrets.token_hex(24))
env_file = STATE / 'runtime.env'
env_file.write_text('\n'.join(f'{key}={env[key]}' for key in ('API_IMAGE','FRONTEND_IMAGE','BINARY_SOURCE','ASSET_SOURCE','APP_ENV','API_PORT','FRONTEND_PORT','POSTGRES_PASSWORD','REDIS_PASSWORD','RABBITMQ_PASSWORD','JWT_SECRET_KEY','WAREHOUSE_SUBNET','TRUSTED_PROXIES'))+'\n')
os.chmod(env_file, 0o600)
(STATE/'runtime.json').write_text(json.dumps({key: value for key, value in env.items() if key not in os.environ or key in ('APP_ENV','TRUSTED_PROXIES')}))
os.chmod(STATE/'runtime.json', 0o600)
base = ['docker','compose','-p',project,'--env-file',str(env_file),'-f',str(ROOT/'compose.production.yml')]
counter = 0
captures = []
def run(args, cwd=ROOT, extra=None, show=False, timeout=1200):
    global counter
    counter += 1
    log = STATE / f'{counter:02d}-{Path(args[0]).name}.log'
    result = subprocess.run(args, cwd=cwd, env={**env, **(extra or {})}, stdout=subprocess.PIPE, stderr=subprocess.STDOUT, timeout=timeout)
    log.write_bytes(result.stdout)
    if show: print(result.stdout.decode(errors='replace'), flush=True)
    if result.returncode:
        diagnostic=result.stdout.decode(errors='replace')
        for key, value in env.items():
            if re.search('PASSWORD|SECRET', key) and len(value)>8:
                diagnostic=diagnostic.replace(value, '[redacted]')
        print(diagnostic[-8000:], flush=True)
        raise RuntimeError(f'{args[0]} {args[1] if len(args)>1 else ""} failed ({result.returncode}); private log {log}')
    return result.stdout.decode().strip()
def compose(*args, **options): return run(base+list(args), **options)
def container(service): return compose('ps','-aq',service)
def request(path, method='GET', body=None, auth=None, frontend=False, status=200, extra=None):
    headers = {'Accept':'application/json', **(extra or {})}
    if auth: headers.update(Authorization='Bearer '+auth['token'], Cookie=auth['cookies'])
    data = None
    if body is not None: data=json.dumps(body).encode(); headers['Content-Type']='application/json'
    url=f'http://127.0.0.1:{env["FRONTEND_PORT" if frontend else "API_PORT"]}'+path
    req=urllib.request.Request(url, data=data, method=method, headers=headers)
    try: response=urllib.request.urlopen(req, timeout=20)
    except urllib.error.HTTPError as failure: response=failure
    text=response.read().decode()
    assert response.status==status, f'{method} {path} expected {status}, got {response.status}'
    if not path.startswith('/api/') or path.startswith('/api/v1/auth/'):
        return text, response.headers
    captures.append({'path':path.split('?')[0].removeprefix('/api/v1'),'method':method,'status':response.status,'body_text':text,'request_text':data.decode() if data else None})
    return json.loads(text).get('data'), response.headers

def login(role):
    text, headers=request('/api/v1/auth/login','POST',{'email':env[role+'_EMAIL'],'password':env[role+'_PASSWORD'],'remember_me':False},frontend=True)
    data=json.loads(text)['data']
    cookies=headers.get_all('Set-Cookie')
    assert len(cookies)==2 and all('HttpOnly' in cookie and 'Secure' in cookie and 'SameSite=Strict' in cookie for cookie in cookies)
    return {'token':data['access_token'],'cookies':'; '.join(cookie.split(';')[0] for cookie in cookies),'user':data['user']}
def until(check, label, seconds=90):
    end=time.monotonic()+seconds
    while time.monotonic()<end:
        if check(): return
        time.sleep(1)
    raise AssertionError(label+' timed out')
def sql(statement): return compose('exec','-T','postgres','psql','-U','warehouse','-d','warehouse','-tAc',statement)
def snapshot(): return sql('SELECT (SELECT count(*) FROM products),(SELECT count(*) FROM receiving_documents),(SELECT sum(quantity) FROM inventory_balances),(SELECT count(*) FROM inventory_transactions),(SELECT count(*) FROM notifications);')
def emit(message): print(message, flush=True)

emit(f'Isolated production gate: project {project}; private evidence {STATE}')
try:
    compose('config','--quiet')
    if os.environ.get('VERIFY_SKIP_BUILD')!='1':
        compose('build','api','frontend')
    emit('PASS production Compose validation and API/frontend runtime images')
    compose('up','-d','--no-build','--wait','--wait-timeout','180','postgres','redis','rabbitmq')
    compose('run','--rm','--no-deps','migrate')
    compose('run','--rm','--no-deps','migrate')
    emit('PASS infrastructure readiness and repeat-safe numbered migrations')
    addresses={}
    for service in ('postgres','redis','rabbitmq'):
        record=json.loads(run(['docker','inspect',container(service)]))[0]
        addresses[service]=next(iter(record['NetworkSettings']['Networks'].values()))['IPAddress']
        assert not any(record['NetworkSettings']['Ports'].values()), service+' unexpectedly published'
    test_env={'TEST_DATABASE_URL':f'postgres://warehouse:{env["POSTGRES_PASSWORD"]}@{addresses["postgres"]}:5432/warehouse?sslmode=disable','TEST_REDIS_URL':f'redis://:{env["REDIS_PASSWORD"]}@{addresses["redis"]}:6379/0','TEST_RABBITMQ_URL':f'amqp://warehouse:{env["RABBITMQ_PASSWORD"]}@{addresses["rabbitmq"]}:5672/','CGO_ENABLED':'1'}
    output=run(['go','test','-count=1','-v','./...'],cwd=ROOT/'backend',extra=test_env)
    assert '--- SKIP' not in output
    emit('PASS full Go tests with actual PostgreSQL/Redis/RabbitMQ; no integration skips')
    run(['go','build','./...'],cwd=ROOT/'backend')
    run(['go','vet','./...'],cwd=ROOT/'backend')
    race=run(['go','test','-race','-count=1','./internal/notifications/...','./internal/router/...','./internal/receiving/...'],cwd=ROOT/'backend',extra=test_env)
    emit('PASS Go build/vet and affected-package race tests with real infrastructure')
    compose('run','--rm','--no-deps','-e','OPERATOR_EMAIL','-e','OPERATOR_PASSWORD','-e','SUPERVISOR_EMAIL','-e','SUPERVISOR_PASSWORD','api','provision-users')
    compose('up','-d','--no-build','--wait','--wait-timeout','120','api','frontend')
    for path in ('/api/v1/health','/api/v1/ready'): request(path)
    html,_=request('/',frontend=True)
    spa,_=request('/transactions/review',frontend=True)
    assert html==spa and '/assets/' in html and '@vite/client' not in html
    request('/assets/does-not-exist.js',frontend=True,status=404)
    request('/.env',frontend=True,status=404)
    request('/api/v1/notifications',status=401)
    emit('PASS API health/readiness, production HTTP assets, SPA fallback, protected routes and missing-asset/private-file handling')
    operator=login('OPERATOR'); supervisor=login('SUPERVISOR')
    # Observe the real host-to-container peer, not an assumed gateway.
    _, probe=request('/api/v1/health')
    request_id=probe['X-Request-ID']
    logs=compose('logs','--no-color','api')
    entry=next(json.loads(line[line.index('{'):]) for line in logs.splitlines() if request_id in line)
    peer=entry['peer_address'].rsplit(':',1)[0]
    assert peer==entry['client_ip'] and ipaddress.ip_address(peer) in ipaddress.ip_network(subnet)
    emit('Observed direct host -> Gin peer: '+peer+'; initial proxy trust is empty')
    prefix='PROD-'+secrets.token_hex(4)
    product,_=request('/api/v1/products','POST',{'sku':prefix,'name':'Production proof cartons'},supervisor,status=201)
    def receive(quantity):
        document,_=request('/api/v1/receivings','POST',{'order_reference':prefix+'-'+secrets.token_hex(3),'items':[{'product_id':product['id'],'expected_quantity':quantity}]},operator,status=201)
        transaction,_=request('/api/v1/receivings/'+document['id']+'/transactions','POST',{'items':[{'receiving_document_item_id':document['items'][0]['id'],'actual_quantity':quantity}]},operator,status=201)
        return document,transaction
    document,transaction=receive(2)
    balances,_=request('/api/v1/inventory?search='+prefix,auth=operator)
    assert balances['balances']==[]
    request('/api/v1/transactions/'+transaction['id']+'/approve','POST',auth=operator,status=403)
    request('/api/v1/transactions/'+transaction['id']+'/approve','POST',auth=supervisor)
    request('/api/v1/transactions/'+transaction['id']+'/approve','POST',auth=supervisor,status=409)
    assert int(sql('SELECT count(*) FROM notification_outbox WHERE published_at IS NULL'))==2
    assert sql('SELECT sum(quantity),count(*) FROM inventory_balances')=='2|1'
    assert sql('SELECT count(*) FROM inventory_transactions')=='1'
    emit('PASS product/RCV/actual receipt/approval: no pending stock, exactly one ledger posting, atomic persisted outbox')
    compose('up','-d','--no-build','--wait','--wait-timeout','180','outbox-publisher','notification-consumer')
    def mine(actor): return request('/api/v1/notifications?limit=100',auth=actor)[0]['notifications']
    until(lambda: len(mine(operator))==2,'worker delivery')
    own=mine(operator)[0]
    assert all(item['recipient_id']==operator['user']['id'] for item in mine(operator))
    request('/api/v1/notifications/'+own['id']+'/read','PATCH',auth=supervisor,status=404)
    marked,_=request('/api/v1/notifications/'+own['id']+'/read','PATCH',auth=operator)
    repeated,_=request('/api/v1/notifications/'+own['id']+'/read','PATCH',auth=operator)
    assert marked['read_at']==repeated['read_at'] and marked['read_at']
    emit('PASS confirmed outbox publication, RabbitMQ consumption, own notification GET/PATCH and recipient isolation')
    compose('stop','rabbitmq')
    _,during_outage=receive(1)
    request('/api/v1/transactions/'+during_outage['id']+'/approve','POST',auth=supervisor)
    request('/api/v1/ready')
    assert sql('SELECT sum(quantity) FROM inventory_balances')=='3'
    assert len(mine(operator))==2
    compose('up','-d','--no-build','--wait','--wait-timeout','180','rabbitmq')
    until(lambda: len(mine(operator))==4,'broker recovery')
    before=mine(operator)
    compose('restart','outbox-publisher','notification-consumer')
    compose('up','-d','--no-build','--wait','--wait-timeout','120','outbox-publisher','notification-consumer')
    assert mine(operator)==before
    assert sql('SELECT count(*) FROM inventory_transactions')=='2'
    emit('PASS broker outage approval, retained-event recovery, independent worker restart and unchanged notification IDs/read state/stock')
    before=snapshot()
    compose('up','-d','--no-build','--no-deps','--force-recreate','--wait','--wait-timeout','120','api','frontend','outbox-publisher','notification-consumer')
    assert snapshot()==before
    request('/api/v1/ready'); request('/health',frontend=True)
    emit('PASS database persistence across application container recreation: '+before)
    browser_env={**env,'E2E_BASE_URL':'http://127.0.0.1:'+env['FRONTEND_PORT'],'E2E_JWT_SECRET':env['JWT_SECRET_KEY'],'PLAYWRIGHT_OUTPUT_DIR':str(STATE/'browser'),'E2E_CONTRACT_DIR':str(STATE/'contracts'),'E2E_SCREENSHOT_DIR':str(STATE/'screenshots')}
    for role in ('OPERATOR','SUPERVISOR'):
        for field in ('EMAIL','PASSWORD'): browser_env['E2E_'+role+'_'+field]=env[role+'_'+field]
    if not browser_env.get('PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH'):
        for executable in ('/opt/google/chrome/chrome','/usr/bin/google-chrome'):
            if Path(executable).exists(): browser_env['PLAYWRIGHT_CHROMIUM_EXECUTABLE_PATH']=executable; break
    browser=run(['npm','run','test:browser','--','tests/catalogue-receiving.spec.ts','tests/receiving-transactions.spec.ts','tests/inventory-notifications.spec.ts'],cwd=ROOT/'frontend',extra=browser_env,show=True)
    assert 'skipped' not in browser
    # Production authentication uses no Vite-module introspection.
    run(['npm','run','test:browser','--','tests/auth-shell.spec.ts','--grep','anonymous protection|Operator login|Supervisor navigation'],cwd=ROOT/'frontend',extra=browser_env,show=True)
    emit('PASS real Chrome production-container frontend integration for both roles, including login/refresh/logout and mobile navigation')
    request('/api/v1/auth/logout','POST',auth=operator,frontend=True,status=204)
    request('/api/v1/notifications',auth=operator,status=401)
    records=json.loads(run(['docker','inspect',*compose('ps','-q').split()]))
    assert len(records)==7 and all(record['State']['Health']['Status']=='healthy' for record in records)
    (STATE/'responses.json').write_text(json.dumps(captures))
    emit('PASS final seven-service health, logout revocation and response captures')
    run(['python3',str(ROOT/'scripts/check-responses.py'),str(STATE)],show=True)
    (STATE/'result.json').write_text(json.dumps({'status':'passed','project':project,'api_port':env['API_PORT'],'frontend_port':env['FRONTEND_PORT'],'observed_peer':peer,'evidence':str(STATE)}))
finally:
    compose('stop',timeout=180)
    emit('Stopped only isolated '+project+'; all volumes and private evidence retained at '+str(STATE))

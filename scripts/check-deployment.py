"""Critical deployment boundary checks; no secrets or services are changed."""
import json
import os
import pathlib
import subprocess

root = pathlib.Path(__file__).resolve().parents[1]
env = {**os.environ, 'API_IMAGE': 'warehouse-api:validation', 'FRONTEND_IMAGE': 'warehouse-frontend:validation', 'POSTGRES_PASSWORD': 'validation-only', 'REDIS_PASSWORD': 'validation-only', 'RABBITMQ_PASSWORD': 'validation-only', 'JWT_SECRET_KEY': 'validation-only-key-at-least-32-bytes'}
config = json.loads(subprocess.check_output(['docker', 'compose', '--profile', 'ops', '-f', str(root / 'compose.production.yml'), 'config', '--format', 'json'], env=env))
services = config['services']
assert set(services) == {'postgres', 'redis', 'rabbitmq', 'api', 'frontend', 'migrate', 'outbox-publisher', 'notification-consumer'}
for name, service in services.items():
    assert 'nginx' not in service.get('image', '').lower()
    if name in ('frontend', 'api'):
        assert len(service['ports']) == 1 and service['ports'][0]['host_ip'] == '127.0.0.1'
    else:
        assert not service.get('ports'), name
    if name != 'migrate': assert service.get('restart') == 'unless-stopped'
    assert service.get('healthcheck'), name
    assert all(mount.startswith('/') for mount in service.get('tmpfs', [])), name
assert set(services['frontend'].get('environment', {})) == {'API_PROXY_TARGET'}
assert not services['api']['environment']['TRUSTED_PROXIES']
assert len(config['volumes']) == 3
assert services['outbox-publisher']['command'] == ['outbox-publisher']
assert services['notification-consumer']['command'] == ['notification-consumer']
workflow = (root / '.github/workflows/deploy.yml').read_text()
for secret in ('DEPLOY_HOST', 'DEPLOY_USER', 'DEPLOY_PORT', 'DEPLOY_KNOWN_HOSTS', 'DEPLOY_SSH_KEY'): assert 'secrets.' + secret + ' }}' in workflow
assert 'cancel-in-progress: false' in workflow and 'contents: read' in workflow
script = (root / 'scripts/deploy.sh').read_text()
for forbidden in ('nginx ', 'systemctl ', 'volume rm', 'down -v', 'docker system prune', 'git reset'): assert forbidden not in script
assert 'flock -n' in script and 'pg_dump' in script and 'trap rollback ERR' in script
assert not (root / 'deploy/nginx.conf').exists()
subprocess.run(['bash', '-n', str(root / 'scripts/deploy.sh')], check=True)
print('PASS: production services, loopback exposure, data persistence, secrets, SSH names, locking, backup/rollback and host-Nginx boundary.')

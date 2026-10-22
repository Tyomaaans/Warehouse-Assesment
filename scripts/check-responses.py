#!/usr/bin/env python3
"""Validate actual production-gate captures without converting int64 to floats."""
import base64
import json
from pathlib import Path
import re
import sys
import uuid
import warnings
warnings.filterwarnings('ignore', category=DeprecationWarning)
import yaml
from jsonschema import RefResolver
from openapi_schema_validator import OAS30Validator

root = Path(__file__).resolve().parents[1]
state = Path(sys.argv[1])
api = yaml.safe_load((root/'docs/openapi.yml').read_text())
resolver = RefResolver.from_schema(api)
responses = payloads = ids = 0
operations = set()
quantities = {'expected_quantity','approved_quantity','remaining_quantity','actual_quantity','quantity','quantity_delta'}
def values(value):
    global ids
    if isinstance(value, dict):
        for key, item in value.items():
            if isinstance(item, str) and (key=='id' or key.endswith('_id') or key in ('created_by','approved_by','corrected_by','reviewed_by')):
                assert re.fullmatch(r'[A-Za-z0-9_-]{21}[AQgw]', item), key
                raw=base64.urlsafe_b64decode(item+'=='); identity=uuid.UUID(bytes=raw)
                assert identity.version==4 and identity.variant==uuid.RFC_4122 and base64.urlsafe_b64encode(raw).decode().rstrip('=')==item
                ids+=1
            if key in quantities:
                assert isinstance(item, int) and not isinstance(item, bool) and 0<=item<=9223372036854775807
            values(item)
    elif isinstance(value,list):
        for item in value: values(item)
files=[state/'responses.json', *sorted((state/'contracts').glob('*.json'))]
for file in files:
    for record in json.loads(file.read_text()):
        method=record['method'].lower(); path=record['path']; status=str(record['status'])
        route=next(candidate for candidate in api['paths'] if re.fullmatch(re.sub(r'\{[^}]+\}',r'[^/]+',candidate),path) and method in api['paths'][candidate])
        operation=api['paths'][route][method]
        response=operation['responses'][status]
        if '$ref' in response: response=resolver.resolve(response['$ref'])[1]
        body=json.loads(record['body_text'])
        OAS30Validator(response['content']['application/json']['schema'],resolver=resolver,format_checker=OAS30Validator.FORMAT_CHECKER).validate(body)
        values(body); responses+=1; operations.add((method,route))
        if 'requestBody' in operation and record.get('request_text') and int(status)<400:
            payload=json.loads(record['request_text'])
            OAS30Validator(operation['requestBody']['content']['application/json']['schema'],resolver=resolver,format_checker=OAS30Validator.FORMAT_CHECKER).validate(payload)
            values(payload); payloads+=1
required={('get','/inventory'),('get','/inventory/transactions'),('get','/notifications'),('patch','/notifications/{id}/read'),('post','/transactions/{id}/approve')}
assert required<=operations
assert responses>0 and ids>0
print(f'PASS: {responses} actual responses, {payloads} payloads, {ids} canonical UUID v4 occurrences across {len(operations)} operations; exact int64 and required inventory/notification/approval contracts.')

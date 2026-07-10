#!/usr/bin/env python3
import json
import os
import yaml
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
ENV_YAML = ROOT / 'postman' / 'environments' / 'basis-data-ltt_dev.environment.yaml'
OUT_JSON = ROOT / 'postman' / 'environments' / 'basis-data-ltt_dev.postman_environment.json'

def main():
    if not ENV_YAML.exists():
        print(f"Error: {ENV_YAML} does not exist.")
        return

    with open(ENV_YAML, 'r') as f:
        data = yaml.safe_load(f)

    env_name = data.get('name', 'basis-data-ltt_dev')
    raw_values = data.get('values', [])
    
    values = []
    for val in raw_values:
        key = val.get('key')
        value = val.get('value', '')
        if key == 'baseUrl' and not value:
            # Set to HTTPS localhost since TLS is enabled on port 19091
            value = 'https://localhost:19091'
        
        values.append({
            'key': key,
            'value': value,
            'enabled': True,
            'type': 'default'
        })

    env_json = {
        'id': '01fb09a3-4c21-4482-8670-d5de97907d93',
        'name': env_name,
        'values': values,
        '_postman_variable_scope': 'environment'
    }

    with open(OUT_JSON, 'w') as f:
        json.dump(env_json, f, indent=2)

    print(f"Wrote environment configuration to {OUT_JSON}")

if __name__ == '__main__':
    main()

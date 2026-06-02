#!/usr/bin/env python3
import json
import os
import sys

try:
    import yaml
except Exception:
    print('Missing PyYAML. Install with: python3 -m pip install pyyaml', file=sys.stderr)
    raise

ROOT = os.path.dirname(os.path.dirname(__file__))
SWAGGER_PATH = os.path.join(ROOT, 'docs', 'swagger.yaml')
OUT_PATH = os.path.join(ROOT, 'postman', 'collections', 'LTT_Backend_API.postman_collection.json')


def to_postman_path_components(path):
    comps = [p for p in path.split('/') if p != '']
    out = []
    for c in comps:
        if c.startswith('{') and c.endswith('}'):
            out.append(':' + c[1:-1])
        else:
            out.append(c)
    return out


def load_spec():
    with open(SWAGGER_PATH, 'r') as f:
        return yaml.safe_load(f)


def build_collection_info(spec):
    info = spec.get('info', {})
    return {
        'name': info.get('title', 'API Collection'),
        'schema': 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json',
        'version': info.get('version', '1.0')
    }


def build_request_item(path, method, op, base_path=''):
    consumes = op.get('consumes') or []
    headers = []
    if 'application/json' in consumes:
        headers.append({'key': 'Content-Type', 'value': 'application/json'})

    full_path = (base_path or '') + path
    url_raw = '{{baseUrl}}' + full_path
    url_raw = url_raw.replace('{', ':').replace('}', '')

    item = {
        'name': op.get('summary') or f"{method.upper()} {path}",
        'request': {
            'method': method.upper(),
            'header': headers,
            'url': {
                'raw': url_raw,
                'host': ['{{baseUrl}}'],
                'path': to_postman_path_components(full_path)
            },
            'description': op.get('description', '')
        }
    }

    for param in op.get('parameters', []) or []:
        if param.get('in') == 'body':
            item['request']['body'] = {
                'mode': 'raw',
                'raw': json.dumps({'example': 'replace with valid JSON'}, indent=2)
            }
            break

    return item


def has_item_endpoints(items):
    for item in items:
        path = item.get('request', {}).get('url', {}).get('path', [])
        if 'item' in path:
            return True
    return False


def build_item_examples():
    return [
        {
            'name': 'List all items',
            'request': {
                'method': 'GET',
                'header': [{'key': 'Content-Type', 'value': 'application/json'}],
                'url': {'raw': '{{baseUrl}}/item', 'host': ['{{baseUrl}}'], 'path': ['item']},
                'description': 'Get a paginated list of items'
            }
        },
        {
            'name': 'Create a new item',
            'request': {
                'method': 'POST',
                'header': [{'key': 'Content-Type', 'value': 'application/json'}],
                'url': {'raw': '{{baseUrl}}/item', 'host': ['{{baseUrl}}'], 'path': ['item']},
                'description': 'Add a new item record',
                'body': {'mode': 'raw', 'raw': json.dumps({'name': 'Bandage', 'quantity': 100, 'price': 25000}, indent=2)}
            }
        },
        {
            'name': 'Get item information',
            'request': {
                'method': 'GET',
                'header': [{'key': 'Content-Type', 'value': 'application/json'}],
                'url': {'raw': '{{baseUrl}}/item/:id', 'host': ['{{baseUrl}}'], 'path': ['item', ':id']},
                'description': 'Retrieve an item record by ID'
            }
        },
        {
            'name': 'Update item information',
            'request': {
                'method': 'PATCH',
                'header': [{'key': 'Content-Type', 'value': 'application/json'}],
                'url': {'raw': '{{baseUrl}}/item/:id', 'host': ['{{baseUrl}}'], 'path': ['item', ':id']},
                'description': 'Update an existing item record',
                'body': {'mode': 'raw', 'raw': json.dumps({'name': 'Bandage', 'quantity': 150}, indent=2)}
            }
        },
        {
            'name': 'Delete an item',
            'request': {
                'method': 'DELETE',
                'header': [{'key': 'Content-Type', 'value': 'application/json'}],
                'url': {'raw': '{{baseUrl}}/item/:id', 'host': ['{{baseUrl}}'], 'path': ['item', ':id']},
                'description': 'Soft delete an item by ID'
            }
        }
    ]


def build_collection(spec):
    collection = {
        'info': build_collection_info(spec),
        'item': []
    }

    for path, methods in spec.get('paths', {}).items():
        for method, op in methods.items():
            collection['item'].append(build_request_item(path, method, op, spec.get('basePath', '') or ''))

    if not has_item_endpoints(collection['item']):
        collection['item'].extend(build_item_examples())

    return collection


def main():
    spec = load_spec()
    coll = build_collection(spec)

    os.makedirs(os.path.dirname(OUT_PATH), exist_ok=True)
    with open(OUT_PATH, 'w') as f:
        json.dump(coll, f, indent=2)

    print('Wrote Postman collection to', OUT_PATH)


if __name__ == '__main__':
    main()

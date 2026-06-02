#!/usr/bin/env python3
import json
import os
import yaml
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
COL_DIR = ROOT / 'postman' / 'collections' / 'LTT Backend API'
OUT = ROOT / 'postman' / 'collections' / 'LTT_Backend_API.postman_collection.json'


def build_request_url(url: str):
    return {
        'raw': url,
        'host': ['{{baseUrl}}'],
        'path': [seg for seg in url.replace('{{baseUrl}}', '').split('/') if seg != '']
    }


def build_headers(headers):
    return [{'key': h.get('key'), 'value': h.get('value')} for h in headers]


def build_query_params(query):
    return [{'key': q.get('key'), 'value': q.get('value'), 'disabled': q.get('disabled', False)} for q in query]


def build_body(body):
    return {'mode': 'raw', 'raw': body.get('content')}


def parse_request_file(p: Path):
    data = yaml.safe_load(p.read_text())
    url = data.get('url', '')
    req = {
        'name': data.get('name') or p.stem,
        'request': {
            'method': data.get('method', 'GET').upper(),
            'header': build_headers(data.get('headers') or []),
            'url': build_request_url(url),
            'description': data.get('description', '')
        },
        'order': data.get('order', 0)
    }

    query = data.get('queryParams') or []
    if query:
        # attach query params into url.raw as well (Postman supports params separately)
        req['request']['url']['query'] = build_query_params(query)

    body = data.get('body')
    if body:
        req['request']['body'] = build_body(body)

    return req


def main():
    collection = {'info': {'name': 'LTT Backend API', 'schema': 'https://schema.getpostman.com/json/collection/v2.1.0/collection.json', 'version': '1.0'}, 'item': []}

    # iterate folders under COL_DIR
    for folder in sorted([d for d in COL_DIR.iterdir() if d.is_dir()]):
        folder_name = folder.name
        items = []
        for f in sorted(folder.glob('*.request.yaml')):
            try:
                req = parse_request_file(f)
                items.append(req)
            except Exception as e:
                print('Failed to parse', f, e)

        # sort by order if present
        items.sort(key=lambda x: x.get('order', 0))
        # remove order key
        for it in items:
            it.pop('order', None)

        if items:
            collection['item'].append({'name': folder_name, 'item': items})

    OUT.parent.mkdir(parents=True, exist_ok=True)
    with open(OUT, 'w') as fh:
        json.dump(collection, fh, indent=2)

    print('Wrote collection to', OUT)


if __name__ == '__main__':
    main()

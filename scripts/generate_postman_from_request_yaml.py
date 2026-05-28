#!/usr/bin/env python3
import json
import os
import yaml
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
COL_DIR = ROOT / 'postman' / 'collections' / 'LTT Backend API'
OUT = ROOT / 'postman' / 'collections' / 'LTT_Backend_API.postman_collection.json'


def parse_request_file(p: Path):
    data = yaml.safe_load(p.read_text())
    name = data.get('name') or p.stem
    method = data.get('method', 'GET').upper()
    url = data.get('url', '')
    headers = data.get('headers') or []
    query = data.get('queryParams') or []
    body = data.get('body')
    order = data.get('order', 0)

    # build request entry
    req = {
        'name': name,
        'request': {
            'method': method,
            'header': [{'key': h.get('key'), 'value': h.get('value')} for h in headers],
            'url': {
                'raw': url,
                'host': ['{{baseUrl}}'],
                'path': [seg for seg in url.replace('{{baseUrl}}', '').split('/') if seg != '']
            },
            'description': data.get('description', '')
        },
        'order': order
    }

    if query:
        # attach query params into url.raw as well (Postman supports params separately)
        qlist = []
        for q in query:
            qlist.append({'key': q.get('key'), 'value': q.get('value'), 'disabled': q.get('disabled', False)})
        req['request']['url']['query'] = qlist

    if body:
        if body.get('type') == 'json':
            req['request']['body'] = {'mode': 'raw', 'raw': body.get('content')}
        else:
            req['request']['body'] = {'mode': 'raw', 'raw': body.get('content')}

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

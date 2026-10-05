#!/usr/bin/env python3
"""Generate a bounded DMZ rule subset from a pinned MIT Gitleaks snapshot."""
import argparse
import hashlib
import json
import tomllib
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
HERE = Path(__file__).resolve().parent
REVISION = 'b58d3f102cf3a2c84cb7f923d05c25c9b1aed84b'
CONFIG_SHA = 'e163e53b9e7e8a8511e77271e2b323ed057759542a6d988258afe3a1fa329caf'
RULES = ['digitalocean-access-token', 'digitalocean-pat', 'gitlab-pat',
         'huggingface-access-token', 'huggingface-organization-api-token', 'linear-api-key',
         'notion-api-token', 'npm-access-token', 'pypi-upload-token', 'sendgrid-api-token',
         'shopify-access-token', 'shopify-custom-access-token', 'shopify-private-app-access-token',
         'shopify-shared-secret', 'slack-bot-token', 'stripe-access-token', 'twilio-api-key']
DELIMITER = r'''(?:[\x60'"\s;]|\\[nr]|$)'''


def extract(path):
    raw = path.read_bytes()
    if hashlib.sha256(raw).hexdigest() != CONFIG_SHA:
        raise ValueError('source config does not match pinned review')
    source = tomllib.loads(raw.decode())
    selected = [r for r in source['rules'] if r['id'] in RULES]
    if sorted(r['id'] for r in selected) != sorted(RULES):
        raise ValueError('missing rule')
    for rule in selected:
        if set(rule) - {'id', 'description', 'regex', 'entropy', 'keywords'}:
            raise ValueError('unsupported rule semantics')
    allow = source['allowlist']
    if set(allow) - {'description', 'paths', 'regexes', 'stopwords'}:
        raise ValueError('unsupported global allowlist semantics')
    return {'schema': 1, 'source': 'https://github.com/gitleaks/gitleaks', 'revision': REVISION,
            'config_sha256': CONFIG_SHA, 'license': 'MIT', 'rules': selected,
            'secret_allow_regexes': allow['regexes'], 'secret_allow_stopwords': allow['stopwords']}


def generate(source):
    if source['revision'] != REVISION or source['config_sha256'] != CONFIG_SHA or sorted(r['id'] for r in source['rules']) != sorted(RULES):
        raise ValueError('wrong source snapshot')
    rules = []
    for rule in source['rules']:
        pattern = rule['regex']
        if pattern.endswith(DELIMITER):
            pattern = pattern[:-len(DELIMITER)]
        rules.append({'id': rule['id'], 'pattern': pattern, 'entropy': rule.get('entropy', 0), 'keywords': rule['keywords']})
    return {'schema': 1, 'source_revision': REVISION, 'source_config_sha256': CONFIG_SHA,
            'adaptation': 'Selected context-free rules; source-code trailing delimiter replaced with DMZ whole-token boundary checks.',
            'rules': rules, 'allow_regexes': source['secret_allow_regexes'], 'allow_stopwords': source['secret_allow_stopwords']}


def serialized(value):
    return json.dumps(value, indent=2, ensure_ascii=False) + '\n'


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--extract', type=Path, help='One-time extraction from the exact upstream config')
    p.add_argument('--check', action='store_true')
    a = p.parse_args()
    snapshot = HERE / 'source.json'
    if a.extract:
        if a.check:
            raise ValueError('extract and check are separate operations')
        with snapshot.open('x') as f:
            f.write(serialized(extract(a.extract)))
    source = json.loads(snapshot.read_text())
    output = serialized(generate(source))
    target = ROOT / 'packages/detect/vendor-secrets.json'
    if a.check:
        if target.read_text() != output:
            raise ValueError('generated vendor rules differ')
    else:
        target.write_text(output)
    print(json.dumps({'rules': len(source['rules']), 'source_config_sha256': CONFIG_SHA, 'generated_sha256': hashlib.sha256(output.encode()).hexdigest()}))


if __name__ == '__main__':
    main()

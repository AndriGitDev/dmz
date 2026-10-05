"""Freeze fabricated vendor-token probes before specialist integration changes."""
import hashlib
import json
import random
import string
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'bench/corpus/vendor-secrets-v1'
rng = random.Random(202610051)


def body(n, alphabet=string.ascii_letters + string.digits):
    return ''.join(rng.choice(alphabet) for _ in range(n))


def main():
    if OUT.exists():
        raise ValueError('frozen corpus already exists')
    hexadecimal = string.digits + 'abcdef'
    fixtures = [
        ('npm', 'npm_' + body(36)),
        ('pypi', 'pypi-AgEIcHlwaS5vcmc' + body(70)),
        ('huggingface', 'hf_' + body(34, string.ascii_letters)),
        ('huggingface-org', 'api_org_' + body(34, string.ascii_letters)),
        ('gitlab', 'glpat-' + body(20)),
        ('linear', 'lin_api_' + body(40)),
        ('notion', 'ntn_' + body(11, string.digits) + body(35)),
        ('sendgrid', 'SG.' + body(22) + '.' + body(43)),
        ('shopify', 'shpat_' + body(32, hexadecimal)),
        ('shopify-custom', 'shpca_' + body(32, hexadecimal)),
        ('shopify-private', 'shppa_' + body(32, hexadecimal)),
        ('shopify-shared', 'shpss_' + body(32, hexadecimal)),
        ('slack', 'xoxb-' + body(12, string.digits) + '-' + body(12, string.digits) + '-' + body(24)),
        ('stripe-live', 'sk_live_' + body(30)),
        ('stripe-test', 'rk_test_' + body(30)),
        ('digitalocean-pat', 'dop_v1_' + body(64, hexadecimal)),
        ('digitalocean-access', 'doo_v1_' + body(64, hexadecimal)),
        ('twilio', 'SK' + body(32, hexadecimal)),
    ]
    rows = []
    for family, token in fixtures:
        for context, template in [
            ('bare', '{}'),
            ('english', 'Please rotate {} before the maintenance window.'),
            ('assignment', 'service_token = "{}"'),
            ('mixed', 'Nýr lykill: {} — rotate it before deployment.'),
        ]:
            rows.append({'id': f'{family}-{context}', 'stratum': context, 'family': family,
                         'text': template.format(token), 'entities': [{'kind': 'secret', 'value': token}]})
        # A token-shaped substring embedded inside an ordinary identifier is a
        # deliberate boundary diagnostic, not an issued credential in this fixture.
        rows.append({'id': family + '-identifier', 'stratum': 'negative-boundary', 'family': family,
                     'text': 'The generated symbol is widget_' + token + '_counter.', 'entities': []})
    benign = [
        'Run npm install after reviewing the package manifest.',
        'The documented prefixes are npm_, hf_, glpat-, SG., xoxb- and shpat_.',
        'Use a publishable browser key: pk_live_' + body(30),
        'Public test key: pk_test_' + body(30),
        'The SHA-256 digest is ' + body(64, hexadecimal),
        'The build identifier is ' + body(40, hexadecimal),
        'secret_count = 37', 'token_budget = 32000', 'service_token = "${TOKEN}"',
        'service_token = "REPLACE_ME"', 'client_id = "public-browser-app"',
        'No credentials are included in this maintenance note.',
    ]
    rows.extend({'id': f'benign-{i:02}', 'stratum': 'negative-prose', 'family': 'benign',
                 'text': text, 'entities': []} for i, text in enumerate(benign))
    OUT.mkdir()
    manifest = {'schema': 1, 'description': 'Deterministically fabricated, never-issued token-shaped values; no service verification.',
                'independent_validation': False, 'scope': ['name_demo', 'address', 'phone', 'email', 'iban', 'card', 'kennitala', 'secret', 'tenant_dictionary'],
                'sets': {}}
    for stratum in sorted({r['stratum'] for r in rows}):
        selected = [r for r in rows if r['stratum'] == stratum]
        name = stratum + '.jsonl'
        raw = ''.join(json.dumps(r, ensure_ascii=False) + '\n' for r in selected).encode()
        (OUT / name).write_bytes(raw)
        manifest['sets'][name] = {'sha256': hashlib.sha256(raw).hexdigest(), 'excerpts': len(selected),
                                  'entities': sum(len(r['entities']) for r in selected)}
    (OUT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({'excerpts': len(rows), 'positive_excerpts': sum(bool(r['entities']) for r in rows), 'families': len(fixtures)}))


if __name__ == '__main__':
    main()

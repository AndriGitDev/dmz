"""Fresh fabricated values/contexts after vendor rule and boundary selection."""
import hashlib
import json
import random
import string
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / 'bench/corpus/vendor-secrets-followup-v1'


def main():
    if OUT.exists():
        raise ValueError('frozen follow-up already exists')
    rng = random.Random(202610052)
    def random_value(n, alphabet=string.ascii_letters + string.digits):
        return ''.join(rng.choice(alphabet) for _ in range(n))
    hexdigits = string.digits + 'abcdef'
    specs = [
        ('npm', 'npm_', [(36, None)]), ('pypi', 'pypi-AgEIcHlwaS5vcmc', [(83, None)]),
        ('huggingface', 'hf_', [(34, string.ascii_letters)]),
        ('huggingface-org', 'api_org_', [(34, string.ascii_letters)]),
        ('gitlab', 'glpat-', [(20, None)]), ('linear', 'lin_api_', [(40, None)]),
        ('notion', 'ntn_', [(11, string.digits), (35, None)]),
        ('sendgrid', 'SG.', [(22, None), '.', (43, None)]),
        ('shopify', 'shpat_', [(32, hexdigits)]), ('shopify-custom', 'shpca_', [(32, hexdigits)]),
        ('shopify-private', 'shppa_', [(32, hexdigits)]), ('shopify-shared', 'shpss_', [(32, hexdigits)]),
        ('slack', 'xoxb-', [(11, string.digits), '-', (13, string.digits), '-', (24, None)]),
        ('stripe-live', 'sk_live_', [(45, None)]), ('stripe-test', 'rk_test_', [(26, None)]),
        ('digitalocean-pat', 'dop_v1_', [(64, hexdigits)]),
        ('digitalocean-access', 'doo_v1_', [(64, hexdigits)]), ('twilio', 'SK', [(32, hexdigits)]),
    ]
    rows = []
    for family, prefix, parts in specs:
        value = prefix + ''.join(p if isinstance(p, str) else random_value(p[0], p[1] or string.ascii_letters + string.digits) for p in parts)
        low = prefix + ''.join(p if isinstance(p, str) else ('1' if p[1] == string.digits else 'a') * p[0] for p in parts)
        for stratum, text, token in [
            ('english-punctuation', 'Replace (' + value + '), then restart the service.', value),
            ('mixed-unicode', 'A\u0301ætlun 🙂\nEndurnýja ' + value + '; do this before lunch.', value),
            ('quoted-assignment', 'settings:\r\n  service_token: "' + value + '"\r\n  retries: 4', value),
            ('path-limit', 'The credential is embedded here: https://example.invalid/v1/' + value, value),
            ('low-entropy-limit', 'This service credential still needs rotation: ' + low, low),
        ]:
            rows.append({'id': family + '-' + stratum, 'stratum': stratum, 'family': family,
                         'text': text, 'entities': [{'kind': 'secret', 'value': token}]})
        for stratum, text in [
            ('negative-boundary', 'Generated identifier: át_' + value + '_stærð'),
            ('negative-short', 'The abbreviated prefix example is "' + prefix + 'a3' + '".'),
        ]:
            rows.append({'id': family + '-' + stratum, 'stratum': stratum, 'family': family,
                         'text': text, 'entities': []})
    benign = [
        'publishable_key = "pk_live_' + random_value(32) + '"',
        'The file checksum is ' + random_value(64, hexdigits),
        'NPM package names and versions are public metadata.',
        'Hugging Face model identifiers are not access tokens.',
        'Rotate the deployment credential without pasting it here.',
        'A\u0301ætlun fyrir uppfærslu — no credential values are present.',
        'client_secret = "${DEPLOYMENT_SECRET}"',
        'service_token: "{{ secrets.DEPLOYMENT_TOKEN }}"',
    ]
    rows.extend({'id': f'benign-{i:02}', 'stratum': 'negative-prose', 'family': 'benign',
                 'text': text, 'entities': []} for i, text in enumerate(benign))
    OUT.mkdir()
    manifest = {'schema': 1, 'description': 'Fresh seeded fabricated vendor tokens; includes declared entropy/path limitations. No issued credentials or verification.',
                'selection_commit': 'cb1f9cd', 'independent_validation': False,
                'scope': ['name_demo', 'address', 'phone', 'email', 'iban', 'card', 'kennitala', 'secret', 'tenant_dictionary'], 'sets': {}}
    for stratum in sorted({r['stratum'] for r in rows}):
        selected = [r for r in rows if r['stratum'] == stratum]
        name = stratum + '.jsonl'
        raw = ''.join(json.dumps(r, ensure_ascii=False) + '\n' for r in selected).encode()
        (OUT / name).write_bytes(raw)
        manifest['sets'][name] = {'sha256': hashlib.sha256(raw).hexdigest(), 'excerpts': len(selected),
                                 'entities': sum(len(r['entities']) for r in selected)}
    (OUT / 'manifest.json').write_text(json.dumps(manifest, indent=2) + '\n')
    print(json.dumps({'excerpts': len(rows), 'positive_excerpts': sum(bool(r['entities']) for r in rows)}))


if __name__ == '__main__':
    main()

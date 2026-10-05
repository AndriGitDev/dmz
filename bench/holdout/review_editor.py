"""Self-contained, network-disabled human annotation page for a private packet."""
import base64
import hashlib
import json
from pathlib import Path
import re

EDITOR = Path(__file__).with_name('editor')


def digest(data):
    return hashlib.sha256(data).hexdigest()


def render_editor(packet, slot, manifest, index):
    def text(name):
        return (packet/name).read_bytes().decode('utf-8')
    form = text(f'reviewer-{slot}.jsonl')
    payload = dict(schema=1, slot=slot, scope=manifest['scope'],
                   packet_sha256=digest((packet/'manifest.json').read_bytes()),
                   index_jsonl=text('index.jsonl'), index_sha256=manifest['index_sha256'],
                   form_jsonl=form, form_sha256=digest(form.encode()),
                   rubric=text('rubric.md'), rubric_sha256=manifest['rubric_sha256'],
                   terms_json=text('tenant-terms.json'), terms_sha256=manifest['tenant_terms_sha256'],
                   texts={identifier: data.decode('utf-8') for identifier, (_, data) in index.items()})
    template = (EDITOR/'review.html').read_text()
    script = (EDITOR/'core.js').read_text()+'\n'+(EDITOR/'app.js').read_text()
    if '</script' in script.lower():
        raise ValueError('unsafe editor source delimiter')
    style = re.search(r'<style>(.*?)</style>', template, re.S).group(1)
    checksum = lambda value: base64.b64encode(hashlib.sha256(value.encode()).digest()).decode()
    csp = ("default-src 'none'; connect-src 'none'; object-src 'none'; base-uri 'none'; "
           "form-action 'none'; frame-src 'none'; "
           f"script-src 'sha256-{checksum(script)}'; style-src 'sha256-{checksum(style)}'")
    # Prevent source text from closing the inert JSON script element.
    encoded = json.dumps(payload, ensure_ascii=True).replace('<', '\\u003c').replace('>', '\\u003e').replace('&', '\\u0026')
    return template.replace('__CSP__', csp).replace('__EDITOR_JS__', script).replace('__PACKET_JSON__', encoded).encode()

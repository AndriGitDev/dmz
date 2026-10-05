"""Offline stdin secret-specialist probe; raw reports never leave memory."""
import argparse
import hashlib
import json
import subprocess
import tempfile
import time
from pathlib import Path
from evaluate import digest, load_corpus, private_output
from quality import score

REVISION = 'b58d3f102cf3a2c84cb7f923d05c25c9b1aed84b'


def original_span(text, finding):
    data = text.encode()
    newlines = [i for i, byte in enumerate(data) if byte == 10]
    def offset(line, column, end=False):
        if type(line) is not int or type(column) is not int or not 1 <= line <= len(newlines) + 1 or column < 1:
            raise ValueError('invalid reported location')
        prefix = 0 if line == 1 else newlines[line - 2]
        return prefix + column - (0 if end else 1)
    start = offset(finding['StartLine'], finding['StartColumn'])
    end = offset(finding['EndLine'], finding['EndColumn'], True)
    matched, secret = finding['Match'].encode(), finding['Secret'].encode()
    if not 0 <= start < end <= len(data) or data[start:end] != matched or not secret or matched.count(secret) != 1:
        raise ValueError('ambiguous or changed reported match')
    start += matched.index(secret)
    end = start + len(secret)
    data[:start].decode(); data[start:end].decode(); data[end:].decode()
    return [start, end, 'secret']


def main():
    p = argparse.ArgumentParser()
    p.add_argument('--binary', type=Path, required=True)
    p.add_argument('--source', type=Path, required=True)
    p.add_argument('--corpus', type=Path, required=True)
    p.add_argument('--output', required=True)
    a = p.parse_args()
    out = private_output(a.output)
    if subprocess.check_output(['git', '-C', str(a.source), 'rev-parse', 'HEAD'], text=True).strip() != REVISION:
        raise ValueError('wrong source revision')
    if subprocess.check_output(['git', '-C', str(a.source), 'status', '--porcelain'], text=True).strip():
        raise ValueError('modified source checkout')
    config = a.source / 'config/gitleaks.toml'
    identities = {str(path): digest(path) for path in [a.binary, config, Path(__file__)]}
    rows, manifest = load_corpus(a.corpus)
    records, failures, rules = [], {}, {}
    began = time.monotonic()
    command = [str(a.binary), 'stdin', '--config', str(config), '--no-banner', '--log-level', 'fatal',
               '--report-format', 'json', '--report-path', '-', '--exit-code', '0',
               '--ignore-gitleaks-allow', '--gitleaks-ignore-path', '/dev/null',
               '--max-decode-depth', '0', '--max-archive-depth', '0']
    with tempfile.TemporaryDirectory(prefix='gitleaks-probe-', dir=out.parent) as working:
        for row in rows:
            predicted, scored, failure = [], [], None
            start = time.monotonic()
            try:
                if len(row['text'].encode()) > 32768:
                    raise ValueError('input exceeds original-span probe bound')
                response = subprocess.run(command, input=row['text'], text=True, capture_output=True,
                                          timeout=10, cwd=working,
                                          env={'PATH': '/usr/bin:/bin', 'LANG': 'C.UTF-8', 'GOMAXPROCS': '2'})
                if response.returncode:
                    raise ValueError('secret scanner failed')
                findings = json.loads(response.stdout)
                for finding in findings:
                    span = original_span(row['text'], finding)
                    rules[finding['RuleID']] = rules.get(finding['RuleID'], 0) + 1
                    if span not in predicted:
                        predicted.append(span)
                        scored.append({'span': span, 'score': 1.0, 'rule': finding['RuleID']})
            except Exception as error:
                failure = type(error).__name__
                failures[failure] = failures.get(failure, 0) + 1
                predicted, scored = [], []
            records.append({'id': row['id'], 'stratum': row['stratum'], 'gold': row['gold'],
                            'text_sha256': hashlib.sha256(row['text'].encode()).hexdigest(),
                            'predicted': predicted, 'scored': scored, 'abstained': failure is not None,
                            'candidate_failure': failure, 'seconds': time.monotonic() - start})
    for path, sha in identities.items():
        if digest(path) != sha:
            raise ValueError('measurement asset changed')
    secret_only = [r | {'gold': [s for s in r['gold'] if s[2] == 'secret']} for r in records]
    result = {'schema': 1, 'status': 'complete', 'candidate': 'gitleaks', 'revision': REVISION,
              'config': {'recursive_decoding': False, 'archives': False, 'ignore_allow_comments': True,
                         'score': '1.0 is an ordering placeholder, not confidence'},
              'corpus': manifest, 'measurement_sha256': identities, 'records': records,
              'seconds': time.monotonic() - began, 'candidate_failures': failures, 'rule_counts': rules,
              'quality_secret_only': score(secret_only, ['secret']), 'independent_validation': False,
              'network': 'native stdin scan; no verification or diagnostics enabled; synthetic inputs only'}
    with out.open('x') as f:
        json.dump(result, f, indent=2)
    print(json.dumps({'excerpts': len(records), 'candidate_failures': failures, 'seconds': result['seconds'],
                      'secret_quality': result['quality_secret_only']['overall']}))


if __name__ == '__main__':
    main()

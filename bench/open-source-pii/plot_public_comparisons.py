"""Render the published aggregate comparisons; never load corpus or row caches."""
import hashlib
import json
from pathlib import Path

import matplotlib
matplotlib.use('Agg')
import matplotlib.pyplot as plt
from matplotlib.lines import Line2D
from matplotlib.patches import Patch


ROOT = Path(__file__).resolve().parents[2]
DIRECTORY = ROOT / 'bench/results/open-source-pii-v1'


def digest(path):
    return hashlib.sha256(path.read_bytes()).hexdigest()


def main():
    if matplotlib.__version__ != '3.10.7':
        raise ValueError('Use the recorded matplotlib version')
    inputs = {}
    labels = ['Original trained core', 'Updated trained core', '+ Presidio',
              '+ BERT Person', '+ GLiNER', '+ GLiNER2', '+ GLiNER2 address-only',
              '+ usaddress', '+ Privacy Filter']
    keys = ['frozen-trained', 'presidio-all-plus-trained', 'bert-all-plus-trained',
            'gliner-all-plus-trained', 'gliner2-all-plus-trained',
            'gliner2-address-only-plus-trained', 'usaddress-standalone-all-plus-trained',
            'opf-all-plus-trained']
    plt.rcParams.update({'font.size': 10, 'svg.hashsalt': 'dmz-public-pii-comparisons-v1'})
    fig, axes = plt.subplots(1, 2, figsize=(13, 8), sharey=True)
    for ax, size, count in zip(axes, ['full', 'sample'], [4996, 512]):
        datasets = {}
        for baseline in ['original', 'current']:
            path = DIRECTORY / f'public-{baseline}-core-{size}-models.json'
            inputs[path.name] = digest(path)
            data = json.loads(path.read_text())
            assert data['evaluated_excerpts'] == count and not data['independent_validation']
            datasets[baseline] = data['runs']
        rows = [datasets['original']['frozen-trained']['overall']]
        rows += [datasets['current'][key]['overall'] if key in datasets['current'] else None for key in keys]
        for y, values in enumerate(rows):
            if values is None:
                ax.text(3, y, 'Evaluated on the 512-excerpt sample only', color='#666666', fontsize=9)
                continue
            assert values['excerpts'] == count
            assert abs(values['accuracy'] - values['exact_excerpts'] / count) < 1e-12
            assert abs(values['recall'] - values['tp'] / (values['tp'] + values['fn'])) < 1e-12
            for offset, metric, color in [(-.17, 'accuracy', '#0072B2'), (.17, 'recall', '#D55E00')]:
                value = values[metric] * 100
                ax.barh(y + offset, value, height=.29, color=color)
                ax.text(value + 1, y + offset, f'{value:.2f}%', va='center', fontsize=8)
        ax.set_title(f'{count:,} excerpts' + (' — complete test projection' if size == 'full' else ' — identical selected IDs'), fontsize=11)
        ax.set_xlim(0, 110)
        ax.set_xticks([0, 20, 40, 60, 80, 100])
        ax.set_xlabel('Percent')
        ax.set_yticks(range(len(labels)), labels)
        ax.set_ylim(len(labels) - .5, -.5)
        ax.axvline(99, color='#555555', linestyle='--', linewidth=1)
        ax.grid(axis='x', color='#e5e5e5', linewidth=.6)
        ax.set_axisbelow(True)
        for side in ['top', 'right', 'left']:
            ax.spines[side].set_visible(False)
        ax.tick_params(axis='y', length=0)
    fig.suptitle('Public English: exact entity sets and recall', fontsize=17, y=.985)
    fig.legend(handles=[Patch(color='#0072B2', label='Exact entity-set accuracy per excerpt'),
                        Patch(color='#D55E00', label='Entity recall'),
                        Line2D([0], [0], color='#555555', linestyle='--', label='99% acceptance target')],
               loc='upper center', bbox_to_anchor=(.5, .95), ncol=3, frameon=False)
    fig.text(.02, .018,
             'Additions retain existing core/name matches. Updated-core scores use unchanged cached name predictions.\n'
             'Public synthetic diagnostics: annotation completeness and training overlap are unknown; these are not independent acceptance results.',
             fontsize=9, color='#555555')
    fig.tight_layout(rect=[0, .07, 1, .9])
    files = []
    for suffix in ['svg', 'png']:
        path = DIRECTORY / f'public-model-tradeoffs.{suffix}'
        if path.exists():
            raise ValueError('Figure already exists')
        fig.savefig(path, dpi=160, metadata={'Date': None} if suffix == 'svg' else {})
        files.append(path)
    plt.close(fig)
    report = {'schema': 1, 'independent_validation': False, 'matplotlib': matplotlib.__version__,
              'input_sha256': inputs, 'measurement_sha256': digest(Path(__file__)),
              'output_sha256': {p.name: digest(p) for p in files}}
    with (DIRECTORY / 'public-model-tradeoffs.json').open('x') as stream:
        json.dump(report, stream, indent=2)
        stream.write('\n')
    print('Rendered two separate, denominator-checked aggregate comparison panels')


if __name__ == '__main__':
    main()

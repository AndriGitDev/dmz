#!/usr/bin/env python3
"""One fixed FP32 parameter average; no data, labels, tuning or compression."""
import argparse
import json
from pathlib import Path
from encoder import digest
from ner import external


def average(run, output, epochs=(1,2)):
    import torch
    from safetensors.torch import load_file,save_file
    run=external(run);output=external(output)
    if len(epochs)<2 or len(set(epochs))!=len(epochs):raise ValueError('need distinct checkpoints')
    training=json.loads((run/'training.json').read_text());sources=[]
    for epoch in epochs:
        matching=[c for c in training['checkpoints'] if c['epoch']==epoch]
        if len(matching)!=1 or matching[0]['complete_epoch'] is not True:
            raise ValueError('average requires complete epochs')
        path=run/matching[0]['file']
        if path.parent!=run or digest(path)!=matching[0]['sha256']:raise ValueError('checkpoint integrity mismatch')
        sources.append(path)
    output.mkdir(parents=True,exist_ok=False)
    torch.set_num_threads(1)
    mean=None
    for path in sources:
        state=load_file(str(path))
        if any(value.dtype!=torch.float32 for value in state.values()):raise ValueError('FP32 parameters required')
        if mean is None:mean={key:value.clone().div_(len(sources)) for key,value in state.items()}
        else:
            if set(state)!=set(mean):raise ValueError('parameter keys differ')
            for key,value in state.items():
                if value.shape!=mean[key].shape:raise ValueError('parameter shape differs')
                mean[key].add_(value,alpha=1/len(sources))
        del state
    if any(not value.isfinite().all() for value in mean.values()):raise ValueError('nonfinite parameter average')
    path=output/'averaged.safetensors';save_file(mean,str(path))
    report=dict(schema=1,method='Fixed equal FP32 parameter average, no new training or threshold tuning',
        base_weights_sha256=training['base_weights_sha256'],input_sha256=training['input_sha256'],
        split='train',source_training_sha256=digest(run/'training.json'),source_epochs=list(epochs),
        source_sha256=[digest(p) for p in sources],independent_final_evaluation=False,
        checkpoints=[dict(epoch=1,kind='parameter_average',complete_epoch=False,file=path.name,
            sha256=digest(path),bytes=path.stat().st_size)])
    (output/'training.json').write_text(json.dumps(report,indent=2)+'\n')
    return report


if __name__=='__main__':
    p=argparse.ArgumentParser(__doc__);p.add_argument('--run',type=Path,required=True);p.add_argument('--output',type=Path,required=True)
    a=p.parse_args();print(json.dumps(average(a.run,a.output),indent=2))

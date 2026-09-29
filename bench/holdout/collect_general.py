#!/usr/bin/env python3
"""Small model-blind general-prose annotation pilot. Raw HTML/text stay private."""
import argparse
from datetime import datetime, timezone
from html.parser import HTMLParser
import json
from pathlib import Path
import re
from urllib.request import Request, urlopen
from collect import sha, shingles
from review import external


class Node:
    def __init__(self, tag, attrs=()):
        self.tag=tag;self.attrs=dict(attrs);self.children=[]

    def text(self):
        if self.tag in {'script','style','nav','noscript','iframe','footer'}:return ''
        value=''.join(child.text() if isinstance(child,Node) else child for child in self.children)
        if self.tag in {'p','div','article','main','h1','h2','h3','li','tr','td','figcaption','br'}:
            value='\n'+value+'\n'
        return value


class Tree(HTMLParser):
    def __init__(self):
        super().__init__();self.root=Node('root');self.stack=[self.root];self.nodes=[]

    def handle_starttag(self,tag,attrs):
        node=Node(tag,attrs);self.stack[-1].children.append(node);self.nodes.append(node)
        if tag not in {'area','base','br','col','embed','hr','img','input','link','meta','param','source','track','wbr'}:
            self.stack.append(node)

    def handle_endtag(self,tag):
        for i in range(len(self.stack)-1,0,-1):
            if self.stack[i].tag==tag:self.stack=self.stack[:i];break

    def handle_data(self,data):self.stack[-1].children.append(data)


def extract(data):
    tree=Tree();tree.feed(data.decode('utf-8'))
    for label,predicate in [
        ('article',lambda n:n.tag=='article'),
        ('div.article',lambda n:n.tag=='div' and 'article' in n.attrs.get('class','').split()),
        ('main',lambda n:n.tag=='main')]:
        candidates=[n for n in tree.nodes if predicate(n)]
        if candidates:
            text=max((n.text() for n in candidates),key=len)
            text='\n'.join(re.sub(r'\s+',' ',line).strip() for line in text.splitlines() if line.strip())
            if len(text)>=200:return text,label
    raise ValueError('No bounded article extraction found; manual extraction required')


def main():
    p=argparse.ArgumentParser(__doc__)
    p.add_argument('--sources',type=Path,default=Path(__file__).with_name('general-sources-v1.json'))
    p.add_argument('--mim',type=Path,required=True);p.add_argument('--output',type=Path,required=True)
    a=p.parse_args();output=external(a.output);output.mkdir(parents=True,exist_ok=False)
    known=set();mim_hashes={}
    for split in ['train','dev','test']:
        source=external(a.mim)/f'{split}.jsonl';mim_hashes[split]=sha(source.read_bytes())
        for line in source.open():known.update(shingles(json.loads(line)['text']))
    results=[];count=0
    with (output/'review.jsonl').open('x') as review:
        for source in json.loads(a.sources.read_text()):
            entry=dict(source)
            if not re.fullmatch(r'[a-z0-9-]+',source['id']) or not source['url'].startswith('https://'):
                raise ValueError('invalid source identifier/URL')
            try:
                request=Request(source['url'],headers={'User-Agent':'DMZ-evaluation-research/0.1','Accept':'text/html'})
                with urlopen(request,timeout=45) as response:
                    data=response.read(2_000_001);entry['resolved_url']=response.url
                if len(data)>2_000_000:raise ValueError('HTML exceeds size bound')
                (output/(source['id']+'.html')).write_bytes(data)
                text,selector=extract(data);overlap=len(shingles(text)&known)
                (output/(source['id']+'.txt')).write_text(text)
                review.write(json.dumps(dict(document_id=source['id'],page=1,text=text,text_sha256=sha(text.encode()),
                    entities=None,reviewed=False,overlapping_mim_12grams=overlap,eligible_for_final=False),ensure_ascii=False)+'\n')
                entry.update(html_sha256=sha(data),text_sha256=sha(text.encode()),bytes=len(data),characters=len(text),
                    extraction_selector=selector,overlapping_mim_12grams=overlap,extraction_reviewed=False)
                count+=1
            except Exception as error:
                entry['error']=type(error).__name__
                if hasattr(error,'code'):entry['http_status']=error.code
            results.append(entry)
    report=dict(schema=1,collected_at=datetime.now(timezone.utc).isoformat(),state='unlabelled general-prose collection pilot; not independent evaluation',
        sampling='first three article links on each of two retrieved news-index snapshots, frozen before detector inference; search/index availability is not representative sampling',
        sources_sha256=sha(a.sources.read_bytes()),documents=results,review_documents=count,mim_hashes=mim_hashes,
        source_use_review='pending; no redistribution or training use approved by this collector',
        overlap='normalized exact 12-grams against all prepared MIM splits; no claim of semantic or pretraining independence',
        human_review_complete=False,model_evaluated=False)
    (output/'manifest.json').write_text(json.dumps(report,indent=2)+'\n');print(json.dumps(report,indent=2))


if __name__=='__main__':main()

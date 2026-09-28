#!/usr/bin/env python3
"""Generate fabricated, review-pending legal/financial development documents."""
import argparse
from collections import Counter
import hashlib
import json
from pathlib import Path
import re

# Fabricated combinations; forms are authored here, not extracted from a lexicon.
TRAIN_NAMES = [
 ('Ása Prófsteinsdóttir','Ásu Prófsteinsdóttur'),
 ('Elín Gervisteinsdóttir','Elínar Gervisteinsdóttur'),
 ('Þóra Sýnisdóttir','Þóru Sýnisdóttur'),
 ('Einar Gervisýnisson','Einars Gervisýnissonar'),
 ('Ólafur Prófmarsson','Ólafs Prófmarssonar'),
 ('Kári Sýnimarsson','Kára Sýnimarssonar'),
 ('Lilja Prófsteinsdóttir','Lilju Prófsteinsdóttur'),
 ('Bjarni Gervisteinsson','Bjarna Gervisteinssonar'),
 ('Sólrún Sýnimarsdóttir','Sólrúnar Sýnimarsdóttur'),
 ('Steinn Prófmarsson','Steins Prófmarssonar'),
 ('Mira Sampleton','Mira Sampleton'),
 ('Alex Demo','Alex Demo'),
]
DEV_NAMES = [
 ('Hildur Sýnisteinsdóttir','Hildar Sýnisteinsdóttur'),
 ('Rósa Prófvaldsdóttir','Rósu Prófvaldsdóttur'),
 ('Björn Gervivaldsson','Björns Gervivaldssonar'),
 ('Valur Prófvaldsson','Vals Prófvaldssonar'),
 ('Þórdís Sýnivaldsdóttir','Þórdísar Sýnivaldsdóttur'),
 ('Margrét Gervimarsdóttir','Margrétar Gervimarsdóttur'),
 ('Jamie Testford','Jamie Testford'),
 ('Robin Exampleton','Robin Exampleton'),
]
# Template families are split before generation. Dev families never augment train.
TRAIN_TEMPLATES = {
 'employment': 'Ráðningarsamningur. {{a}} hefur störf hjá {{org}} samkvæmt þessum samningi. Starfsmaður samþykkir starfslýsingu og trúnaðarskyldu. Undirskrift starfsmanns: {{a}}. Undirskrift fyrir hönd vinnuveitanda: {{b}}.',
 'lease': 'Leigusamningur. Leigutaki er {{a}} og leigusali er {{b}}. Aðilar semja um afnot húsnæðis í tólf mánuði. Trygging verður skráð sérstaklega. Samningur þessi felur ekki í sér heimild til framleigu.',
 'power': 'Umboð. Ég, {{a}}, veiti {{b}} heimild til að taka við afritum gagna fyrir mína hönd. Umboðið gildir aðeins vegna þessa tilbúna máls. Undirskrift: {{a}}.',
 'claim': 'Kröfulýsing. Krafa {{ag}} er byggð á samningi sem fylgir þessari lýsingu. Skuldari er {{b}}. Krafist er greiðslu höfuðstóls en ekki dráttarvaxta í þessu dæmi. Kröfuhafi: {{a}}.',
 'invoice': 'Reikningur. Greiðandi: {{a}}. Seljandi: {{org}}. Lýsing: ráðgjöf í tilraunaverkefni. Upphæð: 45.000 kr. Greiðsla {{ag}} verður færð á viðkomandi viðskiptareikning.',
 'loan': 'Lánssamningur. Lántaki er {{a}}. Ábyrgðarmaður er {{b}}. Höfuðstóll skal endurgreiddur í jöfnum greiðslum. Aðilar hafa kynnt sér greiðsluáætlun. Undirskrift lántaka: {{a}}.',
 'payroll': 'Launayfirlit. Nafn starfsmanns: {{a}}. Vinnuveitandi: {{org}}. Laun {{ag}} eru reiknuð fyrir einn mánuð. Orlof og lífeyrissjóðsiðgjald koma fram í sérstöku yfirliti. Þetta eru tilbúnar prófunarupplýsingar.',
 'board': 'Fundargerð stjórnar {{org}}. Mætt voru {{a}} og {{b}}. Fundurinn samþykkti fjárhagsáætlun með fyrirvara um lokayfirferð. Fundarritari: {{a}}.',
 'audit': 'Endurskoðunarminnisblað. Verkefnisstjóri: {{a}}. Við yfirferð {{ag}} komu fram tvær færslur sem þarf að skýra. {{b}} staðfestir að fylgiskjöl verði afhent síðar. Engin niðurstaða um reikningsskil er gefin hér.',
 'witness': 'Vitnaskýrsla. Vitnið {{a}} staðfestir að hafa verið viðstatt undirritun. Framburður {{ag}} er skráður samkvæmt frásögn. Skýrslutaki: {{b}}. Dagsetning verður skráð við yfirferð.',
 'estate': 'Dánarbú {{ag}}. Umsjónarmaður: {{b}}. Skrá yfir eignir og skuldir fylgir sem viðauki. Beiðni erfingja verður tekin fyrir sérstaklega. Þetta er tilbúið dæmi til prófunar.',
 'settlement': 'Sáttargerð. {{a}} og {{b}} samþykkja að ljúka ágreiningi með greiðslu samkvæmt viðauka. Með undirritun staðfesta aðilar að hafa lesið skjalið. Sáttin tekur aðeins til tilgreindra krafna.',
 'english_contract': 'Service agreement. The client is {{a}} and the supplier is {{org}}. The authorized contact is {{b}}. Changes require written approval from both parties. Signed by the client: {{a}}.',
 'english_finance': 'Payment review. Account holder: {{a}}. Reviewer: {{b}}. The payment remains pending until supporting documents have been checked. This is a fabricated training example.',
 'negative_accounting': 'Bókhaldsminnisblað. {{org}} færir vexti og gengismun samkvæmt fylgiskjölum. Gjöld, eignir og skuldir skulu stemma við aðalbók. Enginn einstaklingur er tilgreindur. Janúar og febrúar eru uppgjörstímabil.',
 'negative_legal': 'Samningsákvæði. Félagið {{org}} ber ábyrgð á afhendingu samkvæmt samningi. Hugtökin kaupandi og seljandi vísa til lögaðila. Réttindi, skyldur og gildistími eru skilgreind í viðauka.',
}
DEV_TEMPLATES = {
 'insurance': 'Tilkynning um tjón. Vátryggður: {{a}}. Málsfulltrúi: {{b}}. Yfirlit {{ag}} fylgir tilkynningunni. Óskað er eftir frekari fylgiskjölum áður en tekin verður afstaða til bótaskyldu.',
 'appeal': 'Kæra. Kærandi er {{a}}. Kæran varðar afgreiðslu umsóknar en felur ekki í sér endanlega niðurstöðu. Lögmaður kæranda: {{b}}. Rök {{ag}} eru sett fram í viðauka.',
 'procurement': 'Tilboðsyfirlit. Tengiliður bjóðanda {{org}} er {{a}}. {{b}} tekur við spurningum fyrir hönd kaupanda. Uppgefin fjárhæð er án viðbótarkostnaðar. Þetta skjal er aðeins prófunardæmi.',
 'beneficiary': 'Beiðni um breytingu. Rétthafi: {{a}}. Samkvæmt beiðni {{ag}} skal senda staðfestingu til viðkomandi eftir afgreiðslu. Yfirfarið af {{b}}. Engar bankaupplýsingar fylgja þessu dæmi.',
 'compliance': 'Eftirlitsblað. Viðskiptavinur {{a}} hefur óskað eftir endurskoðun skráningar. Umsjónarmaður máls: {{b}}. Niðurstaða bíður yfirferðar og undirritunar. Athugasemdir eru í fylgiskjali.',
 'english_estate': 'Estate administration. The estate of {{a}} is represented by {{b}}. The asset schedule is provisional and no distribution has been authorized. Contact the representative before making amendments.',
 'negative_budget': 'Fjárhagsáætlun {{org}}. Tekjur og gjöld eru sýnd eftir ársfjórðungum. Arður, Vöxtur og Jafnvægi eru heiti tilbúinna verkefna en ekki mannanöfn. Engir einstaklingar eru nefndir í skjalinu.',
 'negative_policy': 'Málsmeðferð. {{org}} skal varðveita fylgiskjöl þar til yfirferð lýkur. Starfsheiti vísa til hlutverka. Umsókn, samþykki og afgreiðsla eru skráð sérstaklega. Skjalið inniheldur engin mannanöfn.',
}
TOKEN = re.compile(r"\w+(?:[-’']\w+)*|[^\w\s]", re.UNICODE)
PLACEHOLDER = re.compile(r'\{\{(a|ag|b|org)\}\}')


def render(template, a, ag, b, org):
    values={'a':('Person',a),'ag':('Person',ag),'b':('Person',b),'org':('Organization',org)}
    chunks, entities, cursor, size = [], [], 0, 0
    for match in PLACEHOLDER.finditer(template):
        prefix=template[cursor:match.start()]; chunks.append(prefix); size+=len(prefix.encode())
        kind,value=values[match[1]]
        chunks.append(value); end=size+len(value.encode())
        entities.append(dict(kind=kind,start=size,end=end)); size=end; cursor=match.end()
    chunks.append(template[cursor:]); raw=''.join(chunks)
    tokens=[]; starts={}; ends={}; offset=0
    for match in TOKEN.finditer(raw):
        token=match[0]; a=len(raw[:match.start()].encode()); b=len(raw[:match.end()].encode())
        starts[a]=offset; offset+=len(token.encode()); ends[b]=offset; offset+=1; tokens.append(token)
    normalized=[dict(kind=e['kind'],start=starts[e['start']],end=ends[e['end']]) for e in entities]
    return dict(text=' '.join(tokens),entities=normalized,original_text=raw,original_entities=entities)


def generate():
    result=[]
    for split,templates,names in [('train',TRAIN_TEMPLATES,TRAIN_NAMES),('dev',DEV_TEMPLATES,DEV_NAMES)]:
        for family,template in templates.items():
            for i,(name,genitive) in enumerate(names):
                for variant in ('full','single','upper','initial'):
                    a,ag,b=name,genitive,names[(i+1)%len(names)][0]
                    if variant=='single':a=a.split()[0];ag=ag.split()[0];b=b.split()[0]
                    elif variant=='upper':a=a.upper();ag=ag.upper();b=b.upper()
                    elif variant=='initial':a=a[0]+'. '+a.split(' ',1)[1];ag=ag[0]+'. '+ag.split(' ',1)[1];b=b[0]+'. '+b.split(' ',1)[1]
                    row=render(template,a,ag,b,'Sýnibók ehf.' if split=='train' else 'Prófskjal ehf.')
                    row.update(id=f'legal-financial-v2-{split}-{family}-{i:02}-{variant}',split=split,template_family=family,variant=variant,review_status='unreviewed-synthetic',source='authored-synthetic-v2')
                    # All-negative template variants repeat verbatim; keep one.
                    if family.startswith('negative_'):continue
                    result.append(row)
            if family.startswith('negative_'):
                for i in range(96 if split=='train' else 32):
                    org=('Sýnibók' if split=='train' else 'Prófskjal')+f' {i+1} ehf.'
                    extra=f'\nFylgiskjal {i+1}: heildarfjárhæð {1000+i*137} kr. Staða: í yfirferð.'
                    row=render(template+extra,'','','',org)
                    row.update(id=f'legal-financial-v2-{split}-{family}-negative-{i:03}',split=split,template_family=family,variant='negative',review_status='unreviewed-synthetic',source='authored-synthetic-v2')
                    result.append(row)
    return result


def write(output):
    output.mkdir(parents=True,exist_ok=False)
    records=generate();manifest={'schema':1,'purpose':'Unreviewed synthetic training/development supplement, not independent validation','splits':{}}
    for split in ('train','dev'):
        selected=[r for r in records if r['split']==split]
        path=output/(split+'.jsonl');path.write_text(''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in selected))
        manifest['splits'][split]=dict(documents=len(selected),person_spans=sum(e['kind']=='Person' for r in selected for e in r['entities']),negative_documents=sum(not any(e['kind']=='Person' for e in r['entities']) for r in selected),sha256=hashlib.sha256(path.read_bytes()).hexdigest(),families=sorted({r['template_family'] for r in selected}))
    (output/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    return manifest


if __name__=='__main__':
    p=argparse.ArgumentParser(description=__doc__);p.add_argument('--output',type=Path,required=True)
    print(json.dumps(write(p.parse_args().output),indent=2))

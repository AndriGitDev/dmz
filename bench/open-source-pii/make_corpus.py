"""Author-labelled synthetic diagnostics, fixed before candidate inference.

Markup is annotation, not a detector. It yields unique literal labels compatible
with the existing packaged-system harness. Do not treat these as field samples.
"""
import hashlib
import json
from pathlib import Path
import re

# All names, narratives, addresses and credentials below are fabricated fixtures.
# North American phones use reserved fictional 555-01xx subscriber numbers.
CASES = {
    'english-general': [
        ('greeting', 'Please ask {{name_demo|Emily Carter}} to call me.'),
        ('single', 'I spoke with {{name_demo|Oliver}} yesterday.'),
        ('surname', 'The customer is Ms. {{name_demo|Whitmore}}.'),
        ('title', 'Dr. {{name_demo|Maya Patel}} signed the letter.'),
        ('initials', 'Please contact {{name_demo|R. J. Bennett}} about the delivery.'),
        ('hyphen-name', '{{name_demo|Anna-Louise Foster}} will attend the meeting.'),
        ('apostrophe', 'Our caller was {{name_demo|Siobhan O’Donnell}}.'),
        ('ascii-apostrophe', "Reply to {{name_demo|Patrick O'Neill}}."),
        ('accented', 'The appointment is for {{name_demo|José Navarro}}.'),
        ('nfd', 'The appointment is for {{name_demo|Jose\u0301 Navarro}}.'),
        ('nonwestern', '{{name_demo|Wei Zhang}} asked for a receipt.'),
        ('full-middle', 'The account holder is {{name_demo|Amelia Rose Hartley}}.'),
        ('possessive', "{{name_demo|Daniel Brooks}}'s delivery is delayed."),
        ('list', 'Attendees: {{name_demo|Emma Fletcher}}; {{name_demo|Noah Mercer}}.'),
        ('signature', 'Kind regards,\n{{name_demo|Sophie Lawson}}\nCustomer Support'),
        ('lowercase', 'customer name: {{name_demo|emily carter}}'),
        ('uppercase', 'CUSTOMER: {{name_demo|EMILY CARTER}}'),
        ('street', 'Send the parcel to {{address|18 Maple Avenue}}.'),
        ('us-address', 'Shipping address: {{address|42 Cedar Lane, Springfield, IL 62704}}.'),
        ('unit-address', 'The delivery address is {{address|87 Willow Road, Apt 6}}.'),
        ('uk-address', 'Please deliver to {{address|24 Birch Close, London, SW1A 2AA}}.'),
        ('address-lines', 'Mailing address:\n{{address|31 Orchard Street\nBristol\nBS1 4ST}}'),
        ('phone-us', 'Call {{phone|+1 202-555-0107}} after lunch.'),
        ('phone-local', 'Phone: {{phone|(202) 555-0114}}.'),
        ('phone-dots', 'Contact number: {{phone|202.555.0138}}.'),
        ('email', 'Write to {{email|emily.carter@example.invalid}}.'),
        ('email-tag', 'Support: {{email|case+review@example.invalid}}.'),
        ('email-punctuation', 'Contact <{{email|a.b@example.invalid}}> for help.'),
        ('card', 'Test payment card: {{card|4111 1111 1111 1111}}.'),
        ('multi', 'Customer {{name_demo|Isabel Reed}} lives at {{address|19 Linden Street}}. Email {{email|isabel.reed@example.invalid}}.'),
        ('negative-month', 'May and June are busy months for the support team.'),
        ('negative-verb', 'Mark the completed items and bill the account next week.'),
        ('negative-org', 'Willow Systems acquired Cedar Analytics last spring.'),
        ('negative-location', 'Our offices are in London and Reykjavík.'),
        ('negative-topic', 'The report covers English grammar and Icelandic spelling.'),
        ('negative-title', 'The doctor signed the letter and the customer received it.'),
        ('negative-numeric', 'Order 20251005 has 18 items and costs 149.95 USD.'),
        ('negative-streetword', 'The street is narrow and the avenue has tall trees.'),
        ('negative-placeholder', 'Customer: [NAME], phone: [PHONE], address: [ADDRESS].'),
        ('negative-empty', 'Please summarize the attached draft when it becomes available.'),
    ],
    'english-work': [
        ('legal', 'The claimant {{name_demo|Harriet Collins}} requests a copy of the agreement.'),
        ('finance', 'Account review for {{name_demo|Benjamin Ward}}: no payment is due.'),
        ('medical', 'Patient {{name_demo|Lucas Fleming}} was seen by Dr. {{name_demo|Nora Ellis}}.'),
        ('ticket', 'Ticket owner: {{name_demo|Chloe Morgan}}\nContact: {{email|chloe@example.invalid}}'),
        ('chat', 'agent: Hello, who am I speaking with?\ncustomer: {{name_demo|Ethan Hayes}}.'),
        ('json-name', '{"customer_name": "{{name_demo|Zoe Chambers}}", "status": "open"}'),
        ('csv', 'name,email\n{{name_demo|Oscar Finch}},{{email|oscar.finch@example.invalid}}'),
        ('markdown', '| Customer | Email |\n| {{name_demo|Lily Sutton}} | {{email|lily@example.invalid}} |'),
        ('quoted-email', 'Email address: {{email|"test user"@example.invalid}}'),
        ('unicode-email', 'Email: {{email|jóse@example.invalid}}.'),
        ('phone-json', '{"phone": "{{phone|+1-202-555-0148}}"}'),
        ('phone-spaces', 'Telephone: {{phone|+1 202 555 0162}}'),
        ('phone-parentheses', 'Please call {{phone|+1 (202) 555-0185}}.'),
        ('address-json', '{"street_address": "{{address|56 Hawthorn Drive}}"}'),
        ('address-unit', 'Billing: {{address|63 Elm Street, Unit B, Denver, CO 80202}}'),
        ('name-address', '{{name_demo|Ava Spencer}}\n{{address|71 Meadow Lane}}'),
        ('bearer', 'Authorization: Bearer {{secret|fictitiousA9_qW2-zE7rT4uY6iO8pL1kJ3hG5}}'),
        ('github', 'GitHub token: {{secret|ghp_Ab3Cd4Ef5Gh6Ij7Kl8Mn9Op0Qr1St2Uv3Wx4}}'),
        ('password', 'password="{{secret|V7xQ2mR9tL4pZ8nK3wD6sF1gH5jC0bA2}}"'),
        ('db-url', 'DATABASE_URL={{secret|postgres://fixture:syntheticpass@db.example.invalid:5432/app}}'),
        ('private-key', '{{secret|-----BEGIN PRIVATE KEY-----\nU1lOVEhFVElDLVRFU1QtT05MWQ==\n-----END PRIVATE KEY-----}}'),
        ('is-iban', 'Icelandic test IBAN: {{iban|IS140159260076545510730339}}.'),
        ('card-json', '{"test_card": "{{card|4242 4242 4242 4242}}"}'),
        ('multi-work', '{{name_demo|Grace Holloway}}: {{phone|+1 202-555-0194}}, {{email|grace@example.invalid}}. Please update the contact record.'),
        ('negative-var', 'const customerName = user.displayName;'),
        ('negative-credential-var', 'const apiKey = process.env.API_KEY;'),
        ('negative-guidance', 'Store passwords in a password manager and rotate access tokens.'),
        ('negative-invoice', 'Invoice INV-2048 totals USD 1200.00, payable within 30 days.'),
        ('negative-org-person', 'Carter Publishing released a new edition of the manual.'),
        ('negative-code', '```js\nconst password = credentials.password;\n```'),
        ('negative-url', 'Documentation: https://docs.example.invalid/setup'),
        ('negative-status', '{"status": "ready", "attempts": 3, "enabled": true}'),
        ('negative-role', 'The claimant and the respondent accepted the settlement.'),
        ('negative-meeting', 'Engineering and Customer Support meet every Thursday.'),
        ('negative-ambiguous', 'Rose is a flower; violet is a colour; a robin is a bird.'),
        ('negative-values', 'The sample contains 1024 bytes, 256 tokens and 32 records.'),
    ],
    'mixed-is-en': [
        ('en-icelandic-name', 'Please contact {{name_demo|Arna Jónsdóttir}} about the file.'),
        ('is-english-name', 'Sendu skjalið til {{name_demo|Emily Carter}} fyrir fundinn.'),
        ('two-languages', '{{name_demo|Bjarni Jónsson}} spoke with {{name_demo|Oliver Bennett}} yesterday.'),
        ('case-name', 'Ég ræddi við {{name_demo|Örnu Jónsdóttur}}. Please send the summary.'),
        ('icelandic-street', 'The delivery address is {{address|Laugavegur 12}}.'),
        ('is-english-street', 'Sendu pakkann á {{address|18 Maple Avenue}}.'),
        ('icelandic-phone', 'Sími: {{phone|+354 555 1234}}. Please call tomorrow.'),
        ('us-phone', 'Hringdu í {{phone|+1 202-555-0123}} eftir hádegi.'),
        ('mixed-emails', 'Netfang: {{email|anna@example.invalid}}. Reply in English.'),
        ('combined', '{{name_demo|Arna Jónsdóttir}} og {{name_demo|Charlotte Blake}} mæta á fundinn.'),
        ('unicode-english', 'Viðskiptavinur: {{name_demo|Zoë Fisher}}. Please confirm.'),
        ('unicode-nfd', 'Viðskiptavinur: {{name_demo|Zoe\u0308 Fisher}}. Please confirm.'),
        ('signature', 'Kveðja / Best regards,\n{{name_demo|Henry Caldwell}}'),
        ('company-negative', 'Cedar Systems og Gervifélagið vinna að verkefninu.'),
        ('place-negative', 'Fundurinn er í Reykjavík, followed by a trip to London.'),
        ('month-negative', 'Skýrslan nær yfir maí og júní, May and June.'),
        ('common-negative', 'Please mark the document. Vinsamlegast merktu skjalið.'),
        ('numbers-negative', 'Verkefni 2048 kostar 1200 kr. The total is provisional.'),
        ('code-negative', 'Skoðaðu stillinguna: const email = profile.email;'),
        ('roles-negative', 'Læknirinn og hjúkrunarfræðingurinn reviewed the case.'),
        ('card', 'Prófunarkort: {{card|4111 1111 1111 1111}}. Test only.'),
        ('tenant', 'Please summarize {{tenant_dictionary|Verkefni Gervi}} á íslensku.'),
        ('address-name', '{{name_demo|Bjarni Jónsson}}: {{address|Laugavegur 12}}, reply in English.'),
        ('secret', 'Prófun: Authorization: Bearer {{secret|TestR7xQ2mN9pL4zW8cV3bD6fG1hJ5kS0aU2}}'),
    ],
}

def build():
    root=Path(__file__).resolve().parents[1]/'corpus'/'open-source-pii-v1'
    root.mkdir(exist_ok=True)
    manifest={'schema':1,'independent_validation':False,'authorship':'manually authored synthetic diagnostic, before candidate predictions','scope':['name_demo','address','phone','email','iban','card','kennitala','secret','tenant_dictionary'],'sets':{}}
    regex=re.compile(r'\{\{([a-z_]+)\|(.*?)\}\}', re.S)
    for stratum,cases in CASES.items():
        rows=[]
        for identifier,marked in cases:
            entities=[]
            def repl(m):
                entities.append({'kind':m[1],'value':m[2]});return m[2]
            text=regex.sub(repl,marked)
            assert '{{' not in text
            assert all(text.count(e['value'])==1 for e in entities)
            assert len(set((e['kind'],e['value']) for e in entities))==len(entities)
            rows.append({'id':stratum+'-'+identifier,'stratum':stratum,'family':identifier,'text':text,'entities':entities})
        data=''.join(json.dumps(row,ensure_ascii=False)+'\n' for row in rows).encode()
        path=root/(stratum+'.jsonl')
        if path.exists() and path.read_bytes()!=data:raise ValueError('refuse to overwrite frozen corpus')
        path.write_bytes(data)
        manifest['sets'][path.name]={'sha256':hashlib.sha256(data).hexdigest(),'excerpts':len(rows),'positive_excerpts':sum(bool(r['entities']) for r in rows),'entities':sum(len(r['entities']) for r in rows)}
    (root/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
    print(json.dumps(manifest,indent=2))

if __name__=='__main__':build()

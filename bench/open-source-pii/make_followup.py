"""Fresh synthetic text after bounded calibration; not independent field data."""
import hashlib,json,re
from pathlib import Path
CASES={
'english-general':[
('new-full','Please send the receipt to {{name_demo|Leah Whitfield}}.'),
('title','Professor {{name_demo|Nathaniel Rowan}} approved the draft.'),
('initial','The applicant is {{name_demo|J. A. Prescott}}.'),
('hyphen','Meet {{name_demo|Mary-Jane Calloway}} at the entrance.'),
('apostrophe','The customer is {{name_demo|Fionn O’Rourke}}.'),
('nfd','Please contact {{name_demo|Rene\u0301 Laurent}}.'),
('single','Hello, my name is {{name_demo|Priya}}.'),
('nonwestern','The package belongs to {{name_demo|Min Seo Kim}}.'),
('surname','The caller introduced herself as Ms. {{name_demo|Hargreaves}}.'),
('possessive',"{{name_demo|Aiden Lockwood}}'s appointment was moved."),
('address-number','Delivery address: {{address|68 Copperfield Crescent}}.'),
('address-apartment','Send it to {{address|23 Juniper Court, Apartment 9}}.'),
('address-direction','Shipping: {{address|142 West Beech Street}}.'),
('address-two-lines','Address:\n{{address|9 Sycamore Terrace\nBristol BS1 4ST}}'),
('address-us','The customer lives at {{address|74 Aspen Road, Denver, CO 80202}}.'),
('address-uk','Mail it to {{address|16 Hazel Mews, London, SW1A 2AA}}.'),
('address-unit-first','Deliver to {{address|Flat 4, 35 Poplar Avenue}}.'),
('phone','Telephone: {{phone|+1 303-555-0126}}.'),
('local-phone','Call {{phone|(415) 555-0173}}.'),
('email','Contact {{email|leah.whitfield+case@example.invalid}}.'),
('city-negative','The train runs between Bristol and London.'),
('role-negative','Ask the applicant to contact the receptionist.'),
('street-negative','The avenue has a footpath and the road has a cycle lane.'),
('title-negative','The professor published a textbook on grammar.'),
('number-negative','There are 68 shelves and 23 storage boxes.'),
('month-negative','April is followed by May, then June.'),
('nameword-negative','A rose grew beside the hazel tree.'),
('org-negative','Rowan Logistics is a fictional company in this exercise.'),
('address-field-negative','The form requires an address, postcode and telephone number.'),
('placeholder-negative','Recipient: [FULL NAME]. Address: [STREET ADDRESS].'),
],
'english-work':[
('medical','Patient {{name_demo|Elena Marlowe}} was seen by Dr. {{name_demo|Callum Reeves}}.'),
('legal','The respondent {{name_demo|Freya Ashford}} signed the statement.'),
('finance','Please update the billing record for {{name_demo|Samuel Wren}}.'),
('json','{"full_name":"{{name_demo|Imogen Fenwick}}","status":"pending"}'),
('csv','person,email\n{{name_demo|Tobias Hale}},{{email|tobias.hale@example.invalid}}'),
('signature','Best wishes,\n{{name_demo|Miriam Ellison}}\nAccounts Team'),
('lowercase','customer: {{name_demo|noor al-hassan}}'),
('uppercase','CLIENT NAME: {{name_demo|CALEB STERLING}}'),
('address-json','{"delivery_address":"{{address|48 Hawthorn Crescent}}"}'),
('address-legal','The notice was served at {{address|27 Chestnut Lane, Unit 3}}.'),
('address-finance','Registered mailing address: {{address|86 South Willow Drive, Austin, TX 78701}}.'),
('address-chat','customer: Please send the replacement to {{address|11 Alder Close, Manchester, M1 1AE}}.'),
('address-spaces','Billing address:\n{{address|5 Oakfield Road, Suite 210}}'),
('phone-json','{"contact_phone":"{{phone|+44 20 7946 0632}}"}'),
('phone-extension','Office telephone: {{phone|+1 212-555-0184 ext. 27}}.'),
('phone-local','Customer phone: {{phone|415.555.0173}}.'),
('email-unicode','Customer email: {{email|rené.laurent@example.invalid}}.'),
('secret','Authorization: Bearer {{secret|fixtureP8qR2sT9uV4wX6yZ3aB7cD5eF1gH0jK}}'),
('card','Test card number: {{card|4111 1111 1111 1111}}.'),
('mixed-fields','{{name_demo|Dahlia Finch}}\n{{address|62 Laurel Street}}\n{{email|dahlia@example.invalid}}'),
('negative-schema','{"full_name": null, "street_address": null, "phone": null}'),
('negative-code','const postalAddress = customer.address;'),
('negative-guide','Never paste passwords or private keys into a support ticket.'),
('negative-contract','The claimant and respondent agreed to mediation.'),
('negative-role','Accounts Payable and Customer Care reviewed the request.'),
('negative-placeholder','Patient name: REDACTED. Telephone: WITHHELD.'),
('negative-version','Client version 415.555.0173 failed the compatibility check.'),
('negative-api','The API property `person.full_name` contains a string.'),
('negative-location','The team is based in Reykjavík and Copenhagen.'),
('negative-street-title','The book is called The Long Road Home.'),
],
'mixed-is-en':[
('english-person','Vinsamlegast sendu svarið til {{name_demo|Leah Whitfield}}.'),
('icelandic-person','Please send the invoice to {{name_demo|Katrín Guðmundsdóttir}}.'),
('both','{{name_demo|Ólafur Einarsson}} met {{name_demo|Freya Ashford}} yesterday.'),
('inflected','Ég ræddi við {{name_demo|Katrínu Guðmundsdóttur}}. Please follow up.'),
('english-address','Sendu pakkann á {{address|68 Copperfield Crescent}}.'),
('icelandic-address','Please deliver to {{address|Laugavegur 24}}.'),
('address-flat','Nýtt heimilisfang: {{address|Flat 4, 35 Poplar Avenue}}.'),
('address-line','Heimilisfang / Address:\n{{address|9 Sycamore Terrace\nBristol BS1 4ST}}'),
('english-phone','Hringdu í {{phone|+1 303-555-0126}} eftir hádegi.'),
('british-phone','Sími / Phone: {{phone|+44 20 7946 0632}}.'),
('icelandic-phone','Please call {{phone|+354 555 4321}}.'),
('local-is-phone','Phone: {{phone|555-4321}}.'),
('email','Senda á {{email|katrín@example.invalid}}. Please confirm receipt.'),
('nfd','Viðtakandi: {{name_demo|Rene\u0301 Laurent}}. Delivery tomorrow.'),
('legal','Lögmaður {{name_demo|Callum Reeves}} asks for the document.'),
('health','Sjúklingur {{name_demo|Elena Marlowe}} needs a copy of the summary.'),
('signature','Með kveðju / Regards,\n{{name_demo|Tobias Hale}}'),
('iban','Please use test account {{iban|IS140159260076545510730339}}. Takk.'),
('tenant','Please summarize {{tenant_dictionary|Verkefni Gervi}} á íslensku.'),
('fields','{{name_demo|Miriam Ellison}}: {{email|miriam@example.invalid}}. Svara á ensku.'),
('negative-roles','Læknirinn and the nurse reviewed the chart.'),
('negative-place','Við fórum til London and returned through Reykjavík.'),
('negative-fields','Skráðu nafn, heimilisfang and phone number in the form.'),
('negative-translation','Translate the words person, address and telephone into Icelandic.'),
('negative-code','const heimilisfang = customer.address; // shipping field'),
('negative-organization','Willow Systems og Rowan Logistics birtu skýrslu.'),
('negative-numbers','Það eru 24 pages and 68 rows in the document.'),
('negative-placeholders','Nafn: [NAME]. Heimilisfang: [ADDRESS]. Sími: [PHONE].'),
('negative-calendar','Fundurinn er í apríl. The next meeting is in May.'),
('negative-road','Gatan var lokuð. The road is open again.'),
]}
root=Path(__file__).resolve().parents[1]/'corpus/open-source-pii-followup-v1';root.mkdir(exist_ok=True)
manifest={'schema':1,'independent_validation':False,'authorship':'fresh synthetic text authored after bounded calibration; before follow-up predictions; not independently annotated or sampled','scope':['name_demo','address','phone','email','iban','card','kennitala','secret','tenant_dictionary'],'sets':{}}
for stratum,cases in CASES.items():
 rows=[]
 for identifier,marked in cases:
  entities=[]
  def repl(m):entities.append({'kind':m[1],'value':m[2]});return m[2]
  text=re.sub(r'\{\{([a-z_]+)\|(.*?)\}\}',repl,marked,flags=re.S)
  assert all(text.count(e['value'])==1 for e in entities)
  rows.append({'id':stratum+'-'+identifier,'stratum':stratum,'family':identifier,'text':text,'entities':entities})
 data=''.join(json.dumps(r,ensure_ascii=False)+'\n' for r in rows).encode();p=root/(stratum+'.jsonl')
 if p.exists() and p.read_bytes()!=data:raise ValueError('frozen follow-up changed')
 p.write_bytes(data);manifest['sets'][p.name]={'sha256':hashlib.sha256(data).hexdigest(),'excerpts':len(rows),'positive_excerpts':sum(bool(r['entities']) for r in rows),'entities':sum(len(r['entities']) for r in rows)}
(root/'manifest.json').write_text(json.dumps(manifest,indent=2)+'\n')
print(json.dumps(manifest,indent=2))

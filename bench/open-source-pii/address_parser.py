"""Pinned US-address parser grammar; calibration prototype, not postal validation."""
import re
import usaddress

STREET = {'AddressNumber', 'AddressNumberSuffix', 'StreetNamePreModifier', 'StreetNamePreDirectional',
          'StreetNamePreType', 'StreetName', 'StreetNamePostType', 'StreetNamePostDirectional',
          'OccupancyType', 'OccupancyIdentifier', 'SubaddressType', 'SubaddressIdentifier'}
REQUIRED = {'AddressNumber', 'StreetName', 'StreetNamePostType'}
STATES = set('AL AK AZ AR CA CO CT DE FL GA HI ID IL IN IA KS KY LA ME MD MA MI MN MS MO MT NE NV NH NJ NM NY NC ND OH OK OR PA RI SC SD TN TX UT VT VA WA WV WI WY DC AS GU MP PR VI'.split())
ABBREVIATIONS = set('st rd ave blvd ln ct dr pl pkwy hwy sq ter trl cir'.split())


def parsed(text):
    if len(text.encode()) > 4096 or re.search(r'&(?:amp;|#38;)', text):
        raise ValueError('unsupported input')
    tokens = usaddress.tokenize(text)
    if not tokens:
        return []
    labels = usaddress.TAGGER.tag(usaddress.tokens2features(tokens))
    result, cursor = [], 0
    for i, (token, label) in enumerate(zip(tokens, labels)):
        start = text.find(token, cursor)
        if start < 0:
            raise ValueError('token alignment failure')
        end = start + len(token)
        result.append({'start': start, 'end': end, 'token': token, 'label': label,
                       'probability': usaddress.TAGGER.marginal(label, i)})
        cursor = end
    return result

def qualifying(tokens):
    if not REQUIRED.issubset({t['label'] for t in tokens}):
        return False
    required = [t for t in tokens if t['label'] in REQUIRED]
    if min(t['probability'] for t in required) < .90:
        return False
    numbers = [t['token'].strip('(),;') for t in required if t['label'] == 'AddressNumber']
    suffixes = [t['token'].strip('().,;\n').lower() for t in required if t['label'] == 'StreetNamePostType']
    return all(re.fullmatch(r'[0-9]{1,6}[A-Za-z]?(?:-[0-9]{1,6}[A-Za-z]?)?', n) for n in numbers) and all(s in usaddress.STREET_NAMES for s in suffixes)

def standalone(text):
    tokens = parsed(text)
    result = []
    for i, token in enumerate(tokens):
        if token['label'] != 'AddressNumber':
            continue
        end = i + 1
        while end < len(tokens) and tokens[end]['label'] in STREET and tokens[end]['label'] != 'AddressNumber':
            end += 1
        part = tokens[i:end]
        if not qualifying(part):
            continue
        tail = end
        while tail < len(tokens) and tokens[tail]['label'] in {'PlaceName', 'StateName', 'ZipCode'}:
            tail += 1
        extra = tokens[end:tail]
        state = [t['token'].strip('.,;()').upper() for t in extra if t['label'] == 'StateName']
        postal = [t['token'].strip('.,;()') for t in extra if t['label'] == 'ZipCode']
        if len(state) == 1 and state[0] in STATES and len(postal) == 1 and re.fullmatch(r'[0-9]{5}(?:-[0-9]{4})?', postal[0]):
            end = tail
            part = tokens[i:end]
        s, e = token['start'], part[-1]['end']
        while e > s and text[e-1] in ',;)\n\r':
            e -= 1
        last = text[part[-1]['start']:e]
        if last.endswith('.') and not (part[-1]['label'] == 'StreetNamePostType' and last[:-1].lower() in ABBREVIATIONS):
            e -= 1
        result.append({'span': [len(text[:s].encode()), len(text[:e].encode()), 'address'],
                       'score': min(t['probability'] for t in part)})
    return result


import re

src = open(r'd:\115us\115-server\server\work\dfo-lan\internal\legion\ispins_vectors.generated.go', encoding='utf-8').read()
m = re.search(r'var ispinsInfoTemplates = map\[string\]string\{.*?\n\}', src, re.S)
body = m.group(0)
pairs = re.findall(r'"([a-z0-9]+)":\s*"([0-9a-f]+)"', body)
print('templates:', len(pairs))
for name, h in pairs:
    b = bytes.fromhex(h)
    place, state, outcome, stages_done = b[2], b[3], b[7], b[11]
    recs = [(b[19 + 8 * k], b[20 + 8 * k]) for k in range(4) if 19 + 8 * k + 1 < len(b)]
    print('%-10s place=%02x state=%02x outcome=%02x done=%02x recs(idx,status)=%s' % (
        name, place, state, outcome, stages_done, ['%d:%d' % r for r in recs]))

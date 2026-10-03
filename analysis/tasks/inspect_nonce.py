import re
src = open(r"d:\115us\115-server\server\work\dfo-lan\internal\legion\ispins_vectors.generated.go", encoding="utf-8").read()
for name in ["ispinsRunEntryCharacterInfoTemplate", "ispinsLoginEntryCharacterInfoTemplate"]:
    m = re.search(name + r'\s*=\s*"([0-9a-f]+)"', src)
    if not m:
        print(name, "NOT FOUND")
        continue
    h = m.group(1)
    b = bytes.fromhex(h)
    print(name, "len", len(b))
    print("  nonce 256-260:", b[256:261].hex())
    print("  bytes 21/53/69/85/93/94/117:", b[21], b[53], b[69], b[85], b[93], b[94], b[117])
    print("  head 0..12:", b[0:12].hex())

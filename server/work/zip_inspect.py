import zipfile, sys

ZIP_PATH = r'D:\游戏\端游\DNF\115脱壳\115us千海天\DFO.zip'
zf = zipfile.ZipFile(ZIP_PATH)
infos = zf.infolist()
print('entries:', len(infos))

def real_name(info):
    raw = info.filename
    if info.flag_bits & 0x800:
        return raw
    try:
        return raw.encode('cp437').decode('gbk')
    except Exception:
        return raw

for i in infos[:20]:
    print(i.file_size, hex(i.CRC), real_name(i))

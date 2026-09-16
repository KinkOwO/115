import zipfile

zf = zipfile.ZipFile(r'D:\游戏\端游\DNF\115脱壳\115us千海天\DFO.zip')
want = {'DFO/DFO.exe', 'DFO/Script.pvf', 'DFO/sk.dat',
        'DFO/version.ini', 'DFO/NeopleLauncher.exe', 'DFO/steam_api.dll'}
for info in zf.infolist():
    if info.filename in want:
        print(f'{info.filename:32s} size={info.file_size:>12} crc={info.CRC:08x}')

"""Package the native third-card plugin; never install into a live client."""
import hashlib
import json
from pathlib import Path
import zipfile

ROOT = Path(__file__).resolve().parent
DLL = ROOT / 'dist' / 'ApocalypseThirdCard.dll'


def main():
    data = DLL.read_bytes()
    # A package must carry the reviewed CN provenance rather than merely claim
    # that its resources are original. Keep local installation paths out of it.
    audit = json.loads((ROOT / 'dist' / 'artwork-audit.json').read_text(encoding='utf-8'))
    cn, us = audit['sources']['cn'], audit['sources']['us']
    if (audit.get('generated_artwork') is not False
            or audit.get('third_card_matches_cn_original') is not True
            or cn['third_card'] != us['third_card']
            or cn['symbol_flat_l_sha256'] != us['symbol_flat_l_sha256']):
        raise ValueError('Run the CN/US original-art audit before packaging')
    provenance = {
        'generated_artwork': False,
        'third_card_matches_cn_original': True,
        'background_entry': 'sprite/contents/2026/apocalypse/ui/levelselect.img',
        'background_frame': 3,
        'symbol_entry': 'sprite/interface2/legion/legioncommon/symbol_flat_l.img',
        'sources': audit['sources'],
    }
    manifest = {
        'schema': 2, 'id': 'qol.apocalypse-third-card', 'version': '0.1.0',
        'name': '末世录第三档原生卡片', 'author': 'DFO 115us',
        'description': '扩展当前 US 原生作战卡片数组，启用源资源中的第三档；候选，待实机验证。',
        'permissions': ['client.file.write'], 'requires': ['qol.client-host'],
        'layers': {'client': {'ops': [{
            'kind': 'file.add', 'target': '.115us-mods/' + DLL.name,
            'source': 'client/' + DLL.name, 'size': len(data),
            'sha256': hashlib.sha256(data).hexdigest(),
            'note': '仅内存修改；关闭游戏后卸载。指令不匹配或加载过晚会拒绝启用。',
        }]}},
    }
    output = ROOT / 'dist' / 'apocalypse-third-card-0.1.0.zip'
    with zipfile.ZipFile(output, 'w', zipfile.ZIP_DEFLATED) as package:
        package.writestr('mod.json', json.dumps(manifest, ensure_ascii=False, indent=2) + '\n')
        package.writestr('client/' + DLL.name, data)
        package.write(ROOT / 'README.md', 'README.md')
        package.writestr('artwork-provenance.json', json.dumps(provenance, ensure_ascii=False, indent=2) + '\n')
    print(output)
    print('SHA256', hashlib.sha256(output.read_bytes()).hexdigest())


if __name__ == '__main__':
    main()

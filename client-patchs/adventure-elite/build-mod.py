"""Package the opt-in eligibility candidate. Does not install or start a client."""
import hashlib
import json
from pathlib import Path
import zipfile

HERE = Path(__file__).resolve().parent
DIST = HERE / "dist"
DLL = DIST / "AdventureElite.dll"
VERSION = "0.3.10"


def main():
    data = DLL.read_bytes()
    manifest = {
        "schema": 2,
        "id": "qol.adventure-elite-eligibility",
        "version": VERSION,
        "name": "精锐角色资格候选",
        "author": "DFO 115us",
        "description": "取消资格门槛，保留真实属性；普通单人通关已验证；回城后重新入场登记修复候选；剧情及奥德赛战斗待验收。",
        "permissions": ["client.file.write"],
        "requires": ["qol.client-host"],
        "layers": {"client": {"ops": [{
            "kind": "file.add", "target": ".115us-mods/AdventureElite.dll",
            "source": "client/AdventureElite.dll", "size": len(data),
            "sha256": hashlib.sha256(data).hexdigest(),
            "note": "仅DFO_ADVENTURE_ELITE=1启用；日志及状态均写DLL目录；退出客户端后卸载。",
        }]}},
    }
    archive = DIST / f"adventure-elite-eligibility-{VERSION}.zip"
    with zipfile.ZipFile(archive, "w", zipfile.ZIP_DEFLATED) as z:
        z.writestr("mod.json", json.dumps(manifest, ensure_ascii=False, indent=2) + "\n")
        z.writestr("client/AdventureElite.dll", data)
        z.write(HERE / "README.md", "README.md")
        z.write(HERE / "vendor/minhook/LICENSE.txt", "LICENSE-MinHook.txt")
    print(f"DLL: {len(data)} bytes, SHA256 {manifest['layers']['client']['ops'][0]['sha256']}")
    print(f"Package: {archive} ({archive.stat().st_size} bytes)")


if __name__ == "__main__":
    main()

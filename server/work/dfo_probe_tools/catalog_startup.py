"""Legacy export checks, skipped only for the selected native source domains."""
import json
import pathlib
import sys


def uses_pvf_catalog(environment, domain):
 return domain in {part.strip() for part in environment.get("DFO_PVF_CATALOGS", "").split(",")}


def validate_json_catalogs(command, environment):
 if "-character-catalog" in command and not uses_pvf_catalog(environment, "characters"):
  _catalog = pathlib.Path(command[command.index("-character-catalog") + 1])
  if not _catalog.is_file():
   print("WARNING: 角色目录不存在：%s" % _catalog, file=sys.stderr)
  else:
   try:
    _professions = json.loads(_catalog.read_text(encoding="utf-8-sig")).get("professions") or {}
    if not any((p or {}).get("advancement_growth") for p in _professions.values()):
     print(
      "WARNING: 角色目录 %s 不含 growtype 分段数据（advancement_growth/advancement_skills）："
      "建号选定的转职分支不会落账，角色会停在基础职业。"
      "请改用带该数据的目录（例如 configs/characters.skycastle-release.json）。" % _catalog,
      file=sys.stderr,
     )
   except Exception as exc:
    print("WARNING: 无法解析角色目录 %s：%s" % (_catalog, exc), file=sys.stderr)
 if "-dungeon-catalog" in command and not uses_pvf_catalog(environment, "dungeons"):
  raise RuntimeError("副本内容已退休 JSON 入口，请使用当前源码程序并选择原生 PVF dungeons 域。")

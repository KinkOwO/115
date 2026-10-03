import json

SRC = r'd:\115us\115-server\analysis\tasks\next79-aux-bodies.json'
GO = r'd:\115us\115-server\server\work\dfo-lan\cmd\wireprobe\ispins_flow.go'

NAMES = {2204: 'ispins_legion_field_object', 2201: 'ispins_star_cluster',
         279: 'ispins_lag_statistics', 2168: 'ispins_training_counter',
         14: 'ispins_reward_item_granted'}
SIZES = {2204: 32, 2201: 32, 279: 16, 2168: 48, 14: 192}

# 官服 s4 帧号 → (表, 阶段)；组内按帧号升序即官服发送顺序。
GROUPS = {
    'ispinsAuxPre31': [[469, 470, 471], [570, 571, 572], [653, 654, 655], [758, 759, 760]],
    'ispinsAuxPre2252': [[474, 475, 476, 477, 478], [575, 576, 577], [657, 658, 659, 660], [763, 764, 765]],
    'ispinsAuxPre2253': [[480, 481, 482, 483, 484, 485, 486, 487], [579, 580, 581, 582], [662, 663, 664, 665], [767, 768, 769, 770, 771]],
    'ispinsAuxPre115': [[494], [589], [672], [781]],
}

with open(SRC, encoding='utf-8-sig') as f:
    bodies = {e['frame']: e for e in json.load(f)}

def emit(var):
    lines = ['var %s = [4][]ispinsAuxPacket{' % var]
    for stage, frames in enumerate(GROUPS[var]):
        lines.append('\t{ // stage %d' % stage)
        for fr in sorted(frames):
            e = bodies[fr]
            hexs = e['hex'].strip().lower()
            assert len(hexs) == SIZES[e['id']] * 2, (var, stage, fr, e['id'], len(hexs))
            lines.append('\t\t{"%s", %d, decodeHexOrDie("%s")},' % (NAMES[e['id']], e['id'], hexs))
        lines.append('\t},')
    lines.append('}')
    return '\n'.join(lines)

out = '\n'.join(emit(v) for v in ['ispinsAuxPre31', 'ispinsAuxPre2252', 'ispinsAuxPre2253', 'ispinsAuxPre115'])

with open(GO, encoding='utf-8') as f:
    text = f.read()
start = text.index('var ispinsAuxPre31')
end = text.index('// completeIspinsStage is the per-stage')
text = text[:start] + out + '\n\n' + text[end:]
with open(GO, 'w', encoding='utf-8') as f:
    f.write(text)
print('patched OK, tables bytes verified')

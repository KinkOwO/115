"""Read-only snapshots of the owned 115 client room caches.

No debugger attachment, process writes, client launching or player actions.
Offsets are from the authoritative IDB; see the task evidence document.
"""
import argparse
import ctypes as c
import ctypes.wintypes as w
import datetime
import json
import pathlib
import time

ROOT = pathlib.Path(__file__).resolve().parents[1]
K = c.WinDLL('kernel32', use_last_error=True)
PS = c.WinDLL('psapi', use_last_error=True)
K.OpenProcess.argtypes = [w.DWORD, w.BOOL, w.DWORD]
K.OpenProcess.restype = w.HANDLE
K.CloseHandle.argtypes = [w.HANDLE]
K.ReadProcessMemory.argtypes = [w.HANDLE, c.c_void_p, c.c_void_p, c.c_size_t, c.POINTER(c.c_size_t)]
K.QueryFullProcessImageNameW.argtypes = [w.HANDLE, w.DWORD, w.LPWSTR, c.POINTER(w.DWORD)]
PS.EnumProcessModules.argtypes = [w.HANDLE, c.c_void_p, w.DWORD, c.POINTER(w.DWORD)]


class Reader:
    def __init__(self, pid):
        self.handle = K.OpenProcess(0x410, False, pid)  # QUERY_INFORMATION | VM_READ
        if not self.handle:
            raise c.WinError(c.get_last_error())
        try:
            path = c.create_unicode_buffer(32768)
            size = w.DWORD(len(path))
            if not K.QueryFullProcessImageNameW(self.handle, 0, path, c.byref(size)):
                raise c.WinError(c.get_last_error())
            configured = json.loads((ROOT / 'server/launcher.local.json').read_text(encoding='utf-8-sig'))
            expected = (pathlib.Path(configured['client_dir']) / 'DFO.exe').resolve()
            if pathlib.Path(path.value).resolve() != expected:
                raise RuntimeError('PID is not the launcher-configured owned DFO.exe')
            modules = (c.c_void_p * 1024)()
            needed = w.DWORD()
            if not PS.EnumProcessModules(self.handle, modules, c.sizeof(modules), c.byref(needed)) or not modules[0]:
                raise c.WinError(c.get_last_error())
            self.delta = modules[0] - 0x140000000
        except Exception:
            self.close()
            raise

    def close(self):
        if self.handle:
            K.CloseHandle(self.handle)
            self.handle = None

    def read(self, addr, size):
        if not 0x10000 <= addr < 0x800000000000 or not 0 < size <= 4096:
            raise ValueError('invalid bounded read')
        data = c.create_string_buffer(size)
        got = c.c_size_t()
        if not K.ReadProcessMemory(self.handle, addr, data, size, c.byref(got)) or got.value != size:
            raise c.WinError(c.get_last_error())
        return data.raw

    def u(self, addr, size=8):
        return int.from_bytes(self.read(addr, size), 'little')

    def vec(self, addr, stride, limit=256):
        begin, end = self.u(addr), self.u(addr + 8)
        if end < begin or (end - begin) % stride or (end - begin) // stride > limit:
            raise ValueError('invalid vector bounds')
        return range(begin, end, stride)

    def manager(self, manager):
        if not manager:
            return None
        # 145DF0410 / 145DF0390: monster vector of 24-byte weak references.
        rows = list(self.vec(manager + 392, 24, 1024))
        live = []
        for row in rows:
            control, ref = self.u(row + 8), self.u(row + 16)
            if control and self.u(control + 8, 4) and ref:
                actor = ref - 48
                live.append({'actor': hex(actor), 'team': self.u(actor + 0xf10, 4)})
        return {'pointer': hex(manager), 'monster_slots': len(rows), 'valid_monsters': live}

    def actor_slot(self, slot):
        control, ref = self.u(slot + 8), self.u(slot + 16)
        return self.manager(ref if control and self.u(control + 8, 4) else 0)

    def descriptor(self, desc):
        return {'map': self.u(desc, 4), 'mode': self.u(desc + 56, 4),
                'spawn_rows': len(self.vec(desc + 8, 96, 255))}

    def snapshot(self):
        root = self.u(0x14E6839F0 + self.delta)
        if not root:
            return {'in_dungeon': False}
        # 144ED9C60 is the vtable +216 scene accessor of the owned dungeon state.
        accessor = self.u(self.u(root) + 216)
        if accessor != 0x144ED9C60 + self.delta:
            return {'in_dungeon': False, 'accessor': hex(accessor)}
        scene = self.u(root + 144)
        dungeon = self.u(scene + 280) if scene else 0
        if not dungeon:
            return {'in_dungeon': False}
        current = {'in_dungeon': True, 'scene': hex(scene), 'dungeon': hex(dungeon),
                   'position': [self.u(scene + 288, 4), self.u(scene + 292, 4)],
                   'scene_state': self.u(scene + 2360, 4), 'layer_flag': self.u(scene + 2892, 1),
                   'active_room': hex(self.u(scene + 176)),
                   'transition_hex': self.read(scene + 2896, 40).hex(),
                   'active_entities': self.actor_slot(scene + 256)}
        base_desc = list(self.vec(dungeon + 424, 64))
        base_rooms = list(self.vec(dungeon + 448, 8))
        base_actors = list(self.vec(dungeon + 504, 24))
        ordinals = self.u(dungeon + 544)
        if len(base_desc) != len(base_rooms) or len(base_desc) != len(base_actors):
            raise ValueError('room cache vector sizes disagree')
        current['base_rooms'] = [dict(self.descriptor(desc), index=i,
            layer_ordinal=self.u(ordinals + i * 4, 4), room=hex(self.u(base_rooms[i])),
            entities=self.actor_slot(base_actors[i])) for i, desc in enumerate(base_desc)]
        # 145B2ECC0: tree node +32 grid index, +40 layered-room cache.
        sentinel = self.u(dungeon + 528)
        pending = [self.u(sentinel + 8)]
        seen = set()
        layers = []
        while pending:
            node = pending.pop()
            if node in seen or self.u(node + 25, 1):
                continue
            if len(seen) >= 256:
                raise ValueError('layer tree too large')
            seen.add(node)
            pending.extend([self.u(node), self.u(node + 16)])
            cache = self.u(node + 40)
            if not cache:
                continue
            descs = list(self.vec(cache, 64))
            rooms = list(self.vec(cache + 24, 8))
            actors = list(self.vec(cache + 48, 24))
            if len(descs) != len(rooms) or len(descs) != len(actors):
                raise ValueError('layer cache vector sizes disagree')
            layers.append({'grid_index': self.u(node + 32, 4),
                'rooms': [dict(self.descriptor(desc), ordinal=i, room=hex(self.u(rooms[i])),
                    entities=self.actor_slot(actors[i])) for i, desc in enumerate(descs)]})
        current['layer_rooms'] = layers
        return current


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument('--pid', type=int, required=True)
    parser.add_argument('--output', type=pathlib.Path, required=True)
    parser.add_argument('--samples', type=int, default=3)
    args = parser.parse_args()
    runtime = (ROOT / 'server/work/dfo-lan/runtime').resolve()
    output = args.output.resolve()
    if not output.is_relative_to(runtime) or not 1 <= args.samples <= 20:
        parser.error('output must be within runtime; samples must be 1..20')
    output.parent.mkdir(parents=True, exist_ok=True)
    try:
        reader = Reader(args.pid)
    except Exception as error:
        with output.open('a', encoding='utf-8') as stream:
            stream.write(json.dumps({'capture_error': str(error), 'pid': args.pid, 'read_only': True}) + '\n')
        raise
    try:
        with output.open('a', encoding='utf-8') as stream:
            for i in range(args.samples):
                try:
                    row = reader.snapshot()
                except (OSError, ValueError) as error:
                    row = {'read_retry': str(error)}
                row.update(time=datetime.datetime.now(datetime.timezone.utc).isoformat(), pid=args.pid, read_only=True)
                stream.write(json.dumps(row) + '\n')
                stream.flush()
                if i + 1 < args.samples:
                    time.sleep(0.5)
        print(json.dumps({'output': str(output), 'samples': args.samples, 'read_only': True}))
    finally:
        reader.close()


if __name__ == '__main__':
    main()

"""解析导出规则所需的 CTP 记录及脚本字段。"""
import math
import struct


def read_reward_ctp(raw, allow_cost_values=False):
    """复用 pvf/ctp.go 已核实的容器布局，仅接受奖励表使用的名称和数值单元。"""
    version, count = struct.unpack_from('<II', raw)
    cell_end = struct.unpack_from('<I', raw, 20)[0] + 4
    if version != 1 or count > 10000 or not 36 <= cell_end < len(raw):
        raise ValueError('奖励 CTP 头部无效')
    pool_start = raw.index(b'[', cell_end)
    pool = raw[pool_start:]

    def name(lo, hi):
        lo, hi = sorted((lo, hi))
        if hi > len(pool):
            raise ValueError('奖励 CTP 名称越界')
        value = pool[lo:hi].decode('ascii')
        if any(not 32 <= ord(c) < 127 for c in value):
            raise ValueError('奖励 CTP 名称异常')
        return value

    offset = 36
    records = []
    for index in range(count):
        lo, hi, flags, parent, ncells, nrefs = struct.unpack_from('<QQIqQQ', raw, offset)
        record_name = name(lo, hi)
        offset += 44
        if not -1 <= parent < count or ncells > 10000 or nrefs > count:
            raise ValueError('奖励 CTP 记录无效')
        cells, refs = [], {}
        for _ in range(ncells):
            tag = struct.unpack_from('<I', raw, offset)[0]
            offset += 4
            if tag in (4, 5):
                value = struct.unpack_from('<d', raw, offset)[0]
                if not math.isfinite(value) or not allow_cost_values and (not value.is_integer() or not 0 <= value <= 4294967295):
                    raise ValueError('奖励 CTP 数值不是有效整数')
                cells.append(int(value) if value.is_integer() else value)
                offset += 8
            elif tag == 1:
                cells.append(name(*struct.unpack_from('<QQ', raw, offset)))
                offset += 16
            elif tag == 3 and allow_cost_values:
                cells.append(raw[offset])
                offset += 1
            else:
                raise ValueError('奖励 CTP 单元类型尚未核实：' + str(tag))
        for _ in range(nrefs):
            lo, hi, length = struct.unpack_from('<QQQ', raw, offset)
            offset += 24
            if length > count:
                raise ValueError('奖励 CTP 引用数越界')
            values = list(struct.unpack_from('<' + 'Q' * length, raw, offset))
            offset += 8 * length
            key = name(lo, hi)
            if key in refs or any(v >= count for v in values):
                raise ValueError('奖励 CTP 引用重复或越界')
            refs[key] = values
        records.append({'index': index, 'name': record_name,
                        'parent': parent, 'cells': cells, 'refs': refs})
    if offset != cell_end:
        raise ValueError('奖励 CTP 记录区边界不匹配')
    for record in records:
        for key, children in record['refs'].items():
            if any(records[i]['name'] != key or records[i]['parent'] != record['index'] for i in children):
                raise ValueError('奖励 CTP 子记录归属不符')
    while offset < pool_start and any(raw[offset:pool_start]):
        lo, hi, length = struct.unpack_from('<QQQ', raw, offset)
        offset += 24
        if length > count or offset + length * 8 > pool_start:
            raise ValueError('奖励 CTP 列索引越界')
        indices = struct.unpack_from('<' + 'Q' * length, raw, offset)
        offset += length * 8
        if any(i >= count or records[i]['name'] != name(lo, hi) for i in indices):
            raise ValueError('奖励 CTP 列索引不符')
    return records


def fields(tokens):
    result = {}
    tag = ''
    for typ, value in tokens:
        if typ == 3:
            tag = value
            result.setdefault(tag, [])
        else:
            result.setdefault(tag, []).append(value)
    return result

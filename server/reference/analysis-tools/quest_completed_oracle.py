"""Original NOTI342 count/list and bitmap construction; private emulator only."""
from dungeon_map_oracle import PacketOracle
from unicorn.x86_const import *
import struct,json,pathlib

class CompletedOracle(PacketOracle):
 def __init__(self,payload):
  super().__init__(payload,0x1452c9b50,0x1452c9c3c)
  self.bitmap=None
 def step(self,u,addr,size,_):
  ins=next(self.md.disasm(bytes(u.mem_read(addr,size)),addr))
  if ins.mnemonic=='call':
   if ins.op_str=='0x148860800': # chkstk preserves allocation in RAX
    u.reg_write(UC_X86_REG_RIP,addr+size);return
   if ins.op_str=='0x148aa2510':
    u.mem_write(u.reg_read(UC_X86_REG_RCX),bytes([u.reg_read(UC_X86_REG_RDX)&255])*u.reg_read(UC_X86_REG_R8))
    u.reg_write(UC_X86_REG_RIP,addr+size);return
   if ins.op_str=='0x145ea5240':
    raw=bytes(u.mem_read(u.reg_read(UC_X86_REG_RDX),40000))
    self.bitmap=[i for i,v in enumerate(raw) if v]
  super().step(u,addr,size,_)

if __name__=='__main__':
 ids=[3145,39999];payload=struct.pack('<3I',len(ids),*ids)
 o=CompletedOracle(payload);result=o.parse();assert o.bitmap==ids
 result['completed_ids']=o.bitmap
 result['scope']='original current NOTI342 list reader and bitmap writes; stopped after original bitmap setter call, later quest/UI traversal not emulated'
 try:CompletedOracle(payload[:-1]).parse()
 except ValueError as e:result['truncation_failure']=str(e)
 else:raise AssertionError('truncated completed list accepted')
 dest=pathlib.Path(__file__).parent.parent/'dfo-lan/internal/game/protocol/testdata/native_completed_quests.json'
 dest.write_text(json.dumps(result,indent=2),encoding='utf-8')
 print('completed quest bitmap',o.bitmap,'consumed',result['consumed'])

"""Read only the selected-actor map in the owned, isolated DFO probe process."""
import ctypes as c, ctypes.wintypes as w, pathlib, sys, json, struct, datetime
pid=int(sys.argv[1]); ident=int(sys.argv[2]); dest=pathlib.Path(sys.argv[3])
if not 0<ident<65535: raise ValueError('invalid actor ID')
k=c.WinDLL('kernel32',use_last_error=True); ps=c.WinDLL('psapi',use_last_error=True)
k.OpenProcess.argtypes=[w.DWORD,w.BOOL,w.DWORD]; k.OpenProcess.restype=w.HANDLE
k.QueryFullProcessImageNameW.argtypes=[w.HANDLE,w.DWORD,w.LPWSTR,c.POINTER(w.DWORD)]
k.ReadProcessMemory.argtypes=[w.HANDLE,c.c_void_p,c.c_void_p,c.c_size_t,c.POINTER(c.c_size_t)]
k.CloseHandle.argtypes=[w.HANDLE]
ps.EnumProcessModules.argtypes=[w.HANDLE,c.c_void_p,w.DWORD,c.POINTER(w.DWORD)]
h=k.OpenProcess(0x410,False,pid)
if not h: raise c.WinError(c.get_last_error())
try:
 name=c.create_unicode_buffer(32768); size=w.DWORD(len(name))
 if not k.QueryFullProcessImageNameW(h,0,name,c.byref(size)):raise c.WinError(c.get_last_error())
 expected=pathlib.Path(__file__).resolve().parent.parent/'dfo_probe_client/DFO.exe'
 if pathlib.Path(name.value).resolve()!=expected.resolve():raise ValueError('process is not the owned isolated DFO client')
 modules=(c.c_void_p*1024)(); needed=w.DWORD()
 if not ps.EnumProcessModules(h,modules,c.sizeof(modules),c.byref(needed)):raise c.WinError(c.get_last_error())
 base=modules[0];delta=base-0x140000000
 def read(addr,n):
  if not 0<n<=2048 or addr<65536:raise ValueError('invalid bounded read')
  buf=c.create_string_buffer(n); got=c.c_size_t()
  if not k.ReadProcessMemory(h,addr,buf,n,c.byref(got)) or got.value!=n:raise c.WinError(c.get_last_error())
  return buf.raw
 def u(addr,n=8):return int.from_bytes(read(addr,n),'little')
 def globalu(addr,n=8):return u(addr+delta,n)
 def protected(addr):return ((globalu(addr,4)^0x1f2a025c)-4)&0xffffffff
 manager=globalu(0x14e683c08)
 hv=0xcbf29ce484222325
 for v in struct.pack('<H',ident):hv=((hv^v)*0x100000001b3)&0xffffffffffffffff
 sentinel=u(manager+0x10);bucket=u(manager+0x20)+(hv&u(manager+0x38))*16
 node=u(bucket+8);first=u(bucket);actor=0
 for _ in range(128):
  if node==sentinel:break
  if u(node+0x10,2)==ident:
   control=u(node+0x20)
   if control and u(control+8,4):actor=u(node+0x28)
   break
  if node==first:break
  node=u(node+8)
 result={'time':datetime.datetime.now(datetime.timezone.utc).isoformat(),'pid':pid,'module_base':hex(base),'actor_server_id':ident,'client_my_server_id':protected(0x14ef2ca00),'client_context':[protected(0x14e682a60),protected(0x14e682a58)],'last_userinfo_context':[globalu(0x14e66f231,1),globalu(0x14e66f230,1)],'actor_found':bool(actor),'actor_pointer':hex(actor)}
 result['party_slots']=[]
 party_manager=globalu(0x14e683c20)
 for party_slot in range(8) if party_manager else ():
  control=u(party_manager+0x88+party_slot*24);pointer=u(party_manager+0x90+party_slot*24)
  try:
   if control and u(control+8,4) and pointer:
    result['party_slots'].append({'slot':party_slot,'pointer':hex(pointer),'is_local_actor':pointer==actor})
  except (OSError,ValueError) as error:
   result['party_slots'].append({'slot':party_slot,'read_error':str(error),'control':hex(control)})
 channel_config=globalu(0x1451fa240+7+0x943a1e1)
 if channel_config:
  address_ptr=channel_config+0x10 if u(channel_config+0x28)<8 else u(channel_config+0x10)
  address_len=u(channel_config+0x20)
  result['channel_refresh_config']={'enabled':u(channel_config+8,1),'region':u(channel_config+0x88,4),'host':read(address_ptr,address_len*2).decode('utf-16le') if 0<address_len<256 else '', 'port':u(channel_config+0x30,4)}
 game=globalu(0x14e66c090)
 root_scene=globalu(0x14e6839f0)
 map_obj=0
 if root_scene:
  result['root_scene']={'pointer':hex(root_scene),'vtable':hex(u(root_scene)),'scene_getter':hex(u(u(root_scene)+0xd8))}
  if u(u(root_scene)+0xd8)==0x144ed9c60+delta:
   scene=u(root_scene+0x90);map_obj=u(scene+0xb0) if scene else 0
   result['dungeon_scene']={'pointer':hex(scene),'map':hex(map_obj),'gates':[]}
   if scene:
    result['dungeon_scene']['mode_bytes']=list(read(scene+0xb48,12))
  if map_obj:
    result['dungeon_scene']['move_lock']=u(map_obj+0x3ec0,1)
    result['dungeon_scene']['move_timer_flags']=u(map_obj+0x3f34,4)
    for direction in range(4):
     slot=map_obj+(direction+0x279)*24;control=u(slot+8);ptr=u(slot+16)
     if not control or not u(control+8,4) or not ptr:continue
     gate=ptr-0x30;owner_control=u(gate+0x190);owner=u(gate+0x198)
     row={'direction':direction,'pointer':hex(gate),'active_flags':u(gate+0x15c,4),'open':u(gate+0x2e59,1),'field2e90':u(gate+0x2e90,1),'target':[u(gate+0x2eac,4),u(gate+0x2eb0,4)]}
     if owner_control and u(owner_control+8,4) and owner:
      row['owner']=hex(owner);row['owner_room_clear']=((u(owner+0x29c,1)^0x1a)-1)&255
     result['dungeon_scene']['gates'].append(row)
 if game:
  result['game_inventory_ready_1469']=u(game+0x1469,1)
  result['game_state_fallback']=((u(game+0x104,4)^0x1f2a025c)-4)&0xffffffff
  world=u(game+0x118)
  result['town_map_object']=hex(world)
  if world:
   result['map_town_id']=u(world+0x280,4)
   area=u(world+0x298)
   result['map_area_id']=u(area,4) if area else -1
   result['map_area_index']=u(world+0x2a0,4)
   result['previous_map']=[u(world+0x2a4,4),u(world+0x2a8,4)]
   result['transition_lock']=u(world+0x501,1)
   result['area_warp_flags']=[u(world+0x488,1),u(world+0x489,1)]
 row=read(0x14e66c390+delta,12)
 result['last_area_users_row']=dict(zip(['actor_id','x','y','flag_a','flag_b','town_id','area_id'],struct.unpack('<HHHBBHH',row)))
 if actor:
  result['actor_vtable']=hex(u(actor))
  info=actor+0x80
  result['profession']=u(info+0x10,4)
  result['actor_info_scalar_2c']=u(info+0x2c,4)
  result['advancement']=u(info+0x14,4)
  result['level']=((u(info+0x24,4)^0x1f2a025c)-4)&0xffffffff
  scene_control=u(actor+0x808);scene=u(actor+0x810)
  result['scene_character_found']=bool(scene_control and u(scene_control+8,4) and scene)
  if result['scene_character_found']:result['scene_character_pointer']=hex(scene-0x30)
  control=u(actor+0x820);virtual=u(actor+0x828)
  result['virtual_character_found']=bool(control and u(control+8,4) and virtual)
  if result['virtual_character_found']:
   character=virtual-0x30; vtable=u(character)
   result['character_pointer']=hex(character)
   result['character_vtable']=hex(vtable)
   result['skill_profession']=u(character+0x75e8,4)
   result['skill_tree_index']=u(character+0x10d20,4)
   result['skill_getter']=hex(u(vtable+0x2048))
   result['instantiated_skills']=[]
   for tree in range(2):
    for skill_id in (3,7,46,169,174,179,190,452):
     entry=character+0xad20+(tree*512+skill_id)*24
     control=u(entry+8);pointer=u(entry+16)
     result['instantiated_skills'].append({'tree':tree,'id':skill_id,'control':hex(control),'pointer':hex(pointer),'valid':bool(control and pointer and u(control+8,4))})
   result['packed_addition_setter']=hex(u(vtable+0x1e30))
   result['entry_stats_words']=[((u(character+0x10e10+i*8,4)^0x1f2a025c)-4)&0xffffffff for i in range(40)]
   result['effective_base_attributes']={key:((u(character+off,4)^0x1f2a025c)-4)&0xffffffff for key,off in [('movement',0x2648),('attack',0x2668),('casting',0x2680),('recovery',0x26b0),('jump',0x26d0),('weight',0x26f0)]}
 option_profile=globalu(0x14e634230)
 if option_profile:
  option_data=u(option_profile+0x60)
  if option_data:
   result['account_options']={'source_defaults_ready':u(option_data+0x6419,1),'account_received':u(option_profile+0xc,1),'account_guide_value':u(option_data+0x1ce0+2+198*2,2),'account_guide_set':u(option_data+0x1ce0+574+198,1),'merged_guide_value':u(option_data+0x38f3+2+198*2,2),'merged_guide_set':u(option_data+0x38f3+574+198,1)}
 result['fatigue_used']=protected(0x14ef2ca28)
 transition=globalu(0x144652334+7+0xa005d7d)
 if transition:result['transition_stage']=u(transition+0x58,4)
 # Read-only exact-build dungeon factory lookup14520cfb0 for Mirkwood3.
 hval=0xcbf29ce484222325
 for value in struct.pack('<I',3):hval=((hval^value)*0x100000001b3)&0xffffffffffffffff
 sentinel=globalu(0x14520cfe7+7+0x945eb6a)
 bucket=globalu(0x14520d019+7+0x945eb48)+(hval&globalu(0x14520d007+7+0x945eb72))*16
 node=u(bucket+8);first=u(bucket)
 for _ in range(128):
  if node==sentinel:break
  if u(node+0x10,4)==3:
   dungeon=u(node+0x18)
   result['dungeon3']={'pointer':hex(dungeon),'vtable':hex(u(dungeon)),'select_maze_function':hex(u(u(dungeon)+0x20)), 'fields':{hex(off):((u(dungeon+off,4)^0x1f2a025c)-4)&0xffffffff for off in range(0x191c,0x1955,8)}}
   dimensions=u(dungeon+0x730);dim_end=u(dungeon+0x738)
   result['dungeon3']['maze_dimensions']=[list(struct.unpack('<II',read(dimensions+i*8,8))) for i in range(min(32,(dim_end-dimensions)//8))]
   grid=u(dungeon+0x1a8);grid_end=u(dungeon+0x1b0)
   result['dungeon3']['grid_words']=[list(struct.unpack('<16I',read(grid+i*64,64))) for i in range(min(16,(grid_end-grid)//64))]
   break
  if node==first:break
  node=u(node+8)
 result['fatigue_limit']=protected(0x14ef2ca30)
 dest.parent.mkdir(parents=True,exist_ok=True);dest.write_text(json.dumps(result,indent=2))
 print(json.dumps(result,indent=2))
finally:k.CloseHandle(h)

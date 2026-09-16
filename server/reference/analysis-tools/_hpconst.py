import contextlib,io,runpy,pathlib,struct
p=pathlib.Path(__file__).parent
with contextlib.redirect_stdout(io.StringIO()): e=runpy.run_path(str(p/'decode_literals.py'))
pe,base=e['pe'],e['base']
def rd(va,n):
    return pe.get_data(va-base,n)
def dbl(va): return struct.unpack('<d',rd(va,8))[0]
def flt(va): return struct.unpack('<f',rd(va,4))[0]
# getter 147220e40 constants: compute rip-relative targets
# 147220f34 addsd xmm0,[rip+0x48d6fe4] next=147220f3c
C1=0x147220f3c+0x48d6fe4
# 147220f40 comisd xmm6,[rip+0x48d6fd8] next=147220f48
C2=0x147220f48+0x48d6fd8
# 147220fe6 maxsd xmm0,[rip+0x48d6f32] next=147220fee
C3=0x147220fee+0x48d6f32
print("getter constants (double):")
print(f"  C1 @{C1:x} = {dbl(C1)}")
print(f"  C2 @{C2:x} = {dbl(C2)}")
print(f"  C3 @{C3:x} = {dbl(C3)}")
# 145c08420 K: 145c08552 movss xmm2,[rip+0x35b646e] next=145c0855a
K=0x145c0855a+0x35b646e
print("145c08420 K (float):")
print(f"  K  @{K:x} = {flt(K)}")
# 0.0143 as float bits
print("0.0143 float bits:", hex(struct.unpack('<I',struct.pack('<f',0.0143))[0]))
print("1/70 =", 1/70)

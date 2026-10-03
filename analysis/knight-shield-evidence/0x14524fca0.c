void __fastcall sub_14524FCA0(__int64 a1, char a2)
{
  __int64 v2; // rax
  __int64 v3; // rdi
  __int64 v4; // rax
  int v5; // ebx
  __int64 v6; // rsi
  _DWORD v7[4]; // [rsp+30h] [rbp-18h] BYREF
  char v8; // [rsp+58h] [rbp+10h] BYREF

  if ( a2 != 0 ) /*0x14524fca6*/
  {
    v2 = sub_145EFAFB0(); /*0x14524fcb1*/
    v3 = sub_1450BE540(a1: v2); /*0x14524fcbe*/
    if ( v3 != 0 ) /*0x14524fcc4*/
    {
      v4 = sub_14667EB40(a1: *(_QWORD *)&qword_14E683C78, a2: 1495); /*0x14524fce0*/
      v5 = 0; /*0x14524fce5*/
      v6 = _RTDynamicCast(a1: v4, a2: 0, a3: &off_14DCB4760, a4: &off_14DDA8180, a5: 0); /*0x14524fd03*/
      if ( v6 != 0 ) /*0x14524fd09*/
      {
        sub_146EA09F0(a1: &v8, a2: 1); /*0x14524fd13*/
        do /*0x14524fd4d*/
        {
          sub_146EA0BA0(a1: v7); /*0x14524fd25*/
          sub_144BE6710(a1: v3, a2: (unsigned int)v5, a3: v7[0]); /*0x14524fd34*/
          sub_14505E1C0(a1: v6, a2: (unsigned int)v5++, a3: v7[0]); /*0x14524fd43*/
        }
        while ( v5 < 5 ); /*0x14524fd4d*/
      }
    }
  }
}

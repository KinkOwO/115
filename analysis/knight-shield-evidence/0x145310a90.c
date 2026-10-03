__int64 sub_145310A90()
{
  __int64 v0; // rax
  __int64 result; // rax
  __int64 v2; // rdi
  __int64 v3; // rax
  int v4; // ebx
  __int64 v5; // rsi
  unsigned int v6; // [rsp+50h] [rbp+18h] BYREF

  v0 = sub_145EFAFB0(); /*0x145310a96*/
  result = sub_1450BE540(a1: v0); /*0x145310a9e*/
  v2 = result; /*0x145310aa3*/
  if ( result != 0 ) /*0x145310aa9*/
  {
    v3 = sub_14667EB40(a1: *(_QWORD *)&qword_14E683C78, a2: 1495); /*0x145310ac1*/
    v4 = 0; /*0x145310ac6*/
    result = _RTDynamicCast(a1: v3, a2: 0, a3: &off_14DCB4760, a4: &off_14DDA8180, a5: 0); /*0x145310adf*/
    v5 = result; /*0x145310ae4*/
    if ( result != 0 ) /*0x145310aea*/
    {
      do /*0x145310b1d*/
      {
        sub_146EA0BA0(a1: &v6); /*0x145310af5*/
        sub_144BE6710(a1: v2, a2: (unsigned int)v4, a3: v6); /*0x145310b04*/
        result = sub_14505E1C0(a1: v5, a2: (unsigned int)v4++, a3: v6); /*0x145310b13*/
      }
      while ( v4 < 5 ); /*0x145310b1d*/
    }
  }
  return result; /*0x145310b29*/
}

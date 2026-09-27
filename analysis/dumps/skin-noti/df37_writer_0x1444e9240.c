// writer_0x1444e9240

unsigned __int64 __fastcall sub_1444E9240(__int64 a1, int a2)
{
  _QWORD *v2; // rcx
  _DWORD *v4; // rdx
  unsigned __int64 result; // rax
  int v6; // [rsp+38h] [rbp+10h] BYREF

  v6 = a2;
  v2 = (_QWORD *)(a1 + 1128);
  v4 = (_DWORD *)v2[1];
  result = ((__int64)v4 - *v2) >> 2;
  if ( result < 0xA )
  {
    if ( v4 == (_DWORD *)v2[2] )
    {
      return sub_140154010(v2, v4, &v6);
    }
    else
    {
      *v4 = a2;
      v2[1] += 4LL;
    }
  }
  return result;
}


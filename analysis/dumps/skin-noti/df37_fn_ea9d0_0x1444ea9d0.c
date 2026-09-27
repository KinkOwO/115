// fn_ea9d0_0x1444ea9d0

__int64 __fastcall sub_1444EA9D0(__int64 a1, unsigned int a2)
{
  unsigned int v2; // edi
  unsigned __int64 v3; // rbx
  __int64 v4; // rcx
  unsigned __int64 v5; // rdx
  __int128 v7; // [rsp+30h] [rbp-28h] BYREF
  __int64 v8; // [rsp+40h] [rbp-18h]

  sub_1444EBCC0(a1, &v7, a2, 1);
  v2 = 30000;
  if ( (_QWORD)v7 != *((_QWORD *)&v7 + 1) )
  {
    v3 = (__int64)(*((_QWORD *)&v7 + 1) - v7) >> 2;
    v2 = *(_DWORD *)(v7 + 4LL * (int)((int)sub_146E9BA90() % v3));
  }
  v4 = v7;
  if ( (_QWORD)v7 )
  {
    v5 = (v8 - v7) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v5 >= 0x1000 )
    {
      v5 += 39LL;
      v4 = *(_QWORD *)(v7 - 8);
      if ( (unsigned __int64)(v7 - v4 - 8) > 0x1F )
        sub_148AAF304(v4, v5);
    }
    sub_146E9F3A0(v4, v5);
    v7 = 0;
    v8 = 0;
  }
  return v2;
}


// fn_ea8a0_0x1444ea8a0

__int64 __fastcall sub_1444EA8A0(__int64 a1)
{
  unsigned __int64 v2; // rdi
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  unsigned int v5; // ebx
  __int64 v6; // rcx
  unsigned __int64 v7; // rdx
  __int128 v9; // [rsp+30h] [rbp-28h] BYREF
  __int64 v10; // [rsp+40h] [rbp-18h]

  sub_1444EBC10(a1, &v9, 1);
  *(_DWORD *)(a1 + 1120) = 30000;
  if ( (_QWORD)v9 != *((_QWORD *)&v9 + 1) )
  {
    v2 = (__int64)(*((_QWORD *)&v9 + 1) - v9) >> 2;
    *(_DWORD *)(a1 + 1120) = *(_DWORD *)(v9 + 4LL * (int)((int)sub_146E9BA90() % v2));
  }
  if ( !qword_14E634518 )
  {
    v3 = sub_146E8BA20(2008);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_143C9DEC0(v3);
    else
      v4 = 0;
    qword_14E634518 = (__int64)v4;
    (**v4)(v4);
  }
  if ( (unsigned __int8)sub_143CA0E80() )
    *(_DWORD *)(a1 + 1120) = 30000;
  v5 = *(_DWORD *)(a1 + 1120);
  v6 = v9;
  if ( (_QWORD)v9 )
  {
    v7 = (v10 - v9) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v7 >= 0x1000 )
    {
      v7 += 39LL;
      v6 = *(_QWORD *)(v9 - 8);
      if ( (unsigned __int64)(v9 - v6 - 8) > 0x1F )
        sub_148AAF304(v6, v7);
    }
    sub_146E9F3A0(v6, v7);
    v9 = 0;
    v10 = 0;
  }
  return v5;
}


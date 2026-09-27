// ebc10caller_sub_1444EC5B0_0x1444ec5b0

__int64 __fastcall sub_1444EC5B0(__int64 a1, int a2, int a3)
{
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  unsigned __int8 v8; // di
  _DWORD *v9; // rax
  __int64 v10; // rcx
  unsigned __int64 v11; // rdx
  __int128 v13; // [rsp+30h] [rbp-28h] BYREF
  __int64 v14; // [rsp+40h] [rbp-18h]

  v5 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v6 = sub_146E8BA20(1472);
    if ( v6 )
      v7 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v6);
    else
      v7 = 0;
    qword_14E638F28 = (__int64)v7;
    (**v7)(v7);
    v5 = qword_14E638F28;
  }
  sub_1444EBC10(v5, &v13, a2);
  v8 = 0;
  v9 = (_DWORD *)v13;
  if ( (_QWORD)v13 != *((_QWORD *)&v13 + 1) )
  {
    while ( a3 != *v9 )
    {
      if ( ++v9 == *((_DWORD **)&v13 + 1) )
        goto LABEL_11;
    }
    v8 = 1;
  }
LABEL_11:
  v10 = v13;
  if ( (_QWORD)v13 )
  {
    v11 = (v14 - v13) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v11 >= 0x1000 )
    {
      v11 += 39LL;
      v10 = *(_QWORD *)(v13 - 8);
      if ( (unsigned __int64)(v13 - v10 - 8) > 0x1F )
        sub_148AAF304(v10, v11);
    }
    sub_146E9F3A0(v10, v11);
    v13 = 0;
    v14 = 0;
  }
  return v8;
}


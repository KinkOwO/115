// elemmethod_sub_1441D97A0_0x1441d97a0

__int64 __fastcall sub_1441D97A0(_DWORD *a1)
{
  __int64 v2; // rdx
  __int64 v3; // rcx
  __int64 v4; // rax
  void (__fastcall ***v5)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v7; // rcx
  unsigned __int64 v8; // rdx
  __int128 v9; // [rsp+28h] [rbp-20h] BYREF
  __int64 v10; // [rsp+38h] [rbp-10h]

  a1[32] = -1;
  (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)a1 + 104LL))(a1);
  (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)a1 + 24LL))(a1);
  LOBYTE(v2) = 1;
  (*(void (__fastcall **)(_DWORD *, __int64))(*(_QWORD *)a1 + 32LL))(a1, v2);
  (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)a1 + 120LL))(a1);
  v3 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v4 = sub_146E8BA20(1472);
    if ( v4 )
      v5 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v4);
    else
      v5 = 0;
    qword_14E638F28 = (__int64)v5;
    (**v5)(v5);
    v3 = qword_14E638F28;
  }
  sub_1444EBC10(v3, &v9, 4);
  result = (__int64)(*((_QWORD *)&v9 + 1) - v9) >> 2;
  if ( result )
    result = sub_1441DB4D0(a1, *(unsigned int *)v9);
  v7 = v9;
  if ( (_QWORD)v9 )
  {
    v8 = (v10 - v9) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v8 >= 0x1000 )
    {
      v8 += 39LL;
      v7 = *(_QWORD *)(v9 - 8);
      if ( (unsigned __int64)(v9 - v7 - 8) > 0x1F )
        sub_148AAF304(v7, v8);
    }
    result = sub_146E9F3A0(v7, v8);
    v9 = 0;
    v10 = 0;
  }
  return result;
}


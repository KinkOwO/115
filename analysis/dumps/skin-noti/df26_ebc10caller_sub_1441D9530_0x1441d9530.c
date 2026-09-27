// ebc10caller_sub_1441D9530_0x1441d9530

__int128 *__fastcall sub_1441D9530(_QWORD *a1)
{
  __int64 v2; // rdx
  __int128 *result; // rax
  __int64 v4; // rcx
  __int64 v5; // rbx
  __int64 v6; // rcx
  __int64 v7; // rax
  void (__fastcall ***v8)(_QWORD); // rcx
  __int64 v9; // rdx
  _DWORD *v10; // rdi
  _DWORD *v11; // r14
  _DWORD *v12; // rbp
  __int64 v13; // rcx
  unsigned __int64 v14; // rdx
  __int64 v15; // rcx
  unsigned __int64 v16; // rbx
  __int64 v17; // rcx
  __int128 v18; // [rsp+28h] [rbp-50h] BYREF
  __int64 v19; // [rsp+38h] [rbp-40h]
  __int64 v20; // [rsp+40h] [rbp-38h] BYREF
  __int128 v21; // [rsp+48h] [rbp-30h]

  *((_DWORD *)a1 + 32) = -1;
  (*(void (__fastcall **)(_QWORD *))(*a1 + 104LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 24LL))(a1);
  LOBYTE(v2) = 1;
  (*(void (__fastcall **)(_QWORD *, __int64))(*a1 + 32LL))(a1, v2);
  result = *(__int128 **)(a1[1] + 3520LL);
  if ( *(_DWORD *)result == -1 )
  {
    v4 = a1[19];
    if ( v4 )
      return (__int128 *)(*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v4 + 24LL))(v4, 0);
  }
  else
  {
    v18 = 0;
    v5 = 0;
    v19 = 0;
    v6 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v7 = sub_146E8BA20(1472);
      if ( v7 )
        v8 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v7);
      else
        v8 = 0;
      qword_14E638F28 = (__int64)v8;
      (**v8)(v8);
      v6 = qword_14E638F28;
    }
    result = (__int128 *)sub_1444EBC10(v6, &v20, 7);
    v10 = 0;
    if ( &v18 == result )
    {
      v12 = (_DWORD *)*((_QWORD *)&v18 + 1);
      v11 = (_DWORD *)v18;
    }
    else
    {
      v10 = *(_DWORD **)result;
      v11 = *(_DWORD **)result;
      *(_QWORD *)&v18 = *(_QWORD *)result;
      v12 = (_DWORD *)*((_QWORD *)result + 1);
      *((_QWORD *)&v18 + 1) = v12;
      v5 = *((_QWORD *)result + 2);
      v19 = v5;
      *(_QWORD *)result = 0;
      *((_QWORD *)result + 1) = 0;
      *((_QWORD *)result + 2) = 0;
    }
    v13 = v20;
    if ( v20 )
    {
      v14 = 4 * ((*((_QWORD *)&v21 + 1) - v20) >> 2);
      if ( v14 >= 0x1000 )
      {
        v14 += 39LL;
        v13 = *(_QWORD *)(v20 - 8);
        if ( (unsigned __int64)(v20 - v13 - 8) > 0x1F )
          sub_148AAF304(v13, v14);
      }
      result = (__int128 *)sub_146E9F3A0(v13, v14);
      v20 = 0;
      v21 = 0;
    }
    LOBYTE(v9) = v10 == v12 || (result = *(__int128 **)(a1[1] + 3520LL), *v11 != *(_DWORD *)result);
    v15 = a1[19];
    if ( v15 )
      result = (__int128 *)(*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v15 + 24LL))(v15, v9);
    if ( v10 )
    {
      v16 = (v5 - (_QWORD)v10) & 0xFFFFFFFFFFFFFFFCuLL;
      if ( v16 >= 0x1000 )
      {
        v16 += 39LL;
        v17 = *((_QWORD *)v10 - 1);
        if ( (unsigned __int64)v10 - v17 - 8 > 0x1F )
          sub_148AAF304(v17, v9);
        v10 = (_DWORD *)*((_QWORD *)v10 - 1);
      }
      result = (__int128 *)sub_146E9F3A0(v10, v16);
      v18 = 0;
      v19 = 0;
    }
  }
  return result;
}


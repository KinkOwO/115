// caller_f1090_0x1441e03d0

__int64 __fastcall sub_1441E03D0(__int64 a1)
{
  _DWORD *v1; // r14
  _DWORD *v2; // rdi
  __int64 v3; // rsi
  _DWORD *v4; // rbx
  __int64 v5; // r12
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  __int64 v8; // r15
  __int64 v9; // rbx
  __int64 v10; // rsi
  __int64 v11; // rdi
  __int64 v12; // rcx
  __int64 v13; // rax
  void (__fastcall ***v14)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v16; // rdx
  __int64 v17; // rcx
  __int64 v18; // r8
  int v19; // eax
  unsigned __int64 v20; // r14
  __int64 v21; // rax
  __int128 v22; // [rsp+38h] [rbp-38h] BYREF
  __int64 v23; // [rsp+48h] [rbp-28h]
  __int128 v24; // [rsp+50h] [rbp-20h] BYREF
  _DWORD *v25; // [rsp+60h] [rbp-10h]

  v24 = 0;
  v1 = 0;
  v25 = 0;
  v2 = (_DWORD *)(a1 + 4176);
  v3 = 4;
  v4 = 0;
  do
  {
    if ( v4 == v1 )
    {
      sub_140154010(&v24, v4, v2);
      v1 = v25;
      v4 = (_DWORD *)*((_QWORD *)&v24 + 1);
    }
    else
    {
      *v4++ = *v2;
      *((_QWORD *)&v24 + 1) = v4;
    }
    v2 += 30;
    --v3;
  }
  while ( v3 );
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
  v22 = 0;
  v23 = 0;
  v8 = v24;
  if ( (_DWORD *)v24 != v4 )
  {
    v9 = (__int64)v4 - v24;
    *(_QWORD *)&v22 = sub_140157580(&v22, v9 >> 2);
    *((_QWORD *)&v22 + 1) = v22;
    v10 = 4 * (v9 >> 2);
    v23 = v10 + v22;
    v11 = v22;
    sub_148AA1E60(v22, v8, v9);
    *((_QWORD *)&v22 + 1) = v10 + v11;
  }
  sub_1444F1090(v5, 3, &v22);
  v12 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v13 = sub_146E8BA20(112);
    if ( v13 )
      v14 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v13);
    else
      v14 = 0;
    qword_14E634230 = (__int64)v14;
    (**v14)(v14);
    v12 = qword_14E634230;
  }
  result = sub_1403F49B0(v12, 111);
  if ( (_DWORD)result == 134 )
  {
    v17 = qword_14E683C78;
    if ( qword_14E683C78 )
    {
      LOBYTE(v18) = 1;
      result = sub_146682140(qword_14E683C78, 733, v18);
      if ( !(_BYTE)result )
      {
        v19 = sub_14668C520(qword_14E683C78, 733, 0, 0);
        result = sub_148AA307C(v19, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DCB6A70, 0);
        if ( result )
        {
          *(_DWORD *)(result + 1512) = 0;
          result = sub_1441D9D90(result);
        }
      }
    }
  }
  if ( v8 )
  {
    v20 = ((unsigned __int64)v1 - v8) & 0xFFFFFFFFFFFFFFFCuLL;
    v21 = v8;
    if ( v20 >= 0x1000 )
    {
      v20 += 39LL;
      v8 = *(_QWORD *)(v8 - 8);
      if ( (unsigned __int64)(v21 - v8 - 8) > 0x1F )
        sub_148AAF304(v17, v16);
    }
    result = sub_146E9F3A0(v8, v20);
    v24 = 0;
    v25 = 0;
  }
  return result;
}


__int64 __fastcall sub_1441DB060(__int64 a1, unsigned int a2)
{
  __int64 v4; // rax
  int v5; // r12d
  BOOL v6; // ebx
  _DWORD *v7; // r14
  _DWORD *v8; // rbx
  _DWORD *i; // rdi
  __int64 v10; // rbp
  __int64 v11; // rax
  void (__fastcall ***v12)(_QWORD); // rcx
  _QWORD *v13; // rax
  __int64 result; // rax
  __int64 v15; // rdx
  __int64 v16; // rcx
  __int64 v17; // r13
  unsigned __int64 v18; // rdi
  int v19; // ebx
  _DWORD *v20; // rbp
  __int64 v21; // r8
  int v22; // edx
  _DWORD *v23; // rax
  unsigned __int64 v24; // r14
  __int64 v25; // rax
  __int128 v26; // [rsp+28h] [rbp-50h] BYREF
  _DWORD *v27; // [rsp+38h] [rbp-40h]
  char v28; // [rsp+90h] [rbp+18h] BYREF

  v4 = sub_1444EBAB0(a2);
  v5 = 1;
  if ( v4 )
  {
    v6 = *(_DWORD *)(v4 + 12) == 3;
    sub_146F53250(*(_QWORD *)(a1 + 1456), *(_DWORD *)(v4 + 12) == 3, 0);
    *(_DWORD *)(a1 + 1448) = v6;
    sub_1441ECDB0(*(_QWORD *)(a1 + 8), 1);
    (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 24LL))(a1);
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 32LL))(a1, 1);
  }
  v26 = 0;
  v7 = 0;
  v27 = 0;
  v8 = *(_DWORD **)(a1 + 16);
  for ( i = 0; v8 != *(_DWORD **)(a1 + 24); v8 += 6 )
  {
    if ( ((unsigned int)sub_14206BB60(*(_QWORD *)(a1 + 40)) != 2 || !sub_1444EC2C0((__int64)v8))
      && ((unsigned int)sub_14206BB60(*(_QWORD *)(a1 + 40)) != 3 || sub_1444EC2C0((__int64)v8)) )
    {
      if ( (unsigned int)sub_14206BB60(*(_QWORD *)(a1 + 40)) != 1 )
        goto LABEL_15;
      v10 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        v11 = sub_146E8BA20(1472);
        if ( v11 )
          v12 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v11);
        else
          v12 = 0;
        qword_14E638F28 = (__int64)v12;
        (**v12)(v12);
        v10 = qword_14E638F28;
      }
      v13 = (_QWORD *)sub_1441C3DB0(*(_QWORD *)(a1 + 8), &v28);
      if ( (unsigned __int8)sub_1444EC320(v10, (unsigned int)*v8, *v13) )
      {
LABEL_15:
        if ( i == v7 )
        {
          sub_140154010(&v26, i, v8);
          v7 = v27;
          i = (_DWORD *)*((_QWORD *)&v26 + 1);
        }
        else
        {
          *i++ = *v8;
          *((_QWORD *)&v26 + 1) = i;
        }
      }
    }
  }
  result = sub_1421B2820(*(_QWORD *)(a1 + 112));
  v17 = v26;
  if ( (int)result >= 1 )
  {
    v18 = (__int64)((__int64)i - v26) >> 2;
    v19 = 0;
    v20 = (_DWORD *)v26;
LABEL_21:
    v21 = 0;
    v22 = v19;
    v23 = v20;
    while ( v22 >= v18 || *v23 != a2 )
    {
      ++v22;
      ++v21;
      ++v23;
      if ( v21 >= 3 )
      {
        ++v5;
        v19 += 3;
        v20 += 3;
        result = sub_1421B2820(*(_QWORD *)(a1 + 112));
        if ( v5 > (int)result )
          goto LABEL_28;
        goto LABEL_21;
      }
    }
    result = sub_146B26020(*(_QWORD *)(a1 + 112), (unsigned int)v5, v21);
  }
LABEL_28:
  if ( v17 )
  {
    v24 = ((unsigned __int64)v7 - v17) & 0xFFFFFFFFFFFFFFFCuLL;
    v25 = v17;
    if ( v24 >= 0x1000 )
    {
      v24 += 39LL;
      v17 = *(_QWORD *)(v17 - 8);
      if ( (unsigned __int64)(v25 - v17 - 8) > 0x1F )
        sub_148AAF304(v16, v15);
    }
    result = sub_146E9F3A0(v17, v24);
    v26 = 0;
    v27 = 0;
  }
  return result;
}

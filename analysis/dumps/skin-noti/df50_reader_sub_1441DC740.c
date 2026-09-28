// reader_sub_1441DC740

__int64 __fastcall sub_1441DC740(_QWORD *a1)
{
  int v2; // ebp
  char v3; // r15
  _QWORD *v4; // rdi
  __int64 v5; // rbx
  __int64 v6; // rsi
  __int64 v7; // r8
  _QWORD *v8; // rax
  bool v9; // di
  volatile signed __int32 *v10; // rbx
  __int64 v11; // r14
  _QWORD *v12; // rdi
  _QWORD *v13; // r15
  __int64 v14; // rdx
  __int64 v15; // rdx
  __int64 v16; // rdx
  __int64 v17; // rcx
  _QWORD *v18; // rbx
  __int64 v19; // rsi
  __int64 v20; // rdx
  __int64 v21; // rcx
  float v23; // [rsp+38h] [rbp-70h] BYREF
  float v24; // [rsp+3Ch] [rbp-6Ch]
  float v25[2]; // [rsp+40h] [rbp-68h] BYREF
  __int64 v26; // [rsp+48h] [rbp-60h] BYREF
  volatile signed __int32 *v27; // [rsp+50h] [rbp-58h]
  __int64 v28; // [rsp+58h] [rbp-50h]
  _DWORD v29[4]; // [rsp+60h] [rbp-48h] BYREF

  v28 = -2;
  v2 = 0;
  v3 = 0;
  if ( a1[7] && (unsigned __int8)sub_146F593F0() )
    (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 80LL))(a1);
  v4 = a1 + 27;
  v5 = (__int64)(a1 + 21);
  v6 = 3;
  do
  {
    if ( (unsigned __int8)sub_141FB6530(*v4) )
      sub_1441DC120(v5);
    v5 += 328;
    v4 += 41;
    --v6;
  }
  while ( v6 );
  if ( (unsigned __int8)sub_146EC48A0() )
  {
    sub_1467A3090(a1[1], &v23);
    sub_1467A30D0(a1[1], v25);
    v29[0] = (int)v23;
    v29[1] = (int)v24;
    v29[2] = (int)v23 + (int)v25[0];
    v29[3] = (int)v24 + (int)v25[1];
    LODWORD(v26) = sub_146EC47D0(0);
    HIDWORD(v26) = sub_146EC4820(0);
    v9 = 1;
    if ( (unsigned int)MEMORY[0xDC5DE00](v29, v26) )
    {
      v8 = (_QWORD *)(*(__int64 (__fastcall **)(_QWORD, __int64 *))(*(_QWORD *)a1[1] + 280LL))(a1[1], &v26);
      v3 = 1;
      if ( !(unsigned __int8)sub_146ED0010(*v8) )
        v9 = 0;
    }
    if ( (v3 & 1) != 0 )
    {
      v10 = v27;
      if ( v27 )
      {
        if ( _InterlockedExchangeAdd(v27 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v10)(v10);
          if ( _InterlockedExchangeAdd(v10 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v10 + 8LL))(v10);
        }
      }
    }
    if ( v9 )
    {
      LOBYTE(v7) = 1;
      if ( (unsigned __int8)sub_146682140(qword_14E683C78, 2373, v7) )
        sub_146694510(qword_14E683C78, 2373, -1, 0, 1);
    }
  }
  v11 = a1[1];
  v12 = (_QWORD *)(v11 + 2888);
  v13 = (_QWORD *)(v11 + 2824);
  do
  {
    (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v12 - 8) + 16LL))(*(v12 - 8), 0);
    if ( (unsigned __int8)sub_146ED0010(*(v12 - 10)) || (unsigned __int8)sub_146ED0010(*(v12 - 2)) )
    {
      LOBYTE(v14) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*(v12 - 2) + 16LL))(*(v12 - 2), v14);
      if ( (unsigned __int8)sub_141FB6530(*(v12 - 4)) )
        LOBYTE(v15) = 1;
      else
        v15 = 0;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v12 + 16LL))(*v12, v15);
    }
    else
    {
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*(v12 - 2) + 16LL))(*(v12 - 2), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v12 + 16LL))(*v12, 0);
    }
    if ( (unsigned __int8)sub_146ECFD90(*(v12 - 2)) )
    {
      sub_1441E4B60(v11, (unsigned int)v2);
      v17 = *(_QWORD *)(v11 + 1744);
      if ( v17 )
      {
        LOBYTE(v16) = 1;
        (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v17 + 24LL))(v17, v16);
      }
    }
    else if ( (unsigned __int8)sub_146ECFD90(*(v12 - 10)) )
    {
      v18 = (_QWORD *)(v11 + 2824);
      v19 = 4;
      do
      {
        (*(void (__fastcall **)(_QWORD, _QWORD))(*(_QWORD *)*v18 + 16LL))(*v18, 0);
        v18 += 15;
        --v19;
      }
      while ( v19 );
      *(_DWORD *)(v11 + 4072) = v2;
      LOBYTE(v20) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v13 + 16LL))(*v13, v20);
    }
    ++v2;
    v13 += 15;
    v12 += 15;
  }
  while ( v2 < 4 );
  v21 = *(_QWORD *)(120LL * *(int *)(v11 + 4072) + v11 + 2824);
  LOBYTE(v16) = 1;
  return (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v21 + 16LL))(v21, v16);
}


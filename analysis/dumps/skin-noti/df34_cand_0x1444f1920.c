// cand_0x1444f1920

__int64 __fastcall sub_1444F1920(__int64 a1, __int64 a2)
{
  __int64 v4; // r8
  __int64 v5; // rax
  __int64 v6; // rax
  int v7; // eax
  __int64 v8; // rax
  __int64 v9; // r8
  __int64 v10; // rbp
  _QWORD *v11; // rdx
  __int64 v12; // rax
  __int64 v13; // r8
  volatile signed __int32 *v14; // rbx
  _QWORD *v15; // rdx
  __int64 v16; // rax
  __int64 v17; // r8
  volatile signed __int32 *v18; // rbx
  _QWORD *v19; // rdx
  __int64 v20; // rax
  volatile signed __int32 *v21; // rbx
  __int64 result; // rax
  _BYTE v23[8]; // [rsp+28h] [rbp-40h] BYREF
  volatile signed __int32 *v24; // [rsp+30h] [rbp-38h]
  _BYTE v25[8]; // [rsp+38h] [rbp-30h] BYREF
  volatile signed __int32 *v26; // [rsp+40h] [rbp-28h]
  _BYTE v27[8]; // [rsp+48h] [rbp-20h] BYREF
  volatile signed __int32 *v28; // [rsp+50h] [rbp-18h]

  *(_WORD *)(a1 + 104) = *(_WORD *)a2;
  if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
  {
    v5 = sub_141308BD0(qword_14E66C090);
    *(_DWORD *)a1 = sub_141C4A150(v5);
    v6 = sub_141308BD0(qword_14E66C090);
    v7 = sub_1421B2550(v6);
  }
  else
  {
    *(_DWORD *)a1 = *(_DWORD *)(a2 + 6);
    v7 = *(_DWORD *)(a2 + 10);
  }
  *(_DWORD *)(a1 + 4) = v7;
  *(_DWORD *)(a1 + 8) = *(_DWORD *)(a2 + 14);
  *(_DWORD *)(a1 + 12) = *(_DWORD *)(a2 + 18);
  LOBYTE(v4) = 1;
  v8 = sub_140283D60(qword_14E683BF8, *(unsigned int *)(a2 + 2), v4);
  v10 = v8;
  if ( v8 )
  {
    v11 = *(_QWORD **)(v8 + 680);
    if ( (unsigned __int64)((__int64)(*(_QWORD *)(v8 + 688) - (_QWORD)v11) >> 5) >= 3 )
    {
      if ( v11[3] >= 8u )
        v11 = (_QWORD *)*v11;
      LOBYTE(v9) = 1;
      v12 = sub_144724390(v23, v11, v9, 0);
      sub_1401E5080(a1 + 24, v12);
      v14 = v24;
      if ( v24 )
      {
        if ( _InterlockedExchangeAdd(v24 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v14)(v14);
          if ( _InterlockedExchangeAdd(v14 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v14 + 8LL))(v14);
        }
      }
      v15 = (_QWORD *)(*(_QWORD *)(v10 + 680) + 32LL);
      if ( *(_QWORD *)(*(_QWORD *)(v10 + 680) + 56LL) >= 8u )
        v15 = (_QWORD *)*v15;
      LOBYTE(v13) = 1;
      v16 = sub_144724390(v25, v15, v13, 0);
      sub_1401E5080(a1 + 40, v16);
      v18 = v26;
      if ( v26 )
      {
        if ( _InterlockedExchangeAdd(v26 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
          if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
        }
      }
      v19 = (_QWORD *)(*(_QWORD *)(v10 + 680) + 64LL);
      if ( *(_QWORD *)(*(_QWORD *)(v10 + 680) + 88LL) >= 8u )
        v19 = (_QWORD *)*v19;
      LOBYTE(v17) = 1;
      v20 = sub_144724390(v27, v19, v17, 0);
      sub_1401E5080(a1 + 56, v20);
      v21 = v28;
      if ( v28 )
      {
        if ( _InterlockedExchangeAdd(v28 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
          if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
        }
      }
    }
  }
  *(_BYTE *)(a1 + 16) = *(_BYTE *)(a2 + 22) == 0;
  result = sub_146E9FBC0(a1 + 72);
  *(_DWORD *)(a1 + 20) = 0;
  return result;
}


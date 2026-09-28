// reader_sub_1441DB6C0

__int64 __fastcall sub_1441DB6C0(__int64 a1)
{
  char v2; // bl
  bool v3; // r14
  __int64 v4; // rdi
  volatile signed __int32 *v5; // rbx
  __int64 v6; // rbx
  __int64 v7; // rax
  volatile signed __int32 *v8; // rbx
  volatile signed __int32 *v9; // rbx
  __int64 v10; // rbx
  __int64 v11; // rax
  volatile signed __int32 *v12; // rbx
  volatile signed __int32 *v13; // rbx
  unsigned int v14; // ebx
  __int64 v15; // rdx
  __int64 v16; // rcx
  __int64 v17; // rax
  void (__fastcall ***v18)(_QWORD); // rcx
  __int16 v19; // ax
  __int64 v20; // rax
  int v21; // eax
  _QWORD v23[2]; // [rsp+30h] [rbp-51h] BYREF
  __int64 v24; // [rsp+40h] [rbp-41h]
  __int64 v25; // [rsp+48h] [rbp-39h]
  __int128 v26; // [rsp+50h] [rbp-31h] BYREF
  __int128 v27; // [rsp+60h] [rbp-21h] BYREF
  __int64 v28; // [rsp+70h] [rbp-11h]
  _BYTE v29[8]; // [rsp+78h] [rbp-9h] BYREF
  volatile signed __int32 *v30; // [rsp+80h] [rbp-1h]
  _BYTE v31[8]; // [rsp+88h] [rbp+7h] BYREF
  volatile signed __int32 *v32; // [rsp+90h] [rbp+Fh]
  _BYTE v33[8]; // [rsp+98h] [rbp+17h] BYREF
  volatile signed __int32 *v34; // [rsp+A0h] [rbp+1Fh]
  _BYTE v35[8]; // [rsp+A8h] [rbp+27h] BYREF
  volatile signed __int32 *v36; // [rsp+B0h] [rbp+2Fh]
  _BYTE v37[8]; // [rsp+B8h] [rbp+37h] BYREF
  volatile signed __int32 *v38; // [rsp+C0h] [rbp+3Fh]

  v28 = -2;
  v2 = 0;
  sub_145F70B70(a1);
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 1600)) && (unsigned __int8)sub_146E9FA80(a1 + 1616) )
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1600) + 16LL))(*(_QWORD *)(a1 + 1600), 0);
  v3 = 0;
  if ( qword_14E683C78 )
  {
    v2 = 1;
    if ( *(_QWORD *)(*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v29) )
      v3 = 1;
  }
  v4 = -1;
  if ( (v2 & 1) != 0 )
  {
    v5 = v30;
    if ( v30 )
    {
      if ( _InterlockedExchangeAdd(v30 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v5)(v5);
        if ( _InterlockedExchangeAdd(v5 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v5 + 8LL))(v5);
      }
    }
  }
  if ( v3 && !(unsigned __int8)sub_146684530(qword_14E683C78, a1) )
  {
    v6 = *(_QWORD *)(*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v33);
    v7 = (*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v31);
    v26 = 0;
    v26 = *(_OWORD *)v7;
    *(_QWORD *)v7 = 0;
    *(_QWORD *)(v7 + 8) = 0;
    sub_146ED0FD0(v6, &v26);
    v8 = v32;
    if ( v32 )
    {
      if ( _InterlockedExchangeAdd(v32 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
        if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
      }
    }
    v9 = v34;
    if ( v34 )
    {
      if ( _InterlockedExchangeAdd(v34 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v9)(v9);
        if ( _InterlockedExchangeAdd(v9 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v9 + 8LL))(v9);
      }
    }
    v10 = *(_QWORD *)(*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v37);
    v11 = (*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)a1 + 272LL))(a1, v35);
    v27 = 0;
    v27 = *(_OWORD *)v11;
    *(_QWORD *)v11 = 0;
    *(_QWORD *)(v11 + 8) = 0;
    sub_146ED14B0(v10, &v27);
    v12 = v36;
    if ( v36 )
    {
      if ( _InterlockedExchangeAdd(v36 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
        if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
      }
    }
    v13 = v38;
    if ( v38 )
    {
      if ( _InterlockedExchangeAdd(v38 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v13)(v13);
        if ( _InterlockedExchangeAdd(v13 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v13 + 8LL))(v13);
      }
    }
    sub_146695EE0(qword_14E683C78, a1, 0);
  }
  v14 = 0;
  while ( !(*(unsigned __int8 (__fastcall **)(__int64, _QWORD, __int64))(*(_QWORD *)qword_14F1C0F28 + 40LL))(
             qword_14F1C0F28,
             v14,
             a1) )
  {
    if ( (int)++v14 >= 134 )
      goto LABEL_51;
  }
  v16 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v17 = sub_146E8BA20(112);
    if ( v17 )
      v18 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v17);
    else
      v18 = 0;
    qword_14E634230 = (__int64)v18;
    (**v18)(v18);
    v16 = qword_14E634230;
  }
  v19 = sub_1403F5060(v16, v14);
  switch ( v14 )
  {
    case 4u:
    case 5u:
    case 7u:
    case 8u:
    case 0xAu:
    case 0xCu:
    case 0x10u:
    case 0x1Au:
    case 0x1Fu:
    case 0x20u:
    case 0x5Du:
      sub_1441E4F30(a1);
      v20 = sub_14723C170(100002257);
      v23[0] = 0;
      v24 = 0;
      v25 = 7;
      do
        ++v4;
      while ( *(_WORD *)(v20 + 2 * v4) );
      goto LABEL_50;
    default:
      if ( v19 == 157 )
      {
        *(_DWORD *)(a1 + 1684) = v14;
        sub_1441E4F30(a1);
      }
      else
      {
        sub_1441E4F30(a1);
        if ( *(_DWORD *)(a1 + 1684) != v14 )
        {
          v20 = sub_14723C170(100002251);
          v23[0] = 0;
          v24 = 0;
          v25 = 7;
          do
            ++v4;
          while ( *(_WORD *)(v20 + 2 * v4) );
LABEL_50:
          sub_14014C8D0(v23, v20);
          sub_1441E3470(a1, v23);
        }
      }
      break;
  }
LABEL_51:
  v21 = *(_DWORD *)(a1 + 1684);
  LOBYTE(v15) = *(_DWORD *)(a1 + 1680) != v21 && v21 != 134;
  return (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1648) + 24LL))(*(_QWORD *)(a1 + 1648), v15);
}


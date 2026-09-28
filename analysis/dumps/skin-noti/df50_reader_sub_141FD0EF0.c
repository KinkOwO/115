// reader_sub_141FD0EF0

__int64 __fastcall sub_141FD0EF0(__int64 a1, _QWORD *a2)
{
  _QWORD *v4; // rax
  int v5; // edi
  char v6; // al
  __int64 v7; // r8
  volatile signed __int32 *v8; // rbx
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // r14
  unsigned int v12; // ebx
  __int64 v13; // rax
  volatile signed __int32 *v14; // rbx
  __int64 v15; // rbx
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rdx
  __int64 v23; // rcx
  volatile signed __int32 *v24; // rbx
  __int128 v26; // [rsp+38h] [rbp-70h] BYREF
  __int64 v27; // [rsp+48h] [rbp-60h] BYREF
  volatile signed __int32 *v28; // [rsp+50h] [rbp-58h]
  __int64 v29; // [rsp+58h] [rbp-50h] BYREF
  volatile signed __int32 *v30; // [rsp+60h] [rbp-48h]
  _BYTE v31[8]; // [rsp+68h] [rbp-40h] BYREF
  volatile signed __int32 *v32; // [rsp+70h] [rbp-38h]

  v4 = (_QWORD *)sub_140E7FA20(*a2);
  if ( v4[3] >= 8u )
    v4 = (_QWORD *)*v4;
  if ( (unsigned int)sub_141FC7EB0(a1 - 24, v4) == 16 )
  {
    v5 = 0;
    while ( 1 )
    {
      sub_141FC7B00(a1 - 24, &v29, (unsigned int)v5);
      v6 = sub_146ECA5A0(v29);
      v8 = v30;
      if ( v6 )
        break;
      if ( v30 )
      {
        if ( _InterlockedExchangeAdd(v30 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
          if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
        }
      }
      if ( ++v5 > 5 )
        goto LABEL_17;
    }
    if ( v30 )
    {
      if ( _InterlockedExchangeAdd(v30 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
        if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
      }
    }
    if ( v5 < 0 )
    {
LABEL_17:
      LOBYTE(v7) = 1;
      if ( (unsigned __int8)sub_146682140(qword_14E683C78, 3627, v7) )
        sub_146694510(qword_14E683C78, 3627, -1, 0, 1);
      goto LABEL_29;
    }
    LOBYTE(v7) = 1;
    if ( (unsigned __int8)sub_146682140(qword_14E683C78, 3627, v7) )
    {
      v11 = sub_14667EB40(qword_14E683C78, 3627);
      if ( !v11 )
        goto LABEL_29;
      v12 = sub_1467A2AF0(3627);
      v13 = (*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)v11 + 272LL))(v11, v31);
      v26 = 0;
      v26 = *(_OWORD *)v13;
      *(_QWORD *)v13 = 0;
      *(_QWORD *)(v13 + 8) = 0;
      sub_1466978F0(qword_14E683C78, v12, &v26);
      v14 = v32;
      if ( v32 )
      {
        if ( _InterlockedExchangeAdd(v32 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v14)(v14);
          if ( _InterlockedExchangeAdd(v14 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v14 + 8LL))(v14);
        }
      }
      sub_141FD6990(v11, (unsigned int)v5);
      v10 = v11;
    }
    else
    {
      v9 = sub_14668C520(qword_14E683C78, 3627, 0, v5);
      if ( !v9 )
        goto LABEL_29;
      v10 = v9;
    }
    sub_141FC7160(v10);
  }
LABEL_29:
  if ( (unsigned __int8)sub_146EC5870() )
  {
    sub_141FC7B00(a1 - 24, &v27, 14);
    if ( (unsigned __int8)sub_141FB6530(v27) )
    {
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v27 + 16LL))(v27, 0);
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v27 + 24LL))(v27, 0);
    }
    v15 = sub_141F880C0();
    if ( (unsigned __int8)sub_141F88610(v15) )
    {
      v17 = sub_146D74000(v16);
      sub_146D746E0(v17, 681);
      v19 = sub_146D74000(v18);
      sub_146D75CE0(v19, 2639);
      v21 = sub_146D74000(v20);
      sub_146D75CE0(v21, 3);
      sub_146D75AF0(v23, v22);
      sub_141F88E90(v15);
    }
    sub_141FD5C20(a1 - 24);
    v24 = v28;
    if ( v28 )
    {
      if ( _InterlockedExchangeAdd(v28 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v24)(v24);
        if ( _InterlockedExchangeAdd(v24 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v24 + 8LL))(v24);
      }
    }
  }
  return sub_1467A6790(a1, a2);
}


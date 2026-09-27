// ebc10caller_sub_1458D0540_0x1458d0540

__int64 __fastcall sub_1458D0540(__int64 a1)
{
  int v2; // ecx
  void (__fastcall ***v3)(_QWORD); // r14
  _BOOL8 v4; // rdx
  __int64 v5; // rbx
  __int64 v6; // rax
  __int64 v7; // rdx
  __int64 v8; // rcx
  __int64 v9; // rcx
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  __int64 v12; // rdx
  __int64 v13; // r8
  unsigned int *i; // rbx
  unsigned int *v15; // rbp
  __int64 v16; // rax
  __int64 v17; // rdi
  __int64 *v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rdx
  _QWORD *v21; // rdx
  __int64 v22; // rdx
  _QWORD *v23; // rdx
  __int64 v24; // rcx
  __int64 v25; // rcx
  unsigned __int64 v26; // rdx
  __int64 v27; // rbx
  __int64 (__fastcall *v28)(__int64, _QWORD); // rdi
  __int64 v29; // rcx
  __int64 v30; // rax
  unsigned __int8 v31; // al
  __int64 result; // rax
  __int64 v33; // r8
  volatile signed __int32 *v34; // rbx
  __int64 v35; // rdi
  unsigned __int8 v36; // al
  __int128 v37; // [rsp+38h] [rbp-30h] BYREF
  __int64 v38; // [rsp+48h] [rbp-20h]

  v2 = *(_DWORD *)(a1 + 1216);
  if ( !v2 || (unsigned int)(v2 - 1) <= 1 )
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1176) + 16LL))(*(_QWORD *)(a1 + 1176), 0);
  sub_1458BE240(a1);
  sub_146F05890(*(_QWORD *)(a1 + 1224));
  v3 = 0;
  v4 = (unsigned __int8)sub_1444F6A20(4) != 0;
  (*(void (__fastcall **)(_QWORD, _BOOL8))(**(_QWORD **)(a1 + 1224) + 16LL))(*(_QWORD *)(a1 + 1224), v4);
  v5 = *(_QWORD *)(a1 + 1384);
  if ( v5 )
  {
    v6 = sub_146E8C7D0(&unk_149C66170);
    sub_146EECE20(v5, v6);
    sub_146EECBB0(*(_QWORD *)(a1 + 1384), 0);
    v8 = *(_QWORD *)(a1 + 1560);
    if ( v8 )
    {
      LOBYTE(v7) = 1;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v8 + 16LL))(v8, v7);
    }
    v9 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v10 = sub_146E8BA20(1472);
      if ( v10 )
        v11 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v10);
      else
        v11 = 0;
      qword_14E638F28 = (__int64)v11;
      (**v11)(v11);
      v9 = qword_14E638F28;
    }
    sub_1444EBC10(v9, &v37, 0);
    v15 = (unsigned int *)*((_QWORD *)&v37 + 1);
    for ( i = (unsigned int *)v37; i != v15; ++i )
    {
      v16 = sub_1444EBAB0(*i, v12, v13);
      v17 = v16;
      if ( v16 && *(_DWORD *)(v16 + 12) == 1 )
      {
        v18 = *(__int64 **)(a1 + 1400);
        v19 = *v18;
        if ( *(_QWORD *)(v17 + 592) )
        {
          LOBYTE(v12) = 1;
          (*(void (__fastcall **)(__int64 *, __int64))(v19 + 16))(v18, v12);
          (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1384) + 16LL))(*(_QWORD *)(a1 + 1384), 0);
          v23 = (_QWORD *)(v17 + 576);
          if ( *(_QWORD *)(v17 + 600) >= 8u )
            v23 = (_QWORD *)*v23;
          sub_146AF0AA0(*(_QWORD *)(a1 + 1400), v23);
        }
        else
        {
          (*(void (__fastcall **)(__int64 *, _QWORD))(v19 + 16))(v18, 0);
          LOBYTE(v20) = 1;
          (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1384) + 16LL))(*(_QWORD *)(a1 + 1384), v20);
          v21 = (_QWORD *)(v17 + 152);
          if ( *(_QWORD *)(v17 + 176) >= 8u )
            v21 = (_QWORD *)*v21;
          sub_146EECE20(*(_QWORD *)(a1 + 1384), v21);
          sub_146EECBB0(*(_QWORD *)(a1 + 1384), *(unsigned int *)(v17 + 184));
        }
        v24 = *(_QWORD *)(a1 + 1560);
        if ( v24 )
        {
          LOBYTE(v22) = *(_BYTE *)(v17 + 1608) == 0;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v24 + 16LL))(v24, v22);
        }
        break;
      }
    }
    v25 = v37;
    if ( (_QWORD)v37 )
    {
      v26 = 4 * ((v38 - (__int64)v37) >> 2);
      if ( v26 >= 0x1000 )
      {
        v26 += 39LL;
        v25 = *(_QWORD *)(v37 - 8);
        if ( (unsigned __int64)(v37 - v25 - 8) > 0x1F )
          sub_148AAF304(v25, v26);
      }
      sub_146E9F3A0(v25, v26);
      v37 = 0;
      v38 = 0;
    }
  }
  v27 = *(_QWORD *)(a1 + 1320);
  v28 = *(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v27 + 24LL);
  v29 = qword_14E634470;
  if ( !qword_14E634470 )
  {
    v30 = sub_146E8BA20(5952);
    if ( v30 )
      v3 = (void (__fastcall ***)(_QWORD))sub_14363D390(v30);
    qword_14E634470 = (__int64)v3;
    (**v3)(v3);
    v29 = qword_14E634470;
  }
  v31 = sub_143641E00(v29);
  result = v28(v27, v31);
  v34 = *(volatile signed __int32 **)(a1 + 1480);
  if ( v34 )
  {
    _InterlockedIncrement(v34 + 2);
    v34 = *(volatile signed __int32 **)(a1 + 1480);
  }
  v35 = *(_QWORD *)(a1 + 1472);
  if ( v35 )
  {
    LOBYTE(v33) = 1;
    v36 = sub_146682140(qword_14E683C78, 538, v33);
    result = sub_146F01920(v35, v36);
  }
  if ( v34 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v34 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v34)(v34);
      if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
    }
  }
  return result;
}


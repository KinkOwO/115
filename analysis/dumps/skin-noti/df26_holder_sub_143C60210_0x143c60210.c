// holder_sub_143C60210_0x143c60210

__int64 __fastcall sub_143C60210(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rax
  __int64 v4; // rax
  __int64 v5; // rax
  __int64 v6; // rbx
  __int64 v7; // rax
  __int64 v8; // rcx
  int v9; // ebp
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  __int64 v12; // rbx
  int v13; // edi
  int v14; // eax
  __int64 v15; // rax
  __int64 v16; // rdx
  __int64 v17; // rcx
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rcx
  __int64 v23; // rax
  void (__fastcall ***v24)(_QWORD); // rcx
  unsigned int *v25; // rdi
  unsigned int *v26; // rbp
  __int64 v27; // rax
  __int64 v28; // r14
  __int64 **v29; // rax
  __int64 v30; // rdx
  void (__fastcall *v31)(__int64 *, __int64); // r8
  volatile signed __int32 *v32; // rbx
  __int64 v33; // rax
  void (__fastcall ***v34)(_QWORD); // rcx
  __int64 v35; // rcx
  __int64 v36; // rax
  void (__fastcall ***v37)(_QWORD); // rcx
  __int64 v38; // rcx
  __int64 v39; // rax
  void (__fastcall ***v40)(_QWORD); // rcx
  __int64 v41; // rbx
  __int64 v42; // rax
  void (__fastcall ***v43)(_QWORD); // rcx
  __int64 v44; // rcx
  __int64 v45; // rax
  void (__fastcall ***v46)(_QWORD); // rcx
  __int64 v47; // rdx
  __int64 result; // rax
  __int64 v49; // rbx
  __int64 v50; // rdx
  __int64 v51; // rdx
  _BYTE v52[8]; // [rsp+48h] [rbp-30h] BYREF
  volatile signed __int32 *v53; // [rsp+50h] [rbp-28h]

  v2 = *(_QWORD *)(a1 + 136);
  if ( v2 )
  {
    if ( *(_BYTE *)(v2 + 110) )
    {
      if ( sub_145EFAFB0() )
      {
        v3 = sub_145EFAFB0();
        sub_145D843E0(v3, 0);
        v4 = sub_145EFAFB0();
        v5 = sub_145CE4330(v4);
        v6 = v5;
        if ( v5 )
        {
          if ( (unsigned __int8)sub_145D742E0(v5) )
            sub_145D843E0(v6, 0);
        }
      }
    }
  }
  v7 = *(_QWORD *)(a1 + 136);
  if ( v7 )
  {
    if ( *(_DWORD *)(v7 + 72) == 1 )
    {
      if ( *(_BYTE *)(v7 + 84) )
      {
        if ( (unsigned __int8)sub_143C628C0(a1) )
        {
          v8 = *(int *)(*(_QWORD *)(a1 + 136) + 76LL);
          if ( (int)v8 < 40000 && !*(_BYTE *)(v8 + a1 + 352) )
            sub_143C671C0(a1, (unsigned __int16)v8);
        }
      }
    }
  }
  v9 = qword_14E6343D0;
  if ( !qword_14E6343D0 )
  {
    v10 = sub_146E8BA20(72);
    if ( v10 )
      v11 = (void (__fastcall ***)(_QWORD))sub_146E93360(v10);
    else
      v11 = 0;
    qword_14E6343D0 = (__int64)v11;
    (**v11)(v11);
    v9 = qword_14E6343D0;
  }
  v12 = sub_146E8C7D0(&unk_14A0B68B0);
  v13 = sub_146E8C7D0(&unk_14A0B68D8);
  v14 = sub_146E8C7D0(&unk_14A0B6820);
  sub_146E938E0(v9, 0, v14, v13, 415, (__int64)&qword_14E65FE80, v12);
  v15 = sub_146E74CB0();
  sub_146E817E0(v15, 0);
  v18 = *(_QWORD *)(a1 + 136);
  if ( v18 && *(_DWORD *)(v18 + 72) == 1 || (v19 = sub_1445A03B0(v17, v16), (unsigned int)sub_14017AEF0(v19) == 1) )
    sub_143C64B20(a1);
  v20 = *(_QWORD *)(a1 + 136);
  if ( v20 && !*(_BYTE *)(v20 + 85) )
    sub_143C64830(a1);
  if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
    sub_143C64280(a1);
  v21 = *(_QWORD *)(a1 + 136);
  if ( v21 && *(_DWORD *)(v21 + 116) <= 9u )
  {
    v22 = qword_14E652F20;
    if ( !qword_14E652F20 )
    {
      v23 = sub_146E8BA20(40);
      if ( v23 )
        v24 = (void (__fastcall ***)(_QWORD))sub_145665790(v23);
      else
        v24 = 0;
      qword_14E652F20 = (__int64)v24;
      (**v24)(v24);
      v21 = *(_QWORD *)(a1 + 136);
      v22 = qword_14E652F20;
    }
    sub_145668DB0(v22, *(unsigned int *)(v21 + 116));
  }
  sub_143C63A80(a1);
  v25 = *(unsigned int **)(a1 + 64);
  v26 = *(unsigned int **)(a1 + 72);
  if ( v25 != v26 )
  {
    do
    {
      v27 = sub_14667BB90(qword_14E683C78, *v25, 0);
      v28 = v27;
      if ( v27 )
      {
        v29 = (__int64 **)(*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)v27 + 272LL))(v27, v52);
        v30 = **v29;
        v31 = *(void (__fastcall **)(__int64 *, __int64))(v30 + 16);
        LOBYTE(v30) = 1;
        v31(*v29, v30);
        v32 = v53;
        if ( v53 )
        {
          if ( _InterlockedExchangeAdd(v53 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v32)(v32);
            if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
          }
        }
        sub_1467A9F20(v28, *((unsigned __int8 *)v25 + 4));
      }
      v25 += 2;
    }
    while ( v25 != v26 );
    v25 = *(unsigned int **)(a1 + 64);
  }
  *(_QWORD *)(a1 + 72) = v25;
  if ( !qword_14E63AE60 )
  {
    v33 = sub_146E8BA20(336);
    if ( v33 )
      v34 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v33);
    else
      v34 = 0;
    qword_14E63AE60 = (__int64)v34;
    (**v34)(v34);
  }
  if ( !(unsigned __int8)sub_1447EDEF0() )
  {
    if ( !qword_14E63AE60 )
    {
      v36 = sub_146E8BA20(336);
      if ( v36 )
        v37 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v36);
      else
        v37 = 0;
      qword_14E63AE60 = (__int64)v37;
      (**v37)(v37);
    }
    LOBYTE(v35) = 1;
    sub_1447EF2C0(v35);
  }
  *(_BYTE *)(a1 + 40540) = 0;
  *(_DWORD *)(a1 + 40536) = -1;
  *(_BYTE *)(a1 + 40542) = 0;
  v38 = qword_14E65FE98;
  if ( !qword_14E65FE98 )
  {
    v39 = sub_146E8BA20(168);
    if ( v39 )
      v40 = (void (__fastcall ***)(_QWORD))sub_147BFA5F0(v39);
    else
      v40 = 0;
    qword_14E65FE98 = (__int64)v40;
    (**v40)(v40);
    v38 = qword_14E65FE98;
  }
  sub_147BFAA20(v38, *(unsigned int *)(*(_QWORD *)(a1 + 136) + 76LL));
  v41 = qword_14E636838;
  if ( !qword_14E636838 )
  {
    v42 = sub_146E8BA20(304);
    if ( v42 )
      v43 = (void (__fastcall ***)(_QWORD))sub_1431954A0(v42);
    else
      v43 = 0;
    qword_14E636838 = (__int64)v43;
    (**v43)(v43);
    v41 = qword_14E636838;
  }
  v44 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v45 = sub_146E8BA20(112);
    if ( v45 )
      v46 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v45);
    else
      v46 = 0;
    qword_14E634230 = (__int64)v46;
    (**v46)(v46);
    v44 = qword_14E634230;
  }
  LOBYTE(v47) = (unsigned __int16)sub_1403F5B80(v44, 232) != 0;
  sub_1427D3B70(v41, v47);
  result = sub_145EFAFB0();
  v49 = result;
  if ( result )
  {
    sub_145C3E3D0(result, 0);
    LOBYTE(v50) = 1;
    (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v49 + 1128LL))(v49, v50);
    LOBYTE(v51) = 1;
    (*(void (__fastcall **)(__int64, __int64, _QWORD))(*(_QWORD *)v49 + 1968LL))(v49, v51, 0);
    result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v49 + 1152LL))(v49);
  }
  *(_QWORD *)(a1 + 136) = 0;
  return result;
}


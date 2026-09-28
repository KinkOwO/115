// sender_2039_sub_1441AAEE0

void __fastcall sub_1441AAEE0(__int64 a1, __int64 a2)
{
  __int64 v3; // rax
  __int64 v4; // rax
  volatile signed __int32 *v5; // rbx
  int v6; // eax
  signed int v7; // r15d
  __int64 v8; // rax
  __int64 v9; // rdi
  void (__fastcall *v10)(__int64, __int64); // rbx
  __int64 v11; // rax
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rdx
  __int64 v15; // rcx
  __int64 v16; // rax
  __int64 v17; // rcx
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rcx
  __int64 v22; // rax
  __int64 v23; // rdx
  __int64 v24; // rcx
  __int64 v25; // rdx
  __int64 v26; // rax
  __int64 v27; // rax
  volatile signed __int32 *v28; // rbx
  volatile signed __int32 *v29; // rbx
  unsigned __int64 v30; // rdx
  __int64 v31; // rcx
  unsigned __int64 v32; // rdx
  __int64 v33; // rcx
  __int64 v34; // rcx
  unsigned __int64 v35; // rdx
  volatile signed __int32 *v36; // rbx
  __int64 v37; // [rsp+28h] [rbp-A9h] BYREF
  volatile signed __int32 *v38; // [rsp+30h] [rbp-A1h]
  __int64 v39; // [rsp+40h] [rbp-91h] BYREF
  volatile signed __int32 *v40; // [rsp+48h] [rbp-89h]
  __int64 v41; // [rsp+50h] [rbp-81h]
  _BYTE v42[8]; // [rsp+58h] [rbp-79h] BYREF
  volatile signed __int32 *v43; // [rsp+60h] [rbp-71h]
  _BYTE v44[16]; // [rsp+68h] [rbp-69h] BYREF
  _BYTE v45[8]; // [rsp+78h] [rbp-59h] BYREF
  volatile signed __int32 *v46; // [rsp+80h] [rbp-51h]
  _BYTE v47[8]; // [rsp+88h] [rbp-49h] BYREF
  unsigned int *v48; // [rsp+90h] [rbp-41h]
  __int128 v49; // [rsp+98h] [rbp-39h]
  __int64 v50; // [rsp+A8h] [rbp-29h]
  __int64 v51; // [rsp+B0h] [rbp-21h]
  __int64 v52; // [rsp+C0h] [rbp-11h]
  unsigned __int64 v53; // [rsp+C8h] [rbp-9h]
  __int64 v54; // [rsp+D0h] [rbp-1h]
  __int64 v55; // [rsp+E0h] [rbp+Fh]
  unsigned __int64 v56; // [rsp+E8h] [rbp+17h]

  v41 = -2;
  sub_145454C60(a1, a2);
  if ( *(_QWORD *)(a1 + 1872) )
  {
    v3 = sub_146E8C7D0(&unk_14926F668);
    v4 = sub_145F6E4E0(a1, v42, v3);
    sub_1401E9A20(&v37, v4);
    v5 = v43;
    if ( v43 )
    {
      if ( _InterlockedExchangeAdd(v43 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v5)(v5);
        if ( _InterlockedExchangeAdd(v5 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v5 + 8LL))(v5);
      }
    }
    v48 = 0;
    v49 = 0;
    v50 = 0;
    v51 = 0;
    v52 = 0;
    v53 = 7;
    v54 = 0;
    v55 = 0;
    v56 = 7;
    sub_147BA57F0(*(_QWORD *)(a1 + 1872), 0, v47);
    if ( (__int64)(v49 - (_QWORD)v48) >> 3 )
    {
      v6 = sub_145AD55B0(qword_14E683CD8, *v48);
      if ( v6 >= 0 )
      {
        v8 = sub_145AD8D10(qword_14E683CD8, (unsigned int)v6);
        if ( v8 )
          v7 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v8 + 224LL))(v8);
        else
          v7 = 0;
      }
      else
      {
        v7 = 0;
      }
      v9 = *(_QWORD *)(a1 + 1880);
      v10 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v9 + 688LL);
      v11 = sub_14723C170(30398);
      v12 = sub_146E8CF20(v44, v11, (unsigned int)v7);
      v13 = sub_14014F430(v12);
      v10(v9, v13);
      sub_146E8C910(v44);
      if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 && v7 >= (int)v48[1] )
        LOBYTE(v14) = 1;
      else
        v14 = 0;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v37 + 24LL))(v37, v14);
      if ( sub_146ECFD90(v37) )
      {
        v16 = sub_146D74000(v15);
        sub_146D746E0(v16, 680);
        v18 = sub_146D74000(v17);
        sub_146D75CE0(v18, 407);
        v20 = sub_146D74000(v19);
        sub_146D75CE0(v20, 0);
        v22 = sub_146D74000(v21);
        sub_146D75CE0(v22, 1);
        sub_146D75AF0(v24, v23);
        (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 1856) + 432LL))(*(_QWORD *)(a1 + 1856));
        LOBYTE(v25) = 1;
        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 1856) + 16LL))(*(_QWORD *)(a1 + 1856), v25);
      }
      if ( (unsigned __int8)sub_146AEF960(*(_QWORD *)(a1 + 1856)) )
        (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 1856) + 16LL))(*(_QWORD *)(a1 + 1856), 0);
      v26 = sub_146E8C7D0(&unk_1492428B0);
      v27 = sub_145F6E4E0(a1, v45, v26);
      sub_1401E9A20(&v39, v27);
      v28 = v46;
      if ( v46 )
      {
        if ( _InterlockedExchangeAdd(v46 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v28)(v28);
          if ( _InterlockedExchangeAdd(v28 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v28 + 8LL))(v28);
        }
      }
      if ( sub_146ECFD90(v39) )
        (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 264LL))(a1);
      v29 = v40;
      if ( v40 )
      {
        if ( _InterlockedExchangeAdd(v40 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v29)(v29);
          if ( _InterlockedExchangeAdd(v29 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v29 + 8LL))(v29);
        }
      }
    }
    if ( v56 >= 8 )
    {
      v30 = 2 * v56 + 2;
      v31 = v54;
      if ( v30 >= 0x1000 )
      {
        v30 = 2 * v56 + 41;
        v31 = *(_QWORD *)(v54 - 8);
        if ( (unsigned __int64)(v54 - v31 - 8) > 0x1F )
          sub_148AAF304(v31, v30);
      }
      sub_146E9F3A0(v31, v30);
    }
    v55 = 0;
    v56 = 7;
    LOWORD(v54) = 0;
    if ( v53 >= 8 )
    {
      v32 = 2 * v53 + 2;
      v33 = v51;
      if ( v32 >= 0x1000 )
      {
        v32 = 2 * v53 + 41;
        v33 = *(_QWORD *)(v51 - 8);
        if ( (unsigned __int64)(v51 - v33 - 8) > 0x1F )
          sub_148AAF304(v33, v32);
      }
      sub_146E9F3A0(v33, v32);
    }
    v52 = 0;
    v53 = 7;
    LOWORD(v51) = 0;
    v34 = (__int64)v48;
    if ( v48 )
    {
      v35 = 8 * ((__int64)(*((_QWORD *)&v49 + 1) - (_QWORD)v48) >> 3);
      if ( v35 >= 0x1000 )
      {
        v35 += 39LL;
        v34 = *((_QWORD *)v48 - 1);
        if ( (unsigned __int64)v48 - v34 - 8 > 0x1F )
          sub_148AAF304(v34, v35);
      }
      sub_146E9F3A0(v34, v35);
      v48 = 0;
      v49 = 0;
    }
    v36 = v38;
    if ( v38 && _InterlockedExchangeAdd(v38 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v36)(v36);
      if ( _InterlockedExchangeAdd(v36 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v36 + 8LL))(v36);
    }
  }
}


// global_writer_sub_146ED27D0

__int64 __fastcall sub_146ED27D0(__int64 a1)
{
  int v2; // eax
  _BYTE *v3; // rdx
  __int64 v4; // rbx
  _QWORD *v5; // rax
  unsigned int v6; // eax
  __int64 v7; // rbx
  __int64 v8; // rax
  __int64 v9; // r15
  void (__fastcall *v10)(__int64, __int64, __int64, _QWORD, _QWORD); // rbx
  __int64 v11; // rax
  volatile signed __int32 *v12; // rbx
  unsigned int v13; // ebx
  int v14; // eax
  __int64 v15; // rbx
  __int64 v16; // rax
  __int64 v17; // rbx
  __int64 v18; // rax
  __int64 v19; // r15
  void (__fastcall *v20)(__int64, __int64, __int64, _QWORD, _QWORD); // rbx
  __int64 v21; // rax
  volatile signed __int32 *v22; // rbx
  int v23; // eax
  int v24; // eax
  __int64 v25; // rdx
  signed __int32 v26; // eax
  signed __int32 v27; // ett
  __int64 v28; // rdi
  volatile signed __int32 *v29; // rbx
  __int64 v30; // rbx
  __int64 v31; // rbx
  __int64 v32; // rdx
  signed __int32 v33; // eax
  signed __int32 v34; // ett
  __int64 v35; // rdi
  __int64 *v36; // rbx
  __int64 v37; // rcx
  __int64 v38; // rdi
  __int64 v39; // rax
  volatile signed __int32 *v40; // rcx
  volatile signed __int32 *v41; // rbx
  __int64 v42; // rbx
  __int64 v43; // rax
  _QWORD *v44; // rsi
  __int64 v45; // rcx
  volatile signed __int32 *v46; // rdi
  __int64 v47; // rdi
  __int64 v48; // rcx
  __int64 v49; // rax
  __int64 v50; // rax
  int v51; // ebx
  __int64 v52; // rcx
  __int64 v53; // rax
  int v54; // eax
  __int64 v55; // rbx
  __int64 v56; // rax
  volatile signed __int32 *v57; // rdi
  volatile signed __int32 *v59; // rbx
  __int64 v60; // [rsp+30h] [rbp-79h] BYREF
  volatile signed __int32 *v61; // [rsp+38h] [rbp-71h]
  _BYTE v62[16]; // [rsp+40h] [rbp-69h] BYREF
  __int128 v63; // [rsp+50h] [rbp-59h]
  __int128 v64; // [rsp+60h] [rbp-49h]
  _BYTE v65[16]; // [rsp+70h] [rbp-39h] BYREF
  __int64 v66; // [rsp+80h] [rbp-29h]
  char v67[8]; // [rsp+88h] [rbp-21h] BYREF
  volatile signed __int32 *v68; // [rsp+90h] [rbp-19h]
  char v69[8]; // [rsp+98h] [rbp-11h] BYREF
  volatile signed __int32 *v70; // [rsp+A0h] [rbp-9h]
  char v71[8]; // [rsp+A8h] [rbp-1h] BYREF
  volatile signed __int32 *v72; // [rsp+B0h] [rbp+7h]
  char v73[16]; // [rsp+B8h] [rbp+Fh] BYREF
  char v74[56]; // [rsp+C8h] [rbp+1Fh] BYREF
  __int64 v75; // [rsp+118h] [rbp+6Fh]

  v66 = -2;
  if ( dword_14DC6B64C != -1 && dword_14DC6B64C != *(_DWORD *)(a1 + 496) || *(_BYTE *)(a1 + 584) )
    return 0xFFFFFFFFLL;
  sub_146EC9810(a1, &v60);
  if ( !v60 )
  {
LABEL_104:
    v59 = v61;
    if ( v61 )
    {
      if ( _InterlockedExchangeAdd(v61 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v59)(v59);
        if ( _InterlockedExchangeAdd(v59 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v59 + 8LL))(v59);
      }
    }
    return 0xFFFFFFFFLL;
  }
  if ( !(unsigned __int8)sub_146ECA5A0(a1) )
  {
    if ( (unsigned int)sub_146EC45C0() == 8 )
    {
      dword_14DC6B63C = -1;
      dword_14DC6B66C = -1;
      dword_14DC6B650 = -1;
    }
    if ( (unsigned int)sub_146EC47B0() == 16 )
    {
      dword_14DC6B644 = -1;
      dword_14DC6B650 = -1;
    }
    goto LABEL_104;
  }
  if ( !(*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)a1 + 544LL))(a1) )
  {
    v23 = sub_146EC45C0();
    switch ( v23 )
    {
      case 1:
        if ( dword_14DC6B650 == -1 )
        {
          v75 = sub_146EC45D0(*(_QWORD *)(a1 + 400));
          if ( *(_BYTE *)(a1 + 469) )
          {
            v47 = 0;
            v48 = qword_14E6343D8;
            if ( !qword_14E6343D8 )
            {
              v49 = sub_146E9F2A0(184);
              if ( v49 )
                v50 = sub_146EC2F90(v49);
              else
                v50 = 0;
              qword_14E6343D8 = v50;
              (**(void (__fastcall ***)(__int64))(v50 + 16))(v50 + 16);
              v48 = qword_14E6343D8;
            }
            v51 = sub_146EC32D0(v48, (unsigned int)v75, 2);
            v52 = qword_14E6343D8;
            if ( !qword_14E6343D8 )
            {
              v53 = sub_146E9F2A0(184);
              if ( v53 )
                v47 = sub_146EC2F90(v53);
              qword_14E6343D8 = v47;
              (**(void (__fastcall ***)(__int64))(v47 + 16))(v47 + 16);
              v52 = qword_14E6343D8;
            }
            v54 = sub_146EC35D0(v52, HIDWORD(v75), 2);
          }
          else
          {
            v54 = HIDWORD(v75);
            v51 = v75;
          }
          *(_DWORD *)(a1 + 472) = v51;
          *(_DWORD *)(a1 + 476) = v54;
          dword_14DC6B650 = *(_DWORD *)(a1 + 496);
        }
        v55 = v60;
        v56 = sub_146ECE600(a1, v62);
        v6 = sub_146ED0FD0(v55, v56);
        goto LABEL_94;
      case 8:
        v3 = v62;
        goto LABEL_10;
      case 64:
        v44 = (_QWORD *)sub_146ECE600(a1, v74);
        if ( dword_14DC6B66C == *(_DWORD *)(*v44 + 496LL) )
        {
          dword_14DC6B640 = *(_DWORD *)(*v44 + 496LL);
          v45 = *(_QWORD *)(*v44 + 552LL);
          if ( v45 )
            (*(void (__fastcall **)(__int64, _QWORD *, __int64, _QWORD, _QWORD))(*(_QWORD *)v45 + 8LL))(
              v45,
              v44,
              15,
              0,
              0);
        }
        else
        {
          sub_146F543A0();
        }
        v13 = *(_DWORD *)(*v44 + 496LL);
        dword_14DC6B648 = v13;
        v46 = (volatile signed __int32 *)v44[1];
        if ( v46 )
        {
          if ( _InterlockedExchangeAdd(v46 + 2, 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v46)(v46);
            if ( _InterlockedExchangeAdd(v46 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v46 + 8LL))(v46);
          }
        }
        goto LABEL_95;
    }
    v24 = sub_146EC47B0();
    if ( v24 == 2 )
    {
      if ( dword_14DC6B650 == -1 )
        dword_14DC6B650 = *(_DWORD *)(a1 + 496);
      v42 = v60;
      v43 = sub_146ECE600(a1, v62);
      v6 = sub_146ED18B0(v42, v43);
      goto LABEL_94;
    }
    if ( v24 == 16 )
    {
LABEL_18:
      v15 = v60;
      v16 = sub_146ECE600(a1, v62);
      v6 = sub_146ED1D70(v15, v16);
      goto LABEL_94;
    }
LABEL_30:
    v25 = *((_QWORD *)&xmmword_14F1E5030 + 1);
    if ( *((_QWORD *)&xmmword_14F1E5030 + 1) && *(_DWORD *)(*((_QWORD *)&xmmword_14F1E5030 + 1) + 8LL) )
    {
      v63 = 0;
      v26 = *(_DWORD *)(*((_QWORD *)&xmmword_14F1E5030 + 1) + 8LL);
      if ( v26 )
      {
        while ( 1 )
        {
          v27 = v26;
          v26 = _InterlockedCompareExchange((volatile signed __int32 *)(v25 + 8), v26 + 1, v26);
          if ( v27 == v26 )
            break;
          if ( !v26 )
            goto LABEL_37;
        }
        v63 = xmmword_14F1E5030;
      }
LABEL_37:
      v28 = *(_QWORD *)sub_146ECE600(a1, v71);
      v29 = v72;
      if ( v72 )
      {
        if ( _InterlockedExchangeAdd(v72 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v29)(v29);
          if ( _InterlockedExchangeAdd(v29 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v29 + 8LL))(v29);
        }
      }
      if ( *((_QWORD *)&v63 + 1) )
      {
        if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v63 + 1) + 8LL), 0xFFFFFFFF) == 1 )
        {
          v30 = *((_QWORD *)&v63 + 1);
          (***((void (__fastcall ****)(_QWORD))&v63 + 1))(*((_QWORD *)&v63 + 1));
          if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v30 + 12), 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v63 + 1) + 8LL))(*((_QWORD *)&v63 + 1));
        }
      }
      if ( (_QWORD)v63 != v28 )
      {
        v31 = sub_146EC45D0(*(_QWORD *)(a1 + 400));
        v64 = 0;
        v32 = *((_QWORD *)&xmmword_14F1E5030 + 1);
        if ( *((_QWORD *)&xmmword_14F1E5030 + 1) )
        {
          v33 = *(_DWORD *)(*((_QWORD *)&xmmword_14F1E5030 + 1) + 8LL);
          if ( v33 )
          {
            while ( 1 )
            {
              v34 = v33;
              v33 = _InterlockedCompareExchange((volatile signed __int32 *)(v32 + 8), v33 + 1, v33);
              if ( v34 == v33 )
                break;
              if ( !v33 )
                goto LABEL_52;
            }
            v64 = xmmword_14F1E5030;
          }
        }
LABEL_52:
        (*(void (__fastcall **)(_QWORD, _QWORD, _QWORD))(*(_QWORD *)v64 + 472LL))(v64, (unsigned int)v31, HIDWORD(v31));
        if ( *((_QWORD *)&v64 + 1) )
        {
          if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v64 + 1) + 8LL), 0xFFFFFFFF) == 1 )
          {
            v35 = *((_QWORD *)&v64 + 1);
            (***((void (__fastcall ****)(_QWORD))&v64 + 1))(*((_QWORD *)&v64 + 1));
            if ( _InterlockedExchangeAdd((volatile signed __int32 *)(v35 + 12), 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(_QWORD))(**((_QWORD **)&v64 + 1) + 8LL))(*((_QWORD *)&v64 + 1));
          }
        }
        dword_14DC6B648 = *(_DWORD *)(a1 + 496);
        (*(void (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)a1 + 464LL))(a1, (unsigned int)v31, HIDWORD(v31));
      }
    }
    v36 = (__int64 *)sub_146ECE600(a1, v73);
    v37 = v36[1];
    v38 = 0;
    v39 = 0;
    if ( v37 )
    {
      v38 = *v36;
      _InterlockedIncrement((volatile signed __int32 *)(v37 + 12));
      v39 = v37;
    }
    *(_QWORD *)&xmmword_14F1E5030 = v38;
    v40 = (volatile signed __int32 *)*((_QWORD *)&xmmword_14F1E5030 + 1);
    *((_QWORD *)&xmmword_14F1E5030 + 1) = v39;
    if ( v40 && _InterlockedExchangeAdd(v40 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v40 + 8LL))(v40);
    v41 = (volatile signed __int32 *)v36[1];
    if ( v41 )
    {
      if ( _InterlockedExchangeAdd(v41 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v41)(v41);
        if ( _InterlockedExchangeAdd(v41 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v41 + 8LL))(v41);
      }
    }
    v13 = *(_DWORD *)(a1 + 496);
    dword_14DC6B648 = v13;
    goto LABEL_95;
  }
  v2 = sub_146EC45C0();
  if ( v2 == 1 )
  {
    v7 = v60;
    v8 = sub_146ECE600(a1, v65);
    sub_146ED0FD0(v7, v8);
    v9 = *(_QWORD *)(a1 + 552);
    if ( v9 )
    {
      v10 = *(void (__fastcall **)(__int64, __int64, __int64, _QWORD, _QWORD))(*(_QWORD *)v9 + 8LL);
      v11 = sub_146ECE600(a1, v67);
      v10(v9, v11, 36, 0, 0);
      v12 = v68;
      if ( v68 )
      {
        if ( _InterlockedExchangeAdd(v68 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
          if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
          {
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
            v13 = dword_14DC6B670;
            goto LABEL_95;
          }
        }
      }
      goto LABEL_24;
    }
LABEL_16:
    v14 = sub_146EC47B0();
    if ( v14 != 2 )
    {
      if ( v14 == 8 )
        goto LABEL_18;
      goto LABEL_30;
    }
    v17 = v60;
    v18 = sub_146ECE600(a1, v62);
    sub_146ED18B0(v17, v18);
    v19 = *(_QWORD *)(a1 + 552);
    if ( !v19 )
      goto LABEL_30;
    v20 = *(void (__fastcall **)(__int64, __int64, __int64, _QWORD, _QWORD))(*(_QWORD *)v19 + 8LL);
    v21 = sub_146ECE600(a1, v69);
    v20(v19, v21, 36, 0, 0);
    v22 = v70;
    if ( v70 )
    {
      if ( _InterlockedExchangeAdd(v70 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v22)(v22);
        if ( _InterlockedExchangeAdd(v22 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v22 + 8LL))(v22);
      }
    }
LABEL_24:
    v13 = dword_14DC6B670;
    goto LABEL_95;
  }
  if ( v2 != 8 )
    goto LABEL_16;
  v3 = v65;
LABEL_10:
  v4 = v60;
  v5 = (_QWORD *)sub_146ECE600(a1, v3);
  v6 = sub_146ED14B0(v4, v5);
LABEL_94:
  v13 = v6;
LABEL_95:
  v57 = v61;
  if ( v61 && _InterlockedExchangeAdd(v61 + 2, 0xFFFFFFFF) == 1 )
  {
    (**(void (__fastcall ***)(volatile signed __int32 *))v57)(v57);
    if ( _InterlockedExchangeAdd(v57 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v57 + 8LL))(v57);
  }
  return v13;
}


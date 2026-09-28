// sender_2039_sub_141BBA0D0

void __fastcall sub_141BBA0D0(__int64 a1, _QWORD *a2)
{
  _QWORD *v2; // r15
  __int64 v3; // rsi
  volatile signed __int32 *v4; // rbx
  _QWORD *v5; // rax
  __int64 v6; // rsi
  volatile signed __int32 *v7; // rbx
  volatile signed __int32 *v8; // rbx
  _QWORD *v9; // rax
  _QWORD *v10; // rax
  volatile signed __int32 *v11; // rbx
  volatile signed __int32 *v12; // rbx
  unsigned __int16 *v13; // rcx
  unsigned __int64 v14; // r13
  __int64 v15; // r14
  _QWORD *v16; // rdx
  unsigned __int16 *v17; // r12
  unsigned __int16 v18; // r13
  __int64 v19; // rcx
  unsigned __int64 v20; // r14
  unsigned __int64 v21; // rsi
  _QWORD *v22; // rax
  unsigned __int64 v23; // rbx
  unsigned __int64 v24; // rcx
  __int64 v25; // r15
  __int64 v26; // r14
  __int64 v27; // rbx
  unsigned __int64 v28; // rdx
  __int64 v29; // rcx
  _DWORD *v30; // rax
  _DWORD *v31; // rsi
  _QWORD *v32; // rbx
  char v33; // al
  __int64 v34; // rcx
  __int64 v35; // rax
  __int64 v36; // rcx
  __int64 v37; // rax
  __int64 v38; // rdx
  __int64 v39; // rcx
  __int64 v40; // rcx
  unsigned __int64 v41; // rdx
  __int64 v42; // rcx
  unsigned __int64 v43; // rdx
  __int64 v44; // rax
  volatile signed __int32 *v45; // rbx
  bool v46; // [rsp+28h] [rbp-99h]
  _QWORD *v47; // [rsp+30h] [rbp-91h] BYREF
  unsigned __int16 *v48; // [rsp+38h] [rbp-89h]
  __int64 v49; // [rsp+40h] [rbp-81h]
  _QWORD *v50; // [rsp+48h] [rbp-79h]
  char v51[8]; // [rsp+50h] [rbp-71h] BYREF
  volatile signed __int32 *v52; // [rsp+58h] [rbp-69h]
  char v53[8]; // [rsp+60h] [rbp-61h] BYREF
  volatile signed __int32 *v54; // [rsp+68h] [rbp-59h]
  char v55[8]; // [rsp+70h] [rbp-51h] BYREF
  volatile signed __int32 *v56; // [rsp+78h] [rbp-49h]
  char v57[8]; // [rsp+80h] [rbp-41h] BYREF
  volatile signed __int32 *v58; // [rsp+88h] [rbp-39h]
  char v59[8]; // [rsp+90h] [rbp-31h] BYREF
  volatile signed __int32 *v60; // [rsp+98h] [rbp-29h]
  _QWORD v61[2]; // [rsp+A0h] [rbp-21h] BYREF
  unsigned __int64 v62; // [rsp+B0h] [rbp-11h]
  unsigned __int64 v63; // [rsp+B8h] [rbp-9h]
  _QWORD v64[2]; // [rsp+C0h] [rbp-1h] BYREF
  __int64 v65; // [rsp+D0h] [rbp+Fh]
  unsigned __int64 v66; // [rsp+D8h] [rbp+17h]
  _BYTE v67[16]; // [rsp+E0h] [rbp+1Fh] BYREF

  v49 = -2;
  v2 = a2;
  v47 = a2;
  v50 = a2;
  v3 = *(_QWORD *)sub_146EC9810(*a2, v51);
  v4 = v52;
  if ( v52 )
  {
    if ( _InterlockedExchangeAdd(v52 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v4)(v4);
      if ( _InterlockedExchangeAdd(v4 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v4 + 8LL))(v4);
    }
  }
  if ( v3 )
  {
    v5 = (_QWORD *)sub_146EC9810(*v2, v55);
    v6 = *(_QWORD *)sub_146EC9810(*v5, v53);
    v7 = v54;
    if ( v54 )
    {
      if ( _InterlockedExchangeAdd(v54 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v7)(v7);
        if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
      }
    }
    v8 = v56;
    if ( v56 )
    {
      if ( _InterlockedExchangeAdd(v56 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
        if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
      }
    }
    if ( !v6 )
    {
      sub_1401566D0(v2);
      return;
    }
    v9 = (_QWORD *)sub_146EC9810(*v2, v59);
    v10 = (_QWORD *)sub_146EC9810(*v9, v57);
    sub_140E7FA20(*v10);
    sub_14014C810(v64);
    v11 = v58;
    if ( v58 )
    {
      if ( _InterlockedExchangeAdd(v58 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v11)(v11);
        if ( _InterlockedExchangeAdd(v11 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v11 + 8LL))(v11);
      }
    }
    v12 = v60;
    if ( v60 )
    {
      if ( _InterlockedExchangeAdd(v60 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
        if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
      }
    }
    v61[0] = 0;
    v62 = 0;
    v63 = 7;
    sub_14014C8D0(v61, &byte_14BAF7F08);
    v13 = (unsigned __int16 *)v64;
    v14 = v66;
    v46 = v66 >= 8;
    v15 = v64[0];
    if ( v66 < 8 )
    {
      v17 = (unsigned __int16 *)v64;
      v16 = v64;
    }
    else
    {
      v16 = (_QWORD *)v64[0];
      v13 = (unsigned __int16 *)v64[0];
      v17 = (unsigned __int16 *)v64[0];
    }
    v48 = (unsigned __int16 *)v16 + v65;
    if ( v13 != v48 )
    {
      do
      {
        v18 = *v17;
        if ( (unsigned int)sub_148AAF5C0(*v17) )
        {
          v20 = v62;
          v21 = v63;
          if ( v62 >= v63 )
          {
            if ( v62 == 0x7FFFFFFFFFFFFFFELL )
              sub_14014AC70(v19);
            v23 = (v62 + 1) | 7;
            if ( v23 <= 0x7FFFFFFFFFFFFFFELL )
            {
              v24 = v63 >> 1;
              if ( v63 <= 0x7FFFFFFFFFFFFFFELL - (v63 >> 1) )
              {
                if ( v23 < v63 + v24 )
                  v23 = v63 + v24;
              }
              else
              {
                v23 = 0x7FFFFFFFFFFFFFFELL;
              }
            }
            else
            {
              v23 = 0x7FFFFFFFFFFFFFFELL;
            }
            v25 = sub_14014CB50(v61, v23 + 1);
            v62 = v20 + 1;
            v63 = v23;
            v26 = 2 * v20;
            if ( v21 < 8 )
            {
              sub_148AA1E60(v25, v61, v26);
              *(_WORD *)(v26 + v25) = v18;
              *(_WORD *)(v26 + v25 + 2) = 0;
            }
            else
            {
              v27 = v61[0];
              sub_148AA1E60(v25, v61[0], v26);
              *(_WORD *)(v26 + v25) = v18;
              *(_WORD *)(v26 + v25 + 2) = 0;
              v28 = 2 * v21 + 2;
              if ( v28 >= 0x1000 )
              {
                v28 = 2 * v21 + 41;
                v29 = *(_QWORD *)(v27 - 8);
                if ( (unsigned __int64)(v27 - v29 - 8) > 0x1F )
                  sub_148AAF304(v29, v28);
                v27 = *(_QWORD *)(v27 - 8);
              }
              sub_146E9F3A0(v27, v28);
            }
            v61[0] = v25;
          }
          else
          {
            ++v62;
            v22 = v61;
            if ( v63 >= 8 )
              v22 = (_QWORD *)v61[0];
            *((_WORD *)v22 + v20) = v18;
            *((_WORD *)v22 + v20 + 1) = 0;
          }
        }
        ++v17;
      }
      while ( v17 != v48 );
      v15 = v64[0];
      v2 = v47;
      v14 = v66;
    }
    v30 = (_DWORD *)sub_148ABEA20();
    v31 = v30;
    v32 = v61;
    if ( v63 >= 8 )
      v32 = (_QWORD *)v61[0];
    *v30 = 0;
    v33 = sub_148AC4630(v32, &v47, 10);
    if ( v32 == v47 )
      sub_14883BB30("invalid stoi argument");
    if ( *v31 == 34 )
      sub_14883BB78("stoi argument out of range");
    v67[13] = v33;
    v35 = sub_146D74000(v34);
    sub_146D746E0(v35, 2184);
    v37 = sub_146D74000(v36);
    sub_146D75B10(v37, v67, 14);
    sub_146D75AF0(v39, v38);
    if ( v63 >= 8 )
    {
      v41 = 2 * v63 + 2;
      v42 = v61[0];
      if ( v41 >= 0x1000 )
      {
        v41 = 2 * v63 + 41;
        v42 = *(_QWORD *)(v61[0] - 8LL);
        if ( (unsigned __int64)(v61[0] - v42 - 8) > 0x1F )
          sub_148AAF304(v42, v41);
      }
      sub_146E9F3A0(v42, v41);
    }
    v62 = 0;
    v63 = 7;
    LOWORD(v61[0]) = 0;
    if ( v46 )
    {
      v43 = 2 * v14 + 2;
      v44 = v15;
      if ( v43 >= 0x1000 )
      {
        v43 = 2 * v14 + 41;
        v15 = *(_QWORD *)(v15 - 8);
        if ( (unsigned __int64)(v44 - v15 - 8) > 0x1F )
          sub_148AAF304(v40, v43);
      }
      sub_146E9F3A0(v15, v43);
    }
    v65 = 0;
    v66 = 7;
    LOWORD(v64[0]) = 0;
  }
  v45 = (volatile signed __int32 *)v2[1];
  if ( v45 && _InterlockedExchangeAdd(v45 + 2, 0xFFFFFFFF) == 1 )
  {
    (**(void (__fastcall ***)(volatile signed __int32 *))v45)(v45);
    if ( _InterlockedExchangeAdd(v45 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v45 + 8LL))(v45);
  }
}


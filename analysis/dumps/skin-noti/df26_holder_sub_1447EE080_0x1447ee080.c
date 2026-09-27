// holder_sub_1447EE080_0x1447ee080

_QWORD *__fastcall sub_1447EE080(__int64 a1, _QWORD *a2, __int64 a3, char a4)
{
  unsigned int v5; // ebx
  __int64 v7; // rsi
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  _QWORD *v11; // rax
  __int64 v12; // rdx
  __int128 *v13; // rcx
  __int128 *v14; // rax
  __int64 v15; // rax
  __int64 v16; // rax
  __int64 v17; // r9
  volatile signed __int32 *v18; // rsi
  __int64 v19; // rdx
  volatile signed __int32 *v20; // rsi
  volatile signed __int32 *v21; // rsi
  __int64 v22; // rcx
  __int64 v23; // rax
  __int64 v24; // r8
  __int64 v25; // rax
  __int64 v26; // rcx
  volatile signed __int32 *v27; // rsi
  volatile signed __int32 *v28; // rsi
  __int128 *v29; // rcx
  __int128 *v30; // rax
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // r9
  volatile signed __int32 *v34; // rsi
  __int64 v35; // rdx
  volatile signed __int32 *v36; // rsi
  volatile signed __int32 *v37; // rsi
  __int64 v38; // rax
  __int64 v39; // r8
  __int64 v40; // rax
  __int64 v41; // rcx
  volatile signed __int32 *v42; // rsi
  __int128 v44; // [rsp+30h] [rbp-61h] BYREF
  __int128 v45; // [rsp+40h] [rbp-51h] BYREF
  __int128 v46; // [rsp+50h] [rbp-41h]
  __int128 v47; // [rsp+60h] [rbp-31h]
  __int64 v48; // [rsp+70h] [rbp-21h]
  _BYTE v49[8]; // [rsp+78h] [rbp-19h] BYREF
  volatile signed __int32 *v50; // [rsp+80h] [rbp-11h]
  _BYTE v51[8]; // [rsp+88h] [rbp-9h] BYREF
  volatile signed __int32 *v52; // [rsp+90h] [rbp-1h]
  _BYTE v53[8]; // [rsp+98h] [rbp+7h] BYREF
  volatile signed __int32 *v54; // [rsp+A0h] [rbp+Fh]
  _BYTE v55[8]; // [rsp+A8h] [rbp+17h] BYREF
  volatile signed __int32 *v56; // [rsp+B0h] [rbp+1Fh]
  _BYTE v57[8]; // [rsp+B8h] [rbp+27h] BYREF
  volatile signed __int32 *v58; // [rsp+C0h] [rbp+2Fh]
  _BYTE v59[8]; // [rsp+C8h] [rbp+37h] BYREF
  volatile signed __int32 *v60; // [rsp+D0h] [rbp+3Fh]

  v48 = -2;
  v5 = a3;
  LOBYTE(a3) = 1;
  v7 = sub_140283D60(qword_14E683BF8, v5, a3);
  v8 = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v9 = sub_146E8BA20(336);
    *(_QWORD *)&v44 = v9;
    if ( v9 )
      v10 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v9);
    else
      v10 = 0;
    qword_14E63AE60 = (__int64)v10;
    (**v10)(v10);
    v8 = qword_14E63AE60;
  }
  v11 = (_QWORD *)sub_1447EA070(v8, v5);
  *a2 = 0;
  a2[1] = 0;
  if ( a4 )
  {
    if ( v7 )
    {
      if ( v11 )
      {
        v13 = (__int128 *)v11[20];
        v14 = (__int128 *)v11[21];
        if ( v13 != v14 )
        {
          if ( !(v14 - v13) )
            sub_1401790B0(v13, v12);
          v44 = 0;
          v15 = *((_QWORD *)v13 + 1);
          if ( v15 )
            _InterlockedIncrement((volatile signed __int32 *)(v15 + 8));
          v44 = *v13;
          if ( (_QWORD)v44 )
          {
            v16 = sub_144724580(v49);
            sub_1401E5080(a2, v16);
            v18 = v50;
            if ( v50 )
            {
              if ( _InterlockedExchangeAdd(v50 + 2, 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
                if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
              }
            }
            LOBYTE(v17) = 1;
            ((void (__fastcall *)(_QWORD, _BYTE *, __int128 *, __int64, int))sub_146B32610)(*a2, v51, &v44, v17, 1);
            v20 = v52;
            if ( v52 )
            {
              if ( _InterlockedExchangeAdd(v52 + 2, 0xFFFFFFFF) == 1 )
              {
                (**(void (__fastcall ***)(volatile signed __int32 *))v20)(v20);
                if ( _InterlockedExchangeAdd(v20 + 3, 0xFFFFFFFF) == 1 )
                  (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v20 + 8LL))(v20);
              }
            }
            LOBYTE(v19) = 1;
            sub_146B5AA70(*a2, v19);
          }
          v21 = (volatile signed __int32 *)*((_QWORD *)&v44 + 1);
          if ( *((_QWORD *)&v44 + 1) )
          {
            if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v44 + 1) + 8LL), 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v21)(v21);
              if ( _InterlockedExchangeAdd(v21 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v21 + 8LL))(v21);
            }
          }
        }
      }
    }
    v22 = *a2;
    if ( *a2 )
      goto LABEL_63;
    v23 = sub_146E8C7D0(&unk_14A3F44D0);
    LOBYTE(v24) = 1;
    v25 = sub_144724390(v53, v23, v24, 0);
    v46 = 0;
    v46 = *(_OWORD *)v25;
    v12 = *((_QWORD *)&v46 + 1);
    v26 = v46;
    *(_QWORD *)v25 = 0;
    *(_QWORD *)(v25 + 8) = 0;
    *(_QWORD *)&v46 = *a2;
    *a2 = v26;
    *((_QWORD *)&v46 + 1) = a2[1];
    v27 = (volatile signed __int32 *)*((_QWORD *)&v46 + 1);
    a2[1] = v12;
    if ( v27 )
    {
      if ( _InterlockedExchangeAdd(v27 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v27)(v27);
        if ( _InterlockedExchangeAdd(v27 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v27 + 8LL))(v27);
      }
    }
    v28 = v54;
  }
  else
  {
    if ( v7 )
    {
      v29 = (__int128 *)v11[17];
      v30 = (__int128 *)v11[18];
      if ( v29 != v30 )
      {
        if ( !(v30 - v29) )
          sub_1401790B0(v29, v12);
        v45 = 0;
        v31 = *((_QWORD *)v29 + 1);
        if ( v31 )
          _InterlockedIncrement((volatile signed __int32 *)(v31 + 8));
        v45 = *v29;
        if ( (_QWORD)v45 )
        {
          v32 = sub_144724580(v55);
          sub_1401E5080(a2, v32);
          v34 = v56;
          if ( v56 )
          {
            if ( _InterlockedExchangeAdd(v56 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v34)(v34);
              if ( _InterlockedExchangeAdd(v34 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v34 + 8LL))(v34);
            }
          }
          LOBYTE(v33) = 1;
          sub_146B32610(*a2, v57, &v45, v33, 1, v44);
          v36 = v58;
          if ( v58 )
          {
            if ( _InterlockedExchangeAdd(v58 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v36)(v36);
              if ( _InterlockedExchangeAdd(v36 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v36 + 8LL))(v36);
            }
          }
          LOBYTE(v35) = 1;
          sub_146B5AA70(*a2, v35);
        }
        v37 = (volatile signed __int32 *)*((_QWORD *)&v45 + 1);
        if ( *((_QWORD *)&v45 + 1) )
        {
          if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v45 + 1) + 8LL), 0xFFFFFFFF) == 1 )
          {
            (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
            if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
              (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
          }
        }
      }
    }
    v22 = *a2;
    if ( *a2 )
      goto LABEL_63;
    v38 = sub_146E8C7D0(&unk_14A3F4530);
    LOBYTE(v39) = 1;
    v40 = sub_144724390(v59, v38, v39, 0);
    v47 = 0;
    v47 = *(_OWORD *)v40;
    v12 = *((_QWORD *)&v47 + 1);
    v41 = v47;
    *(_QWORD *)v40 = 0;
    *(_QWORD *)(v40 + 8) = 0;
    *(_QWORD *)&v47 = *a2;
    *a2 = v41;
    *((_QWORD *)&v47 + 1) = a2[1];
    v42 = (volatile signed __int32 *)*((_QWORD *)&v47 + 1);
    a2[1] = v12;
    if ( v42 )
    {
      if ( _InterlockedExchangeAdd(v42 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v42)(v42);
        if ( _InterlockedExchangeAdd(v42 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v42 + 8LL))(v42);
      }
    }
    v28 = v60;
  }
  if ( v28 )
  {
    if ( _InterlockedExchangeAdd(v28 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v28)(v28);
      if ( _InterlockedExchangeAdd(v28 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v28 + 8LL))(v28);
    }
  }
  v22 = *a2;
  if ( *a2 )
  {
LABEL_63:
    LOBYTE(v12) = 1;
    sub_146B649E0(v22, v12);
  }
  return a2;
}


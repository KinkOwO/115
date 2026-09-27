// method_sub_1441DD8B0_0x1441dd8b0

__int64 __fastcall sub_1441DD8B0(_QWORD *a1)
{
  __int16 v2; // bx
  __int64 v3; // rcx
  bool v4; // r15
  __int64 v5; // rsi
  __int64 v6; // r14
  volatile signed __int32 *v7; // rbx
  unsigned __int64 v8; // rdx
  __int64 v9; // rax
  void (__fastcall *v10)(_QWORD *, __int128 *, __int64); // r10
  __int64 v11; // rcx
  volatile signed __int32 *v12; // rsi
  __int16 v13; // bx
  __int64 v14; // rcx
  bool v15; // r15
  __int64 v16; // rsi
  __int64 v17; // r14
  volatile signed __int32 *v18; // rbx
  unsigned __int64 v19; // rdx
  __int64 v20; // rax
  __int64 v21; // rsi
  __int64 v22; // r14
  volatile signed __int32 *v23; // rbx
  unsigned __int64 v24; // rdx
  __int64 v25; // rax
  __int16 v26; // bx
  __int64 result; // rax
  __int64 v28; // rcx
  bool v29; // r15
  __int64 v30; // rsi
  __int64 v31; // r14
  volatile signed __int32 *v32; // rbx
  unsigned __int64 v33; // rdx
  __int64 v34; // rax
  __int64 v35; // rsi
  __int64 v36; // r14
  volatile signed __int32 *v37; // rbx
  unsigned __int64 v38; // rdx
  __int64 v39; // rax
  volatile signed __int32 *v40; // rbx
  __int16 v41; // [rsp+38h] [rbp-D0h]
  __int16 v42; // [rsp+38h] [rbp-D0h]
  __int16 v43; // [rsp+38h] [rbp-D0h]
  char v44; // [rsp+38h] [rbp-D0h]
  __int64 v45; // [rsp+40h] [rbp-C8h] BYREF
  volatile signed __int32 *v46; // [rsp+48h] [rbp-C0h]
  __int128 v47; // [rsp+50h] [rbp-B8h] BYREF
  __int64 v48; // [rsp+60h] [rbp-A8h] BYREF
  char v49; // [rsp+68h] [rbp-A0h]
  __int128 v50; // [rsp+70h] [rbp-98h]
  __int64 v51; // [rsp+80h] [rbp-88h]
  __int64 v52; // [rsp+88h] [rbp-80h] BYREF
  char v53; // [rsp+90h] [rbp-78h]
  __int128 v54; // [rsp+98h] [rbp-70h]
  __int64 v55; // [rsp+A8h] [rbp-60h]
  __int64 v56; // [rsp+B0h] [rbp-58h] BYREF
  char v57; // [rsp+B8h] [rbp-50h]
  __int128 v58; // [rsp+C0h] [rbp-48h]
  __int64 v59; // [rsp+D0h] [rbp-38h]
  __int64 v60; // [rsp+D8h] [rbp-30h] BYREF
  char v61; // [rsp+E0h] [rbp-28h]
  __int128 v62; // [rsp+E8h] [rbp-20h]
  __int64 v63; // [rsp+F8h] [rbp-10h]
  __int64 v64; // [rsp+100h] [rbp-8h] BYREF
  char v65; // [rsp+108h] [rbp+0h]
  __int128 v66; // [rsp+110h] [rbp+8h]
  __int64 v67; // [rsp+120h] [rbp+18h]
  __int64 v68; // [rsp+128h] [rbp+20h]

  v68 = -2;
  v2 = 0;
  v41 = 0;
  sub_1467A2E80(&v45, 730);
  v4 = 0;
  if ( a1[274] )
  {
    v48 = 0;
    v49 = 0;
    v50 = 0;
    v51 = 0;
    v2 = 3;
    v41 = 3;
    if ( (unsigned __int8)sub_144A9E030(v45, 31, &v48) )
      v4 = 1;
  }
  if ( (v2 & 2) != 0 )
  {
    v2 &= ~2u;
    v41 = v2;
    v5 = v50;
    if ( (_QWORD)v50 )
    {
      v6 = *((_QWORD *)&v50 + 1);
      if ( (_QWORD)v50 != *((_QWORD *)&v50 + 1) )
      {
        do
        {
          v7 = *(volatile signed __int32 **)(v5 + 8);
          if ( v7 )
          {
            if ( _InterlockedExchangeAdd(v7 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v7)(v7);
              if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
            }
          }
          v5 += 16;
        }
        while ( v5 != v6 );
        v5 = v50;
        v2 = v41;
      }
      v8 = (v51 - v5) & 0xFFFFFFFFFFFFFFF0uLL;
      v9 = v5;
      if ( v8 >= 0x1000 )
      {
        v8 += 39LL;
        v5 = *(_QWORD *)(v5 - 8);
        if ( (unsigned __int64)(v9 - v5 - 8) > 0x1F )
          sub_148AAF304(v3, v8);
      }
      sub_146E9F3A0(v5, v8);
      v50 = 0;
      v51 = 0;
    }
  }
  if ( v4 )
  {
    sub_146F52530(a1[274]);
    v10 = *(void (__fastcall **)(_QWORD *, __int128 *, __int64))(*a1 + 40LL);
    v47 = 0;
    v11 = a1[275];
    if ( v11 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v11 + 8));
      v11 = a1[275];
      v2 = v41;
    }
    *(_QWORD *)&v47 = a1[274];
    *((_QWORD *)&v47 + 1) = v11;
    v10(a1, &v47, 24);
    v12 = (volatile signed __int32 *)*((_QWORD *)&v47 + 1);
    if ( *((_QWORD *)&v47 + 1) )
    {
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v47 + 1) + 8LL), 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v12)(v12);
        if ( _InterlockedExchangeAdd(v12 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v12 + 8LL))(v12);
      }
      v2 = v41;
    }
  }
  v56 = 0;
  v57 = 0;
  v58 = 0;
  v59 = 0;
  v13 = v2 | 0xC;
  v15 = 1;
  if ( !(unsigned __int8)sub_144A9E030(v45, 81, &v56) )
  {
    v52 = 0;
    v53 = 0;
    v54 = 0;
    v55 = 0;
    v13 |= 0x30u;
    if ( !(unsigned __int8)sub_144A9E030(v45, 19, &v52) )
      v15 = 0;
  }
  if ( (v13 & 0x20) != 0 )
  {
    v13 &= ~0x20u;
    v42 = v13;
    v16 = v54;
    if ( (_QWORD)v54 )
    {
      v17 = *((_QWORD *)&v54 + 1);
      if ( (_QWORD)v54 != *((_QWORD *)&v54 + 1) )
      {
        do
        {
          v18 = *(volatile signed __int32 **)(v16 + 8);
          if ( v18 )
          {
            if ( _InterlockedExchangeAdd(v18 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v18)(v18);
              if ( _InterlockedExchangeAdd(v18 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v18 + 8LL))(v18);
            }
          }
          v16 += 16;
        }
        while ( v16 != v17 );
        v16 = v54;
        v13 = v42;
      }
      v19 = (v55 - v16) & 0xFFFFFFFFFFFFFFF0uLL;
      v20 = v16;
      if ( v19 >= 0x1000 )
      {
        v19 += 39LL;
        v16 = *(_QWORD *)(v16 - 8);
        if ( (unsigned __int64)(v20 - v16 - 8) > 0x1F )
          sub_148AAF304(v14, v19);
      }
      sub_146E9F3A0(v16, v19);
      v54 = 0;
      v55 = 0;
    }
  }
  if ( (v13 & 8) != 0 )
  {
    v13 &= ~8u;
    v43 = v13;
    v21 = v58;
    if ( (_QWORD)v58 )
    {
      v22 = *((_QWORD *)&v58 + 1);
      if ( (_QWORD)v58 != *((_QWORD *)&v58 + 1) )
      {
        do
        {
          v23 = *(volatile signed __int32 **)(v21 + 8);
          if ( v23 )
          {
            if ( _InterlockedExchangeAdd(v23 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v23)(v23);
              if ( _InterlockedExchangeAdd(v23 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v23 + 8LL))(v23);
            }
          }
          v21 += 16;
        }
        while ( v21 != v22 );
        v21 = v58;
        v13 = v43;
      }
      v24 = (v59 - v21) & 0xFFFFFFFFFFFFFFF0uLL;
      v25 = v21;
      if ( v24 >= 0x1000 )
      {
        v24 += 39LL;
        v21 = *(_QWORD *)(v21 - 8);
        if ( (unsigned __int64)(v25 - v21 - 8) > 0x1F )
          sub_148AAF304(v14, v24);
      }
      sub_146E9F3A0(v21, v24);
      v58 = 0;
      v59 = 0;
    }
  }
  if ( v15 )
  {
    sub_146B25D90(a1[14]);
LABEL_85:
    result = (*(__int64 (__fastcall **)(_QWORD *, _QWORD))(*a1 + 32LL))(a1, 0);
    goto LABEL_86;
  }
  v64 = 0;
  v65 = 0;
  v66 = 0;
  v67 = 0;
  v26 = v13 | 0xC0;
  result = sub_144A9E030(v45, 82, &v64);
  v29 = 1;
  if ( !(_BYTE)result )
  {
    v60 = 0;
    v61 = 0;
    v62 = 0;
    v63 = 0;
    v26 |= 0x300u;
    result = sub_144A9E030(v45, 20, &v60);
    if ( !(_BYTE)result )
      v29 = 0;
  }
  if ( (v26 & 0x200) != 0 )
  {
    v44 = v26;
    v30 = v62;
    if ( (_QWORD)v62 )
    {
      v31 = *((_QWORD *)&v62 + 1);
      if ( (_QWORD)v62 != *((_QWORD *)&v62 + 1) )
      {
        do
        {
          v32 = *(volatile signed __int32 **)(v30 + 8);
          if ( v32 )
          {
            if ( _InterlockedExchangeAdd(v32 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v32)(v32);
              if ( _InterlockedExchangeAdd(v32 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v32 + 8LL))(v32);
            }
          }
          v30 += 16;
        }
        while ( v30 != v31 );
        v30 = v62;
        LOBYTE(v26) = v44;
      }
      v33 = (v63 - v30) & 0xFFFFFFFFFFFFFFF0uLL;
      v34 = v30;
      if ( v33 >= 0x1000 )
      {
        v33 += 39LL;
        v30 = *(_QWORD *)(v30 - 8);
        if ( (unsigned __int64)(v34 - v30 - 8) > 0x1F )
          sub_148AAF304(v28, v33);
      }
      result = sub_146E9F3A0(v30, v33);
      v62 = 0;
      v63 = 0;
    }
  }
  if ( (v26 & 0x80u) != 0 )
  {
    v35 = v66;
    if ( (_QWORD)v66 )
    {
      v36 = *((_QWORD *)&v66 + 1);
      if ( (_QWORD)v66 != *((_QWORD *)&v66 + 1) )
      {
        do
        {
          v37 = *(volatile signed __int32 **)(v35 + 8);
          if ( v37 )
          {
            if ( _InterlockedExchangeAdd(v37 + 2, 0xFFFFFFFF) == 1 )
            {
              (**(void (__fastcall ***)(volatile signed __int32 *))v37)(v37);
              if ( _InterlockedExchangeAdd(v37 + 3, 0xFFFFFFFF) == 1 )
                (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v37 + 8LL))(v37);
            }
          }
          v35 += 16;
        }
        while ( v35 != v36 );
        v35 = v66;
      }
      v38 = (v67 - v35) & 0xFFFFFFFFFFFFFFF0uLL;
      v39 = v35;
      if ( v38 >= 0x1000 )
      {
        v38 += 39LL;
        v35 = *(_QWORD *)(v35 - 8);
        if ( (unsigned __int64)(v39 - v35 - 8) > 0x1F )
          sub_148AAF304(v28, v38);
      }
      result = sub_146E9F3A0(v35, v38);
      v66 = 0;
      v67 = 0;
    }
  }
  if ( v29 )
  {
    sub_146B25D40(a1[14]);
    goto LABEL_85;
  }
LABEL_86:
  v40 = v46;
  if ( v46 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v46 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v40)(v40);
      if ( _InterlockedExchangeAdd(v40 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v40 + 8LL))(v40);
    }
  }
  return result;
}


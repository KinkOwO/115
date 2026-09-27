// holder_sub_140F64710_0x140f64710

__int64 __fastcall sub_140F64710(__int64 a1, unsigned __int8 a2)
{
  __int64 result; // rax
  int v5; // eax
  __int64 v6; // rdx
  _QWORD *i; // rbx
  _QWORD *v8; // rdi
  int v9; // eax
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rax
  __int64 v13; // rdx
  __int64 v14; // r8
  __int64 v15; // rcx
  __int64 v16; // rax
  void (__fastcall ***v17)(_QWORD); // rcx
  __int64 v18; // rax
  void (__fastcall ***v19)(_QWORD); // rcx
  __int64 v20; // rcx
  unsigned __int64 v21; // rdx
  _QWORD *v22; // rdi
  _QWORD *j; // rbx
  __int64 v24; // rax
  __int64 v25; // rax
  __int64 v26; // rbp
  __int64 v27; // rax
  __int64 v28; // r14
  __int64 v29; // rax
  __int64 v30; // r14
  __int64 v31; // rax
  __int64 v32; // rax
  __int64 v33; // rax
  __int64 v34; // rcx
  __int64 v35; // rax
  __int64 v36; // rax
  __int64 v37; // rcx
  __int64 v38; // rax
  __int64 v39; // rax
  __int64 v40; // rcx
  __int64 v41; // rax
  __int64 v42; // rax
  __int64 v43; // rcx
  __int64 v44; // rax
  __int64 v45; // rax
  __int64 v46; // rcx
  __int64 v47; // rax
  _BYTE v48[16]; // [rsp+38h] [rbp-70h] BYREF
  __int128 v49; // [rsp+48h] [rbp-60h] BYREF
  __int64 v50; // [rsp+58h] [rbp-50h]
  _BYTE v51[8]; // [rsp+60h] [rbp-48h] BYREF
  __int64 v52; // [rsp+68h] [rbp-40h]

  result = sub_145388560(a1);
  if ( (_BYTE)result )
  {
    if ( a2 )
    {
      sub_145387AB0(a1);
      sub_1402E0390(a1 + 984);
      v5 = sub_145385ED0(a1);
      v49 = 0u;
      v50 = 0;
      sub_145DF44E0(v5, (unsigned int)&v49, 0, 0, 0);
      v8 = (_QWORD *)*((_QWORD *)&v49 + 1);
      for ( i = (_QWORD *)v49; i != v8; ++i )
      {
        if ( (*(_BYTE *)(*i + 348LL) & 1) == 0 )
        {
          if ( !(unsigned __int8)sub_145B8D6B0() )
            continue;
          if ( (unsigned __int8)sub_145B8E3F0(*i) )
          {
            v9 = sub_1409DDAA0(*i);
            if ( v9 == -1 || v9 == 93 )
              continue;
          }
        }
        v10 = *(_QWORD *)(a1 + 160);
        if ( v10 && *(_DWORD *)(v10 + 8) )
          v11 = *(_QWORD *)(a1 + 168);
        else
          v11 = 0;
        v6 = *i;
        v12 = v11 - 48;
        if ( !v11 )
          v12 = 0;
        if ( v6 != v12
          && (*(unsigned __int8 (__fastcall **)(_QWORD))(*(_QWORD *)v6 + 1552LL))(*i)
          && !(unsigned __int8)sub_145B8B020(*i) )
        {
          v13 = *i + 48LL;
          if ( !*i )
            v13 = 0;
          sub_146EA47F0(v51, v13);
          sub_1405B2BF0(a1 + 984, v48, v51);
          v15 = v52;
          if ( v52 && _InterlockedExchangeAdd((volatile signed __int32 *)(v52 + 12), 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v15 + 8LL))(v15);
          LOBYTE(v14) = 1;
          sub_1408BD0A0(a1, *i, v14);
        }
      }
      LOBYTE(v6) = 1;
      sub_140F643A0(a1, v6);
      if ( !qword_14E63AE60 )
      {
        v16 = sub_146E8BA20(336);
        if ( v16 )
          v17 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v16);
        else
          v17 = 0;
        qword_14E63AE60 = (__int64)v17;
        (**v17)(v17);
      }
      sub_1447E6A00();
      if ( !qword_14E63AE68 )
      {
        v18 = sub_146E8BA20(96);
        if ( v18 )
          v19 = (void (__fastcall ***)(_QWORD))sub_142AC5B80(v18);
        else
          v19 = 0;
        qword_14E63AE68 = (__int64)v19;
        (**v19)(v19);
      }
      sub_142AC9AB0();
      v20 = v49;
      if ( (_QWORD)v49 )
      {
        v21 = (v50 - v49) & 0xFFFFFFFFFFFFFFF8uLL;
        if ( v21 >= 0x1000 )
        {
          v21 += 39LL;
          v20 = *(_QWORD *)(v49 - 8);
          if ( (unsigned __int64)(v49 - v20 - 8) > 0x1F )
            sub_148AAF304(v20, v21);
        }
        sub_146E9F3A0(v20, v21);
        v49 = 0;
        v50 = 0;
      }
    }
    else
    {
      sub_14538AE20(a1);
      v22 = *(_QWORD **)(a1 + 992);
      for ( j = (_QWORD *)*v22; j != v22; j = (_QWORD *)*j )
      {
        v24 = j[3];
        if ( v24 && *(_DWORD *)(v24 + 8) )
        {
          v25 = j[4];
          v26 = v25 - 48;
          if ( !v25 )
            v26 = 0;
          if ( v26 )
          {
            sub_145B97380(v26, 0);
            v27 = sub_1450BE260(v26);
            v28 = v27;
            if ( v27 )
            {
              sub_145C3AAF0(v27, 0);
              (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v28 + 2648LL))(v28, 0);
            }
            v29 = sub_1450BE2C0(v26);
            v30 = v29;
            if ( v29 && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v29 + 6048LL))(v29) )
            {
              v31 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v30 + 6048LL))(v30);
              sub_145E4C210(v31, 0);
            }
          }
        }
      }
      sub_1402E0390(a1 + 984);
      if ( (unsigned __int8)sub_145388560(a1) )
      {
        if ( *(float *)(a1 + 1096) >= 0.0 )
        {
          sub_146B35C50();
          *(_DWORD *)(a1 + 1096) = -1082130432;
        }
        if ( *(int *)(a1 + 1100) >= 0 )
        {
          sub_1447EF1D0();
          *(_DWORD *)(a1 + 1100) = -1;
        }
        if ( *(int *)(a1 + 1104) >= 0 )
        {
          sub_1447EF1A0();
          *(_DWORD *)(a1 + 1104) = -1;
        }
        if ( *(int *)(a1 + 1108) >= 0 )
        {
          sub_1447EF090();
          *(_DWORD *)(a1 + 1108) = -1;
        }
      }
    }
    *(_BYTE *)(a1 + 1112) = a2;
    v32 = *(_QWORD *)(a1 + 160);
    if ( v32 && *(_DWORD *)(v32 + 8) )
      v33 = *(_QWORD *)(a1 + 168);
    else
      v33 = 0;
    v34 = v33 - 48;
    if ( !v33 )
      v34 = 0;
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v34 + 2648LL))(v34, a2);
    v35 = *(_QWORD *)(a1 + 160);
    if ( v35 && *(_DWORD *)(v35 + 8) )
      v36 = *(_QWORD *)(a1 + 168);
    else
      v36 = 0;
    v37 = v36 - 48;
    if ( !v36 )
      v37 = 0;
    sub_145C3AAF0(v37, a2);
    v38 = *(_QWORD *)(a1 + 160);
    if ( v38 && *(_DWORD *)(v38 + 8) )
      v39 = *(_QWORD *)(a1 + 168);
    else
      v39 = 0;
    v40 = v39 - 48;
    if ( !v39 )
      v40 = 0;
    sub_145C3E3B0(v40, a2);
    v41 = *(_QWORD *)(a1 + 160);
    if ( v41 && *(_DWORD *)(v41 + 8) )
      v42 = *(_QWORD *)(a1 + 168);
    else
      v42 = 0;
    v43 = v42 - 48;
    if ( !v42 )
      v43 = 0;
    result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v43 + 6048LL))(v43);
    if ( result )
    {
      v44 = *(_QWORD *)(a1 + 160);
      if ( v44 && *(_DWORD *)(v44 + 8) )
        v45 = *(_QWORD *)(a1 + 168);
      else
        v45 = 0;
      v46 = v45 - 48;
      if ( !v45 )
        v46 = 0;
      v47 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v46 + 6048LL))(v46);
      return sub_145E4C210(v47, a2);
    }
  }
  return result;
}


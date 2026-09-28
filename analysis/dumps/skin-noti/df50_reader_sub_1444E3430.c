// reader_sub_1444E3430

__int64 __fastcall sub_1444E3430(__int64 a1)
{
  __int64 v2; // rcx
  __int64 result; // rax
  __int64 v4; // rcx
  __int64 v5; // rax
  unsigned int v6; // eax
  __int64 v7; // rdx
  __int64 v8; // rcx
  __int64 v9; // rcx
  __int64 v10; // rax
  void (__fastcall ***v11)(_QWORD); // rcx
  _WORD *v12; // rbp
  unsigned int v13; // ebx
  int v14; // ebx
  int v15; // r14d
  __int64 v16; // r8
  __int64 v17; // rdx
  __int64 v18; // rcx
  __int64 v19; // rcx
  __int64 v20; // rdx
  int i; // ebx
  int v22; // ebp
  __int64 *v23; // rsi
  _DWORD *v24; // r13
  __int64 v25; // r14
  unsigned int v26; // ebx
  unsigned int v27; // r15d
  __int64 v28; // rdx
  __int64 v29; // rcx
  __int64 v30; // rdx
  __int64 v31; // r8
  __int64 v32; // rcx
  __int64 v33; // rax
  void (__fastcall ***v34)(_QWORD); // rcx
  __int64 v35; // rdx
  __int64 v36; // rcx
  __int64 v37; // rdx
  __int64 v38; // r8
  __int64 v39; // rcx
  __int64 v40; // rax
  void (__fastcall ***v41)(_QWORD); // rcx
  __int64 v42; // rdx
  __int64 v43; // r8
  __int64 v44; // rcx
  __int64 v45; // rax
  void (__fastcall ***v46)(_QWORD); // rcx
  __int64 v47; // rdx
  __int64 v48; // rcx
  __int64 v49; // rdx
  __int64 v50; // r8
  __int64 v51; // rcx
  __int64 v52; // rax
  void (__fastcall ***v53)(_QWORD); // rcx
  __int64 v54; // rcx
  __int64 v55; // rax
  void (__fastcall ***v56)(_QWORD); // rcx
  __int64 v57; // rcx
  __int64 v58; // rax
  void (__fastcall ***v59)(_QWORD); // rcx

  v2 = *(_QWORD *)(a1 + 3864);
  if ( v2 && (unsigned __int8)sub_141FB6530(v2) )
  {
    result = sub_146AEF960(*(_QWORD *)(a1 + 3864));
    if ( !(_BYTE)result )
      return result;
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3864) + 16LL))(*(_QWORD *)(a1 + 3864), 0);
    sub_1444E4E40(a1);
    v5 = sub_1444D2BB0(v4);
    v6 = sub_1401B63C0(v5);
    sub_1444DD420(a1, v6, 0xFFFFFFFFLL);
    v8 = *(_QWORD *)(a1 + 2568);
    if ( v8 )
    {
      LOBYTE(v7) = 1;
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v8 + 24LL))(v8, v7);
    }
  }
  v9 = qword_14E664BF8;
  if ( !qword_14E664BF8 )
  {
    v10 = sub_146E8BA20(2792);
    if ( v10 )
      v11 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v10);
    else
      v11 = 0;
    qword_14E664BF8 = (__int64)v11;
    (**v11)(v11);
    v9 = qword_14E664BF8;
  }
  v12 = (_WORD *)sub_1444D2A20(v9);
  if ( v12 )
  {
    v13 = 0;
    if ( (*(_DWORD *)(a1 + 2508) & 2) != 0 )
    {
      v14 = sub_146E9F840(a1 + 2504);
      v15 = sub_140193D40(a1 + 2504);
      if ( (unsigned __int8)sub_146E9FA80(a1 + 2504) )
      {
        v14 = v15;
        sub_146E9FBD0(a1 + 2504, 1000, 0);
      }
      if ( v14 >= 500 )
      {
        v16 = (unsigned int)(v14 - 500);
        v17 = 1;
        v18 = 255;
      }
      else
      {
        v16 = (unsigned int)v14;
        v17 = 255;
        v18 = 1;
      }
      v13 = sub_146EA1750(v18, v17, v16, 500);
    }
    v19 = *(_QWORD *)(a1 + 2568);
    if ( v19 )
    {
      if ( (unsigned __int8)sub_146ED0010(v19) )
      {
        v20 = 255;
LABEL_25:
        (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2568) + 376LL))(*(_QWORD *)(a1 + 2568), v20);
        goto LABEL_26;
      }
      if ( !*v12 || ((*(_QWORD *)(a1 + 4360) - *(_QWORD *)(a1 + 4352)) & 0xFFFFFFFFFFFFFFFCuLL) == 0 )
      {
        v20 = v13;
        goto LABEL_25;
      }
    }
  }
LABEL_26:
  for ( i = 46; i < 52; ++i )
  {
    if ( (*(unsigned __int8 (__fastcall **)(__int64, _QWORD, _QWORD))(*(_QWORD *)qword_14F1C0F28 + 48LL))(
           qword_14F1C0F28,
           (unsigned int)i,
           0) )
    {
      sub_1444E5E80(a1, (unsigned int)(i - 46));
    }
  }
  v22 = 0;
  v23 = (__int64 *)(a1 + 3688);
  v24 = (_DWORD *)(a1 + 4028);
  v25 = a1 + 4024;
  do
  {
    result = *v24 >> 1;
    if ( (*v24 & 2) != 0 )
    {
      v26 = sub_146E9F840(v25);
      v27 = sub_140193D40(v25);
      result = sub_146E9FA80(v25);
      if ( (_BYTE)result )
      {
        v26 = v27;
        result = sub_146E9FBC0(v25);
      }
      if ( *v23 )
      {
        result = sub_141FB6530(*v23);
        if ( (_BYTE)result )
        {
          sub_1444D8010(a1, (unsigned int)v22);
          sub_146ECA0C0(*v23);
          sub_146EA1820(v29, v28, v26, v27);
          v32 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v33 = sub_146E8BA20(2792);
            if ( v33 )
              v34 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v33);
            else
              v34 = 0;
            qword_14E664BF8 = (__int64)v34;
            (**v34)(v34);
            v32 = qword_14E664BF8;
          }
          sub_1444D2C40(v32, v30, v31);
          sub_142757420(*(_QWORD *)(a1 + 3672));
          sub_142757420(*v23);
          sub_146EA1820(v36, v35, v26, v27);
          v39 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v40 = sub_146E8BA20(2792);
            if ( v40 )
              v41 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v40);
            else
              v41 = 0;
            qword_14E664BF8 = (__int64)v41;
            (**v41)(v41);
            v39 = qword_14E664BF8;
          }
          sub_1444D29D0(v39, v37, v38);
          v44 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v45 = sub_146E8BA20(2792);
            if ( v45 )
              v46 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v45);
            else
              v46 = 0;
            qword_14E664BF8 = (__int64)v46;
            (**v46)(v46);
            v44 = qword_14E664BF8;
          }
          sub_1444D2C30(v44, v42, v43);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)*v23 + 664LL))(*v23);
          sub_146EA1820(v48, v47, v26, v27);
          v51 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v52 = sub_146E8BA20(2792);
            if ( v52 )
              v53 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v52);
            else
              v53 = 0;
            qword_14E664BF8 = (__int64)v53;
            (**v53)(v53);
            v51 = qword_14E664BF8;
          }
          sub_1444D2C30(v51, v49, v50);
          v54 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v55 = sub_146E8BA20(2792);
            if ( v55 )
              v56 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v55);
            else
              v56 = 0;
            qword_14E664BF8 = (__int64)v56;
            (**v56)(v56);
            v54 = qword_14E664BF8;
          }
          sub_140B84E90(v54);
          v57 = qword_14E664BF8;
          if ( !qword_14E664BF8 )
          {
            v58 = sub_146E8BA20(2792);
            if ( v58 )
              v59 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v58);
            else
              v59 = 0;
            qword_14E664BF8 = (__int64)v59;
            (**v59)(v59);
            v57 = qword_14E664BF8;
          }
          sub_140E49440(v57);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)*v23 + 112LL))(*v23);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)*v23 + 144LL))(*v23);
          (*(void (__fastcall **)(__int64))(*(_QWORD *)*v23 + 152LL))(*v23);
          result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)*v23 + 320LL))(*v23);
        }
      }
    }
    ++v22;
    v25 += 32;
    v24 += 8;
    v23 += 2;
  }
  while ( v22 < 8 );
  return result;
}


// caller_f1090_0x1441d5da0

__int64 __fastcall sub_1441D5DA0(__int64 a1, _QWORD *a2, int a3)
{
  _QWORD *v3; // r12
  __int64 v5; // rax
  __int64 v6; // rax
  __int64 v7; // rdx
  __int64 v8; // r8
  __int64 v9; // r15
  unsigned int *i; // r13
  __int64 v11; // rax
  __int64 v12; // rcx
  __int64 v13; // rax
  void (__fastcall ***v14)(_QWORD); // rcx
  __int128 v15; // rdi
  _DWORD *v16; // rbx
  __int64 v17; // rax
  int v18; // ecx
  int v19; // ecx
  __int64 v20; // rax
  int v21; // eax
  int v22; // eax
  __int64 v23; // r12
  __int64 v24; // rax
  void (__fastcall ***v25)(_QWORD); // rcx
  __int64 v26; // rsi
  __int64 v27; // r15
  __int64 v28; // rdi
  __int64 v29; // rbx
  __int64 v30; // rcx
  unsigned __int64 v31; // rdx
  __int64 v32; // rax
  int v33; // eax
  __int64 v34; // rcx
  int v35; // eax
  void *v36; // r8
  __int64 v37; // r15
  __int64 v38; // rax
  void (__fastcall ***v39)(_QWORD); // rcx
  __int64 v40; // rsi
  __int64 v41; // r12
  __int64 v42; // rdi
  __int64 v43; // rbx
  __int64 v44; // rax
  __int64 v45; // rcx
  unsigned __int64 v46; // rdx
  __int64 v47; // rcx
  unsigned __int64 v48; // rdx
  __int64 v49; // rax
  __int64 v50; // rax
  __int64 v51; // rdx
  __int128 v53; // [rsp+48h] [rbp-C0h] BYREF
  __int64 v54; // [rsp+58h] [rbp-B0h]
  __int64 v55; // [rsp+60h] [rbp-A8h]
  __int128 v56; // [rsp+68h] [rbp-A0h] BYREF
  _DWORD *v57; // [rsp+78h] [rbp-90h]
  __int128 v58; // [rsp+80h] [rbp-88h] BYREF
  __int64 v59; // [rsp+90h] [rbp-78h]
  __int128 v60; // [rsp+98h] [rbp-70h] BYREF
  __int64 v61; // [rsp+A8h] [rbp-60h]
  __int128 v62; // [rsp+B0h] [rbp-58h] BYREF
  __int64 v63; // [rsp+C0h] [rbp-48h]
  __int128 *v64; // [rsp+C8h] [rbp-40h]
  __int128 *v65; // [rsp+D0h] [rbp-38h]
  __int64 v66; // [rsp+D8h] [rbp-30h]
  __int64 v67; // [rsp+E0h] [rbp-28h]
  __int64 v68; // [rsp+E8h] [rbp-20h]
  __int128 *v69; // [rsp+F0h] [rbp-18h]
  __int64 v70; // [rsp+F8h] [rbp-10h]
  __int128 v71; // [rsp+100h] [rbp-8h]
  _QWORD *v72; // [rsp+160h] [rbp+58h]
  int v73; // [rsp+168h] [rbp+60h]

  v72 = a2;
  v66 = -2;
  v3 = a2;
  if ( a3 != 13 )
  {
    if ( a3 == 12 )
    {
      sub_142757420(*a2);
      sub_146ECA0C0(*v3);
      sub_1441D9EE0(a1);
    }
    else if ( a3 == 24 )
    {
      v50 = *a2;
      if ( *a2 == *(_QWORD *)(a1 + 112) )
      {
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)a1 + 32LL))(a1, 0);
      }
      else if ( v50 == *(_QWORD *)(a1 + 40) )
      {
        LOBYTE(a2) = 1;
        (*(void (__fastcall **)(__int64, _QWORD *))(*(_QWORD *)a1 + 32LL))(a1, a2);
      }
      else if ( v50 == *(_QWORD *)(a1 + 2352) )
      {
        (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 104LL))(a1);
        *(_DWORD *)(a1 + 128) = -1;
        *(_DWORD *)(a1 + 2344) = sub_146F50350(*(_QWORD *)(a1 + 2352));
        (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 24LL))(a1);
        LOBYTE(v51) = 1;
        (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 32LL))(a1, v51);
      }
    }
    else if ( a3 == 6 && *a2 == *(_QWORD *)(a1 + 136) )
    {
      sub_14417BD90(*(_QWORD *)(a1 + 8));
    }
    return 0;
  }
  v5 = *(_QWORD *)(a1 + 72);
  if ( v5 && v5 == *a2 )
  {
    (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 112LL))(a1);
  }
  else
  {
    v6 = *(_QWORD *)(a1 + 88);
    if ( v6 && v6 == *a2 )
      (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 104LL))(a1);
  }
  sub_1421B2800(*(_QWORD *)(a1 + 112));
  v73 = 0;
  v9 = 0;
  v55 = 0;
  for ( i = (unsigned int *)(a1 + 192); ; i += 100 )
  {
    v11 = sub_1444EBAB0(*i, v7, v8);
    if ( !v11 || *(_DWORD *)(v11 + 8) )
      goto LABEL_76;
    v12 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v13 = sub_146E8BA20(1472);
      v67 = v13;
      if ( v13 )
        v14 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v13);
      else
        v14 = 0;
      qword_14E638F28 = (__int64)v14;
      (**v14)(v14);
      v12 = qword_14E638F28;
    }
    sub_1444EBC10(v12, &v60, 0);
    v56 = 0;
    v57 = 0;
    v15 = v60;
    if ( (_QWORD)v60 != *((_QWORD *)&v60 + 1) )
    {
      v16 = (_DWORD *)*((_QWORD *)&v56 + 1);
      do
      {
        v17 = sub_1444EBAB0(*(_DWORD *)v15, v7, v8);
        if ( v17 )
        {
          v7 = *(unsigned int *)(a1 + 2344);
          v18 = *(_DWORD *)(v17 + 12);
          if ( v18 )
          {
            v19 = v18 - 1;
            if ( v19 )
            {
              if ( v19 == 1 )
              {
                if ( (_DWORD)v7 != 3 )
                  goto LABEL_29;
              }
              else if ( (unsigned int)(v7 - 1) <= 2 )
              {
                goto LABEL_29;
              }
            }
            else if ( (_DWORD)v7 != 2 )
            {
              goto LABEL_29;
            }
          }
          else if ( (_DWORD)v7 != 1 )
          {
LABEL_29:
            if ( v16 == v57 )
            {
              sub_140154010(&v56, v16, v15);
              v16 = (_DWORD *)*((_QWORD *)&v56 + 1);
            }
            else
            {
              *v16++ = *(_DWORD *)v15;
              *((_QWORD *)&v56 + 1) = v16;
            }
          }
        }
        *(_QWORD *)&v15 = v15 + 4;
      }
      while ( (_QWORD)v15 != *((_QWORD *)&v15 + 1) );
    }
    v20 = *v3;
    if ( *v3 == *((_QWORD *)i + 1) )
    {
      sub_1441E0820(*(_QWORD *)(a1 + 8), 0, *i, 0);
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)a1 + 32LL))(a1, 0);
      v21 = sub_146E8C7D0(&unk_149242AE8);
      sub_145A31380(v21, -1, 0, 0, -1, -1, 0);
      goto LABEL_68;
    }
    if ( v20 == *((_QWORD *)i + 26) )
    {
      v22 = sub_146E8C7D0(&unk_149242AE8);
      sub_145A31380(v22, -1, 0, 0, -1, -1, 0);
      v62 = 0;
      v63 = 0;
      sub_140154010(&v62, 0, i);
      sub_1441E1DD0(a1, &v56, &v62, *(unsigned int *)(a1 + 2344));
      v23 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        v24 = sub_146E8BA20(1472);
        v68 = v24;
        if ( v24 )
          v25 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v24);
        else
          v25 = 0;
        qword_14E638F28 = (__int64)v25;
        (**v25)(v25);
        v23 = qword_14E638F28;
      }
      v69 = &v53;
      v53 = 0;
      v54 = 0;
      v26 = v62;
      if ( (_QWORD)v62 != *((_QWORD *)&v62 + 1) )
      {
        v27 = *((_QWORD *)&v62 + 1) - v62;
        v28 = (__int64)(*((_QWORD *)&v62 + 1) - v62) >> 2;
        *(_QWORD *)&v53 = sub_140157580(&v53, v28);
        *((_QWORD *)&v53 + 1) = v53;
        v54 = v53 + 4 * v28;
        v64 = &v53;
        v29 = v53;
        sub_148AA1E60(v53, v26, v27);
        *((_QWORD *)&v53 + 1) = v29 + 4 * v28;
        v64 = 0;
      }
      sub_1444F1090(v23, 0, &v53);
      if ( v26 )
      {
        v31 = 4 * ((v63 - v26) >> 2);
        v32 = v26;
        if ( v31 >= 0x1000 )
        {
          v31 += 39LL;
          v26 = *(_QWORD *)(v26 - 8);
          if ( (unsigned __int64)(v32 - v26 - 8) > 0x1F )
            goto LABEL_100;
        }
        sub_146E9F3A0(v26, v31);
        v62 = 0;
        v63 = 0;
      }
      goto LABEL_67;
    }
    if ( v20 == *((_QWORD *)i + 28) )
      break;
LABEL_68:
    v45 = v56;
    if ( (_QWORD)v56 )
    {
      v46 = 4 * ((__int64)((__int64)v57 - v56) >> 2);
      if ( v46 >= 0x1000 )
      {
        v46 += 39LL;
        v45 = *(_QWORD *)(v56 - 8);
        if ( (unsigned __int64)(v56 - v45 - 8) > 0x1F )
          goto LABEL_101;
      }
      sub_146E9F3A0(v45, v46);
      v56 = 0;
      v57 = 0;
    }
    v47 = v60;
    if ( (_QWORD)v60 )
    {
      v48 = (v61 - v60) & 0xFFFFFFFFFFFFFFFCuLL;
      if ( v48 >= 0x1000 )
      {
        v48 += 39LL;
        v47 = *(_QWORD *)(v60 - 8);
        if ( (unsigned __int64)(v60 - v47 - 8) > 0x1F )
          goto LABEL_99;
      }
      sub_146E9F3A0(v47, v48);
      v60 = 0;
      v61 = 0;
    }
LABEL_76:
    ++v73;
    v55 = ++v9;
    if ( v9 >= 5 )
      return 0;
  }
  v33 = sub_146E8C7D0(&unk_149242AE8);
  sub_145A31380(v33, -1, 0, 0, -1, -1, 0);
  v58 = 0;
  v59 = 0;
  v35 = *(_DWORD *)(a1 + 2344);
  if ( v35 != 3 )
  {
    switch ( v35 )
    {
      case 1:
        v36 = &unk_14A22E3A4;
        goto LABEL_55;
      case 0:
        v36 = &unk_14A22E3A0;
        goto LABEL_55;
      case 2:
        v36 = &unk_14A22E3A8;
LABEL_55:
        sub_140154010(&v58, 0, v36);
        break;
    }
    sub_1441E1DD0(a1, &v56, &v58, *(unsigned int *)(a1 + 2344));
    v37 = qword_14E638F28;
    if ( !qword_14E638F28 )
    {
      v38 = sub_146E8BA20(1472);
      v70 = v38;
      if ( v38 )
        v39 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v38);
      else
        v39 = 0;
      qword_14E638F28 = (__int64)v39;
      (**v39)(v39);
      v37 = qword_14E638F28;
    }
    *(_QWORD *)&v71 = &v53;
    v53 = 0;
    v54 = 0;
    v40 = v58;
    if ( (_QWORD)v58 != *((_QWORD *)&v58 + 1) )
    {
      v41 = *((_QWORD *)&v58 + 1) - v58;
      v42 = (__int64)(*((_QWORD *)&v58 + 1) - v58) >> 2;
      *(_QWORD *)&v53 = sub_140157580(&v53, v42);
      *((_QWORD *)&v53 + 1) = v53;
      v54 = v53 + 4 * v42;
      v65 = &v53;
      v43 = v53;
      sub_148AA1E60(v53, v40, v41);
      *((_QWORD *)&v53 + 1) = v43 + 4 * v42;
      v65 = 0;
    }
    sub_1444F1090(v37, 0, &v53);
    if ( v40 )
    {
      v31 = 4 * ((v59 - v40) >> 2);
      v44 = v40;
      if ( v31 >= 0x1000 )
      {
        v31 += 39LL;
        v40 = *(_QWORD *)(v40 - 8);
        if ( (unsigned __int64)(v44 - v40 - 8) > 0x1F )
LABEL_100:
          sub_148AAF304(v30, v31);
      }
      sub_146E9F3A0(v40, v31);
      v58 = 0;
      v59 = 0;
    }
LABEL_67:
    v3 = v72;
    v9 = v55;
    goto LABEL_68;
  }
  v49 = sub_140764510(v34);
  sub_1444F1880(v49, 0, *(_DWORD *)(400LL * v73 + a1 + 192));
  v45 = v56;
  if ( (_QWORD)v56 )
  {
    v46 = 4 * ((__int64)((__int64)v57 - v56) >> 2);
    if ( v46 >= 0x1000 )
    {
      v46 += 39LL;
      v45 = *(_QWORD *)(v56 - 8);
      if ( (unsigned __int64)(v56 - v45 - 8) > 0x1F )
LABEL_101:
        sub_148AAF304(v45, v46);
    }
    sub_146E9F3A0(v45, v46);
    v56 = 0;
    v57 = 0;
  }
  v47 = v60;
  if ( (_QWORD)v60 )
  {
    v48 = (v61 - v60) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v48 >= 0x1000 )
    {
      v48 += 39LL;
      v47 = *(_QWORD *)(v60 - 8);
      if ( (unsigned __int64)(v60 - v47 - 8) > 0x1F )
LABEL_99:
        sub_148AAF304(v47, v48);
    }
    sub_146E9F3A0(v47, v48);
    v60 = 0;
    v61 = 0;
  }
  return 0;
}


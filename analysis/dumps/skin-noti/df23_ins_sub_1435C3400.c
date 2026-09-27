// ins_sub_1435C3400

void __fastcall sub_1435C3400(__int64 a1, int a2, __int64 a3)
{
  __int64 v3; // r15
  __int64 v5; // r14
  __int64 v6; // r13
  _QWORD *v7; // rdi
  __int64 v8; // r12
  __int64 v9; // rsi
  __int64 v10; // rbx
  bool v11; // zf
  _QWORD *v12; // rsi
  int v13; // r10d
  _QWORD *v14; // r13
  __int64 v15; // r8
  _QWORD *v16; // rbx
  __int64 v17; // r15
  int v18; // r12d
  __int64 v19; // r9
  __int64 v20; // r14
  __int64 v21; // rax
  int v22; // edx
  unsigned int *v23; // r15
  unsigned __int64 v24; // rbx
  unsigned __int64 v25; // r12
  _QWORD *v26; // r14
  __int64 v27; // r13
  __int64 v28; // rdi
  __int64 v29; // rdx
  char *v30; // rcx
  unsigned __int64 v31; // rsi
  unsigned __int64 v32; // rdx
  __int64 *v33; // r9
  __int64 *v34; // rcx
  __int64 *v35; // rdx
  __int64 v36; // r8
  signed __int8 v37; // r11
  signed __int8 v38; // r10
  bool v39; // cc
  char v40; // al
  bool v41; // cc
  int v42; // eax
  unsigned __int64 v43; // rsi
  __int64 v44; // r14
  _QWORD *v45; // rbx
  __int64 v46; // r15
  unsigned __int64 v47; // rdx
  char *v48; // rbx
  char *v49; // rsi
  __int64 v50; // rcx
  unsigned __int64 v51; // rdx
  __int64 v52; // r8
  __int64 v53; // rcx
  __int64 v54; // rcx
  unsigned __int64 v55; // rdx
  __int64 v56; // r8
  __int64 v57; // rcx
  __int64 v58; // rcx
  unsigned __int64 v59; // rdx
  __int64 v60; // r8
  __int64 v61; // rcx
  __int64 v62; // rcx
  __int64 v63; // r8
  signed __int64 v64; // rcx
  char *v65; // rax
  __int64 v66; // [rsp+20h] [rbp-20h]
  __int128 v67; // [rsp+28h] [rbp-18h] BYREF
  _QWORD *v68; // [rsp+38h] [rbp-8h]
  int v70; // [rsp+88h] [rbp+48h]
  bool v71; // [rsp+88h] [rbp+48h]
  _QWORD *v72; // [rsp+90h] [rbp+50h] BYREF

  v66 = -2;
  v3 = a2;
  v5 = a2;
  LOBYTE(a3) = 1;
  v72 = (_QWORD *)sub_140283D60(qword_14E683B38, *(unsigned int *)(a1 + 4LL * a2 + 1648), a3);
  if ( !v72 )
    return;
  v6 = a1 + 816 * v5;
  v7 = (_QWORD *)(v6 + 1936);
  v8 = 6;
  v9 = 6;
  do
  {
    v10 = *v7;
    sub_141FB6530(*v7);
    sub_146F01920(v10, 0);
    v7 += 17;
    --v9;
  }
  while ( v9 );
  v11 = (_DWORD)v3 == 0;
  if ( (_DWORD)v3 )
  {
    if ( (_DWORD)v3 != 1 )
    {
      if ( (_DWORD)v3 == 2 )
      {
        v67 = 0;
        v12 = 0;
        v68 = 0;
        v13 = 0;
        v70 = 0;
        v14 = v72;
        v15 = v72[329];
        v16 = 0;
        if ( (v72[330] - v15) / 80 )
        {
          v17 = 0;
          do
          {
            v18 = 0;
            v19 = v15;
            if ( (*(_QWORD *)(v17 + v15 + 56) - *(_QWORD *)(v17 + v15 + 48)) / 144LL )
            {
              v20 = 0;
              do
              {
                v21 = *(_QWORD *)(v17 + v15 + 48);
                v22 = *(_DWORD *)(v21 + v20 + 8);
                LODWORD(v72) = *(_DWORD *)(v21 + v20);
                HIDWORD(v72) = v22;
                if ( v16 == v12 )
                {
                  sub_140183FB0(&v67, v16, &v72);
                  v12 = v68;
                  v16 = (_QWORD *)*((_QWORD *)&v67 + 1);
                }
                else
                {
                  *v16++ = v72;
                  *((_QWORD *)&v67 + 1) = v16;
                }
                ++v18;
                v20 += 144;
                v19 = v14[329];
                v15 = v19;
              }
              while ( v18 < (unsigned __int64)((*(_QWORD *)(v17 + v19 + 56) - *(_QWORD *)(v17 + v19 + 48)) / 144LL) );
              v13 = v70;
            }
            v70 = ++v13;
            v17 += 80;
            v15 = v19;
          }
          while ( v13 < (unsigned __int64)((v14[330] - v19) / 80) );
        }
        v23 = (unsigned int *)v67;
        v24 = (__int64)((__int64)v16 - v67) >> 3;
        v25 = 0;
        v26 = (_QWORD *)(a1 + 3520);
        v27 = a1 + 3512;
        v28 = 6;
        do
        {
          (*(void (__fastcall **)(_QWORD, _QWORD, __int64))(*(_QWORD *)*v26 + 16LL))(*v26, 0, v15);
          if ( v24 > v25 )
          {
            LOBYTE(v29) = 1;
            (*(void (__fastcall **)(_QWORD, __int64))(*(_QWORD *)*v26 + 16LL))(*v26, v29);
            sub_1435C3030(v27, *v23, v23[1], 0, v66);
          }
          ++v25;
          v27 += 136;
          v26 += 17;
          v23 += 2;
          --v28;
        }
        while ( v28 );
        v30 = (char *)v67;
        if ( (_QWORD)v67 )
        {
          v31 = ((unsigned __int64)v12 - v67) & 0xFFFFFFFFFFFFFFF8uLL;
          if ( v31 >= 0x1000 )
          {
            v31 += 39LL;
            v30 = *(char **)(v67 - 8);
            if ( (unsigned __int64)(v67 - (_QWORD)v30 - 8) > 0x1F )
              sub_148AAF304(v30, v29);
          }
          v32 = v31;
LABEL_80:
          sub_146E9F3A0(v30, v32);
          v68 = 0;
          v67 = 0;
          return;
        }
      }
      return;
    }
    v11 = 0;
  }
  v71 = v11;
  v67 = 0;
  v68 = 0;
  v33 = (__int64 *)v72[334];
  v34 = (__int64 *)v33[1];
  v35 = v33;
  v36 = 1;
  v37 = *(_DWORD *)(a1 + 4 * v3 + 1684);
  v38 = *(_DWORD *)(a1 + 4 * v3 + 1672);
  if ( !*((_BYTE *)v34 + 25) )
  {
    do
    {
      v39 = *((_BYTE *)v34 + 32) < (unsigned __int8)v38;
      if ( *((_BYTE *)v34 + 32) == v38
        && (v39 = *((_BYTE *)v34 + 33) < (unsigned __int8)v37, *((_BYTE *)v34 + 33) == v37) )
      {
        v40 = 0;
      }
      else
      {
        v40 = 1;
        if ( v39 )
          v40 = -1;
      }
      if ( v40 >= 0 )
      {
        v33 = v34;
        v34 = (__int64 *)*v34;
      }
      else
      {
        v34 = (__int64 *)v34[2];
      }
    }
    while ( !*((_BYTE *)v34 + 25) );
    v35 = (__int64 *)v72[334];
  }
  if ( !*((_BYTE *)v33 + 25) )
  {
    v41 = v38 < *((_BYTE *)v33 + 32);
    if ( v38 == *((_BYTE *)v33 + 32) && (v41 = v37 < *((_BYTE *)v33 + 33), v37 == *((_BYTE *)v33 + 33)) )
    {
      LOBYTE(v36) = 0;
    }
    else if ( v41 )
    {
      v36 = 255;
    }
    if ( (v36 & 0x80u) == 0LL && v33 != v35 )
      sub_1435BEEB0(&v67, v33 + 5);
  }
  if ( (_DWORD)v3 )
    v42 = sub_1421B2800(*(_QWORD *)(a1 + 4360)) - 1;
  else
    v42 = 0;
  v43 = 6 * v42;
  v44 = 864LL * v42;
  v45 = (_QWORD *)(v6 + 1888);
  v46 = v6 + 1880;
  do
  {
    (*(void (__fastcall **)(_QWORD, _QWORD, __int64))(*(_QWORD *)*v45 + 16LL))(*v45, 0, v36);
    v47 = (*((_QWORD *)&v67 + 1) - (_QWORD)v67) / 144LL;
    if ( v47 > v43 )
    {
      LOBYTE(v47) = 1;
      (*(void (__fastcall **)(_QWORD, unsigned __int64))(*(_QWORD *)*v45 + 16LL))(*v45, v47);
      sub_1435C3030(v46, *(unsigned int *)(v44 + v67), *(unsigned int *)(v44 + v67 + 8), v71, v66);
    }
    ++v43;
    v46 += 136;
    v45 += 17;
    v44 += 144;
    --v8;
  }
  while ( v8 );
  v48 = (char *)v67;
  if ( (_QWORD)v67 )
  {
    v49 = (char *)*((_QWORD *)&v67 + 1);
    if ( (_QWORD)v67 != *((_QWORD *)&v67 + 1) )
    {
      do
      {
        v50 = *((_QWORD *)v48 + 15);
        if ( v50 )
        {
          v51 = 4 * ((*((_QWORD *)v48 + 17) - v50) >> 2);
          if ( v51 >= 0x1000 )
          {
            v51 += 39LL;
            v52 = *(_QWORD *)(v50 - 8);
            v53 = v50 - v52;
            if ( (unsigned __int64)(v53 - 8) > 0x1F )
              sub_148AAF304(v53, v51);
            v50 = v52;
          }
          sub_146E9F3A0(v50, v51);
          *((_QWORD *)v48 + 15) = 0;
          *((_QWORD *)v48 + 16) = 0;
          *((_QWORD *)v48 + 17) = 0;
        }
        v54 = *((_QWORD *)v48 + 11);
        if ( v54 )
        {
          v55 = 4 * ((*((_QWORD *)v48 + 13) - v54) >> 2);
          if ( v55 >= 0x1000 )
          {
            v55 += 39LL;
            v56 = *(_QWORD *)(v54 - 8);
            v57 = v54 - v56;
            if ( (unsigned __int64)(v57 - 8) > 0x1F )
              sub_148AAF304(v57, v55);
            v54 = v56;
          }
          sub_146E9F3A0(v54, v55);
          *((_QWORD *)v48 + 11) = 0;
          *((_QWORD *)v48 + 12) = 0;
          *((_QWORD *)v48 + 13) = 0;
        }
        v58 = *((_QWORD *)v48 + 6);
        if ( v58 )
        {
          v59 = 8 * ((*((_QWORD *)v48 + 8) - v58) >> 3);
          if ( v59 >= 0x1000 )
          {
            v59 += 39LL;
            v60 = *(_QWORD *)(v58 - 8);
            v61 = v58 - v60;
            if ( (unsigned __int64)(v61 - 8) > 0x1F )
              sub_148AAF304(v61, v59);
            v58 = v60;
          }
          sub_146E9F3A0(v58, v59);
          *((_QWORD *)v48 + 6) = 0;
          *((_QWORD *)v48 + 7) = 0;
          *((_QWORD *)v48 + 8) = 0;
        }
        v62 = *((_QWORD *)v48 + 3);
        if ( v62 )
        {
          v32 = (*((_QWORD *)v48 + 5) - v62) & 0xFFFFFFFFFFFFFFF8uLL;
          if ( v32 >= 0x1000 )
          {
            v32 += 39LL;
            v63 = *(_QWORD *)(v62 - 8);
            v64 = v62 - v63;
            if ( (unsigned __int64)(v64 - 8) > 0x1F )
              goto LABEL_84;
            v62 = v63;
          }
          sub_146E9F3A0(v62, v32);
          *((_QWORD *)v48 + 3) = 0;
          *((_QWORD *)v48 + 4) = 0;
          *((_QWORD *)v48 + 5) = 0;
        }
        v48 += 144;
      }
      while ( v48 != v49 );
      v48 = (char *)v67;
    }
    v64 = (char *)v68 - v48;
    v32 = 144 * (((char *)v68 - v48) / 144);
    v65 = v48;
    if ( v32 >= 0x1000 )
    {
      v32 += 39LL;
      v48 = (char *)*((_QWORD *)v48 - 1);
      if ( (unsigned __int64)(v65 - v48 - 8) > 0x1F )
LABEL_84:
        sub_148AAF304(v64, v32);
    }
    v30 = v48;
    goto LABEL_80;
  }
}


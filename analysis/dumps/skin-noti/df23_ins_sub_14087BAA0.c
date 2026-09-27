// ins_sub_14087BAA0

__int64 __fastcall sub_14087BAA0(__int64 a1)
{
  unsigned int v1; // r14d
  unsigned int v2; // esi
  int v3; // eax
  __int64 v4; // r8
  __int64 v5; // rax
  char *v6; // rbx
  char *v7; // rdi
  __int64 v8; // rcx
  unsigned __int64 v9; // rdx
  __int64 v10; // r8
  __int64 v11; // rcx
  __int64 v12; // rcx
  unsigned __int64 v13; // rdx
  __int64 v14; // r8
  __int64 v15; // rcx
  __int64 v16; // rcx
  unsigned __int64 v17; // rdx
  __int64 v18; // r8
  __int64 v19; // rcx
  __int64 v20; // rcx
  unsigned __int64 v21; // rdx
  __int64 v22; // r8
  __int64 v23; // rcx
  char *v24; // rax
  char *v25; // rbx
  char *v26; // rdi
  __int64 v27; // rcx
  unsigned __int64 v28; // rdx
  __int64 v29; // r8
  __int64 v30; // rcx
  __int64 v31; // rcx
  unsigned __int64 v32; // rdx
  __int64 v33; // r8
  __int64 v34; // rcx
  __int64 v35; // rcx
  unsigned __int64 v36; // rdx
  __int64 v37; // r8
  __int64 v38; // rcx
  __int64 v39; // rcx
  __int64 v40; // r8
  char *v41; // rax
  __int64 v42; // rax
  __int64 v43; // rbx
  __int64 v44; // rdi
  __int64 v45; // rcx
  unsigned __int64 v46; // rdx
  __int64 v47; // rax
  __int128 v49; // [rsp+28h] [rbp-38h] BYREF
  __int64 v50; // [rsp+38h] [rbp-28h]
  __int128 v51; // [rsp+40h] [rbp-20h] BYREF
  __int64 v52; // [rsp+50h] [rbp-10h]

  v1 = 48;
  if ( !a1 )
    return v1;
  v51 = 0;
  v52 = 0;
  if ( (unsigned __int8)sub_14087C600(a1, 0, 0, &v51) && (_QWORD)v51 != *((_QWORD *)&v51 + 1) )
  {
    v2 = -1;
    v3 = sub_14500CD30(*(unsigned int *)v51);
    if ( v3 != 1 )
    {
      if ( v3 == 2 )
        v2 = *(_DWORD *)v51;
      goto LABEL_63;
    }
    LOBYTE(v4) = 1;
    v5 = sub_140283D60(qword_14E683B38, *(unsigned int *)v51, v4);
    v49 = 0;
    v50 = 0;
    if ( (unsigned __int8)sub_14087C600(v5, 0, 0, &v49) )
    {
      v2 = *(_DWORD *)v49;
      if ( *(_DWORD *)v49 != -1 )
      {
        v6 = (char *)v49;
        if ( !(_QWORD)v49 )
          goto LABEL_63;
        v7 = (char *)*((_QWORD *)&v49 + 1);
        if ( (_QWORD)v49 != *((_QWORD *)&v49 + 1) )
        {
          do
          {
            v8 = *((_QWORD *)v6 + 15);
            if ( v8 )
            {
              v9 = 4 * ((*((_QWORD *)v6 + 17) - v8) >> 2);
              if ( v9 >= 0x1000 )
              {
                v9 += 39LL;
                v10 = *(_QWORD *)(v8 - 8);
                v11 = v8 - v10;
                if ( (unsigned __int64)(v11 - 8) > 0x1F )
                  sub_148AAF304(v11, v9);
                v8 = v10;
              }
              sub_146E9F3A0(v8, v9);
              *((_QWORD *)v6 + 15) = 0;
              *((_QWORD *)v6 + 16) = 0;
              *((_QWORD *)v6 + 17) = 0;
            }
            v12 = *((_QWORD *)v6 + 11);
            if ( v12 )
            {
              v13 = 4 * ((*((_QWORD *)v6 + 13) - v12) >> 2);
              if ( v13 >= 0x1000 )
              {
                v13 += 39LL;
                v14 = *(_QWORD *)(v12 - 8);
                v15 = v12 - v14;
                if ( (unsigned __int64)(v15 - 8) > 0x1F )
                  sub_148AAF304(v15, v13);
                v12 = v14;
              }
              sub_146E9F3A0(v12, v13);
              *((_QWORD *)v6 + 11) = 0;
              *((_QWORD *)v6 + 12) = 0;
              *((_QWORD *)v6 + 13) = 0;
            }
            v16 = *((_QWORD *)v6 + 6);
            if ( v16 )
            {
              v17 = 8 * ((*((_QWORD *)v6 + 8) - v16) >> 3);
              if ( v17 >= 0x1000 )
              {
                v17 += 39LL;
                v18 = *(_QWORD *)(v16 - 8);
                v19 = v16 - v18;
                if ( (unsigned __int64)(v19 - 8) > 0x1F )
                  sub_148AAF304(v19, v17);
                v16 = v18;
              }
              sub_146E9F3A0(v16, v17);
              *((_QWORD *)v6 + 6) = 0;
              *((_QWORD *)v6 + 7) = 0;
              *((_QWORD *)v6 + 8) = 0;
            }
            v20 = *((_QWORD *)v6 + 3);
            if ( v20 )
            {
              v21 = 8 * ((*((_QWORD *)v6 + 5) - v20) >> 3);
              if ( v21 >= 0x1000 )
              {
                v21 += 39LL;
                v22 = *(_QWORD *)(v20 - 8);
                v23 = v20 - v22;
                if ( (unsigned __int64)(v23 - 8) > 0x1F )
                  goto LABEL_77;
                v20 = v22;
              }
              sub_146E9F3A0(v20, v21);
              *((_QWORD *)v6 + 3) = 0;
              *((_QWORD *)v6 + 4) = 0;
              *((_QWORD *)v6 + 5) = 0;
            }
            v6 += 144;
          }
          while ( v6 != v7 );
          v6 = (char *)v49;
        }
        v23 = v50 - (_QWORD)v6;
        v21 = 144 * ((v50 - (__int64)v6) / 144);
        v24 = v6;
        if ( v21 < 0x1000 || (v21 += 39LL, v6 = (char *)*((_QWORD *)v6 - 1), (unsigned __int64)(v24 - v6 - 8) <= 0x1F) )
        {
          sub_146E9F3A0(v6, v21);
          v49 = 0;
          v50 = 0;
LABEL_63:
          LOBYTE(v4) = 1;
          v42 = sub_14021BE90(qword_14E683B30, v2, v4);
          if ( v42 )
            v1 = *(_DWORD *)(v42 + 2120);
          goto LABEL_65;
        }
LABEL_77:
        sub_148AAF304(v23, v21);
      }
    }
    v25 = (char *)v49;
    if ( (_QWORD)v49 )
    {
      v26 = (char *)*((_QWORD *)&v49 + 1);
      if ( (_QWORD)v49 != *((_QWORD *)&v49 + 1) )
      {
        do
        {
          v27 = *((_QWORD *)v25 + 15);
          if ( v27 )
          {
            v28 = 4 * ((*((_QWORD *)v25 + 17) - v27) >> 2);
            if ( v28 >= 0x1000 )
            {
              v28 += 39LL;
              v29 = *(_QWORD *)(v27 - 8);
              v30 = v27 - v29;
              if ( (unsigned __int64)(v30 - 8) > 0x1F )
                sub_148AAF304(v30, v28);
              v27 = v29;
            }
            sub_146E9F3A0(v27, v28);
            *((_QWORD *)v25 + 15) = 0;
            *((_QWORD *)v25 + 16) = 0;
            *((_QWORD *)v25 + 17) = 0;
          }
          v31 = *((_QWORD *)v25 + 11);
          if ( v31 )
          {
            v32 = 4 * ((*((_QWORD *)v25 + 13) - v31) >> 2);
            if ( v32 >= 0x1000 )
            {
              v32 += 39LL;
              v33 = *(_QWORD *)(v31 - 8);
              v34 = v31 - v33;
              if ( (unsigned __int64)(v34 - 8) > 0x1F )
                sub_148AAF304(v34, v32);
              v31 = v33;
            }
            sub_146E9F3A0(v31, v32);
            *((_QWORD *)v25 + 11) = 0;
            *((_QWORD *)v25 + 12) = 0;
            *((_QWORD *)v25 + 13) = 0;
          }
          v35 = *((_QWORD *)v25 + 6);
          if ( v35 )
          {
            v36 = 8 * ((*((_QWORD *)v25 + 8) - v35) >> 3);
            if ( v36 >= 0x1000 )
            {
              v36 += 39LL;
              v37 = *(_QWORD *)(v35 - 8);
              v38 = v35 - v37;
              if ( (unsigned __int64)(v38 - 8) > 0x1F )
                sub_148AAF304(v38, v36);
              v35 = v37;
            }
            sub_146E9F3A0(v35, v36);
            *((_QWORD *)v25 + 6) = 0;
            *((_QWORD *)v25 + 7) = 0;
            *((_QWORD *)v25 + 8) = 0;
          }
          v39 = *((_QWORD *)v25 + 3);
          if ( v39 )
          {
            v21 = 8 * ((*((_QWORD *)v25 + 5) - v39) >> 3);
            if ( v21 >= 0x1000 )
            {
              v21 += 39LL;
              v40 = *(_QWORD *)(v39 - 8);
              v23 = v39 - v40;
              if ( (unsigned __int64)(v23 - 8) > 0x1F )
                goto LABEL_77;
              v39 = v40;
            }
            sub_146E9F3A0(v39, v21);
            *((_QWORD *)v25 + 3) = 0;
            *((_QWORD *)v25 + 4) = 0;
            *((_QWORD *)v25 + 5) = 0;
          }
          v25 += 144;
        }
        while ( v25 != v26 );
        v25 = (char *)v49;
      }
      v23 = v50 - (_QWORD)v25;
      v21 = 144 * ((v50 - (__int64)v25) / 144);
      v41 = v25;
      if ( v21 >= 0x1000 )
      {
        v21 += 39LL;
        v25 = (char *)*((_QWORD *)v25 - 1);
        if ( (unsigned __int64)(v41 - v25 - 8) > 0x1F )
          goto LABEL_77;
      }
      sub_146E9F3A0(v25, v21);
      v49 = 0;
      v50 = 0;
    }
  }
LABEL_65:
  v43 = v51;
  if ( (_QWORD)v51 )
  {
    v44 = *((_QWORD *)&v51 + 1);
    if ( (_QWORD)v51 != *((_QWORD *)&v51 + 1) )
    {
      do
      {
        sub_1401574A0(v43 + 120);
        sub_1401574A0(v43 + 88);
        sub_14017AC10(v43 + 48);
        sub_14017AC10(v43 + 24);
        v43 += 144;
      }
      while ( v43 != v44 );
      v43 = v51;
    }
    v45 = v52 - v43;
    v46 = 144 * ((v52 - v43) / 144);
    v47 = v43;
    if ( v46 >= 0x1000 )
    {
      v46 += 39LL;
      v43 = *(_QWORD *)(v43 - 8);
      if ( (unsigned __int64)(v47 - v43 - 8) > 0x1F )
        sub_148AAF304(v45, v46);
    }
    sub_146E9F3A0(v43, v46);
    v51 = 0;
    v52 = 0;
  }
  return v1;
}


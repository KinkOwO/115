// mgr_helper_f1410 = sub_1444F1410 0x1444F1410

__int64 __fastcall sub_1444F1410(_QWORD *a1)
{
  __int64 v2; // r13
  __int64 v3; // rdi
  __int64 v4; // rbx
  char *v5; // r14
  char *v6; // rbx
  unsigned __int64 i; // rdi
  char *v8; // rdx
  _DWORD *v9; // r15
  _DWORD *v10; // rdi
  char *v11; // rcx
  char *v12; // rdx
  __int64 v13; // rcx
  __int64 v14; // rdx
  __int64 result; // rax
  __int64 v16; // rcx
  int v17; // r8d
  __int64 v18; // rdx
  int v19; // r9d
  __int64 v20; // rdx
  __int64 v21; // r8
  __int128 v22; // kr00_16
  __int64 v23; // rbx
  __int64 v24; // rdi
  __int64 v25; // rbx
  unsigned __int64 v26; // rdx
  unsigned __int64 v27; // rdx
  char *v28; // rax
  __int64 v29; // rcx
  unsigned __int64 v30; // rdx
  __int128 v31; // [rsp+20h] [rbp-49h] BYREF
  __int64 v32; // [rsp+30h] [rbp-39h]
  __int128 v33; // [rsp+38h] [rbp-31h] BYREF
  __int64 v34; // [rsp+48h] [rbp-21h]
  __int128 v35; // [rsp+50h] [rbp-19h] BYREF
  __int64 v36; // [rsp+60h] [rbp-9h]
  __int128 v37; // [rsp+68h] [rbp-1h] BYREF
  __int64 v38; // [rsp+78h] [rbp+Fh]
  __int64 v39; // [rsp+88h] [rbp+1Fh]
  __int128 *v40; // [rsp+D0h] [rbp+67h] BYREF
  __int128 *v41; // [rsp+D8h] [rbp+6Fh]

  v39 = -2;
  sub_140BF77E0(a1[144], a1[145], (__int64)(a1[145] - a1[144]) >> 2, 0);
  sub_140BF77E0(a1[147], a1[148], (__int64)(a1[148] - a1[147]) >> 2, 0);
  sub_1444EBC10((__int64)a1, &v37, 1);
  v35 = 0;
  v2 = 0;
  v36 = 0;
  v3 = a1[141];
  v4 = a1[142] - v3;
  if ( v4 >> 2 )
  {
    sub_1401C4C50(&v35);
    v2 = v36;
  }
  v5 = (char *)v35;
  sub_148AA1E60(v35, v3, v4);
  v6 = &v5[v4];
  *((_QWORD *)&v35 + 1) = v6;
  for ( i = 0; i < (v6 - v5) >> 2; ++i )
    *(_DWORD *)&v5[4 * i] = sub_1473A1580(a1 + 166, *(unsigned int *)&v5[4 * i]);
  v8 = v6;
  v9 = (_DWORD *)*((_QWORD *)&v37 + 1);
  v10 = (_DWORD *)v37;
  if ( (((v6 - v5) ^ (*((_QWORD *)&v37 + 1) - (_QWORD)v37)) & 0xFFFFFFFFFFFFFFFCuLL) == 0
    && (_QWORD)v37 != *((_QWORD *)&v37 + 1) )
  {
    do
    {
      v11 = v5;
      v8 = v6;
      if ( v5 != v6 )
      {
        while ( 1 )
        {
          v12 = v11 + 4;
          if ( *(_DWORD *)v11 == *v10 )
            break;
          v11 += 4;
          if ( v12 == v6 )
            goto LABEL_12;
        }
        sub_148AA1E60(v11, v12, v6 - v12);
        v6 -= 4;
        *((_QWORD *)&v35 + 1) = v6;
LABEL_12:
        v8 = v6;
      }
      ++v10;
    }
    while ( v10 != v9 );
  }
  if ( v5 != v8
    || (v13 = a1[147], v14 = a1[144], (((a1[148] - v13) ^ (a1[145] - v14)) & 0xFFFFFFFFFFFFFFFCuLL) != 0)
    || (result = sub_148AA33F0(v13, v14, a1[148] - v13), (_DWORD)result) )
  {
    v31 = 0;
    v32 = 0;
    sub_140954BF0(&v31, 20, &v40);
    v17 = 0;
    if ( (__int64)(a1[148] - a1[147]) >> 2 )
    {
      v18 = 0;
      do
      {
        *(_DWORD *)(v31 + v18) = *(_DWORD *)(v18 + a1[147]);
        ++v17;
        v18 += 4;
      }
      while ( v17 < (unsigned __int64)((__int64)(a1[148] - a1[147]) >> 2) );
    }
    v19 = 0;
    if ( (__int64)(a1[142] - a1[141]) >> 2 )
    {
      v20 = 0;
      v21 = 10;
      do
      {
        if ( v21 >= 20 )
          break;
        *(_DWORD *)(v31 + v20 + 40) = *(_DWORD *)(v20 + a1[141]);
        ++v19;
        ++v21;
        v20 += 4;
      }
      while ( v19 < (unsigned __int64)((__int64)(a1[142] - a1[141]) >> 2) );
    }
    v41 = &v33;
    v33 = 0;
    v34 = 0;
    v22 = v31;
    if ( (_QWORD)v31 != *((_QWORD *)&v31 + 1) )
    {
      v23 = (__int64)(*((_QWORD *)&v31 + 1) - v31) >> 2;
      *(_QWORD *)&v33 = sub_140157580(&v33, v23);
      *((_QWORD *)&v33 + 1) = v33;
      v24 = 4 * v23;
      v34 = 4 * v23 + v33;
      v40 = &v33;
      v25 = v33;
      sub_148AA1E60(v33, v22, *((_QWORD *)&v22 + 1) - v22);
      *((_QWORD *)&v33 + 1) = v25 + v24;
      v40 = 0;
    }
    result = sub_1444F1090((__int64)a1, 1, &v33);
    v16 = v31;
    if ( (_QWORD)v31 )
    {
      v26 = 4 * ((v32 - (__int64)v31) >> 2);
      if ( v26 >= 0x1000 )
      {
        v26 += 39LL;
        v16 = *(_QWORD *)(v31 - 8);
        if ( (unsigned __int64)(v31 - v16 - 8) > 0x1F )
          sub_148AAF304(v16, v26);
      }
      result = sub_146E9F3A0(v16, v26);
      v31 = 0;
      v32 = 0;
    }
  }
  if ( v5 )
  {
    v27 = 4 * ((v2 - (__int64)v5) >> 2);
    v28 = v5;
    if ( v27 >= 0x1000 )
    {
      v27 += 39LL;
      v5 = (char *)*((_QWORD *)v5 - 1);
      if ( (unsigned __int64)(v28 - v5 - 8) > 0x1F )
        sub_148AAF304(v16, v27);
    }
    result = sub_146E9F3A0(v5, v27);
    v35 = 0;
    v36 = 0;
  }
  v29 = v37;
  if ( (_QWORD)v37 )
  {
    v30 = (v38 - v37) & 0xFFFFFFFFFFFFFFFCuLL;
    if ( v30 >= 0x1000 )
    {
      v30 += 39LL;
      v29 = *(_QWORD *)(v37 - 8);
      if ( (unsigned __int64)(v37 - v29 - 8) > 0x1F )
        sub_148AAF304(v29, v30);
    }
    result = sub_146E9F3A0(v29, v30);
    v37 = 0;
    v38 = 0;
  }
  return result;
}


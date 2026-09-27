__int64 __fastcall sub_1441ECDB0(__int64 a1, int a2)
{
  __int64 v2; // rsi
  __int64 v4; // r15
  __int64 v5; // rcx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  __int64 result; // rax
  unsigned __int64 v9; // rdx
  __int64 v10; // r8
  __int64 v11; // r9
  unsigned int *v12; // rdi
  unsigned int *v13; // rbp
  char *v14; // rcx
  unsigned __int64 v15; // rdx
  unsigned __int64 v16; // rdx
  unsigned __int64 v17; // kr00_8
  __int64 v18; // rax
  unsigned int *v19; // r14
  unsigned int *i; // rsi
  unsigned int *v21; // rsi
  int v22; // r9d
  __int64 v24; // r8
  int v25; // esi
  unsigned int *v26; // r14
  __int64 v27; // rax
  __int64 v28; // rcx
  unsigned int v29; // eax
  __int64 v30; // rax
  __int64 v31; // rax
  unsigned int *v32; // r14
  unsigned int *j; // rsi
  unsigned __int64 v34; // rdx
  char *v35; // r8
  unsigned int v36; // r9d
  char *v37; // rdx
  unsigned __int64 v38; // r15
  unsigned int *v39; // rax
  __int128 v40; // [rsp+28h] [rbp-60h] BYREF
  __int64 v41; // [rsp+38h] [rbp-50h]
  _OWORD v42[2]; // [rsp+40h] [rbp-48h] BYREF

  v2 = a2;
  v40 = 0;
  v4 = 0;
  v41 = 0;
  v5 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v6 = sub_146E8BA20(1472);
    if ( v6 )
      v7 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v6);
    else
      v7 = 0;
    qword_14E638F28 = (__int64)v7;
    (**v7)(v7);
    v5 = qword_14E638F28;
  }
  result = sub_1444EBC10(v5, v42, (unsigned int)v2);
  if ( &v40 == (__int128 *)result )
  {
    v13 = (unsigned int *)*((_QWORD *)&v40 + 1);
    v12 = (unsigned int *)v40;
  }
  else
  {
    v12 = *(unsigned int **)result;
    *(_QWORD *)&v40 = *(_QWORD *)result;
    v13 = *(unsigned int **)(result + 8);
    *((_QWORD *)&v40 + 1) = v13;
    v4 = *(_QWORD *)(result + 16);
    v41 = v4;
    *(_QWORD *)result = 0;
    *(_QWORD *)(result + 8) = 0;
    *(_QWORD *)(result + 16) = 0;
  }
  v14 = *(char **)&v42[0];
  if ( *(_QWORD *)&v42[0] )
  {
    v15 = 4 * ((__int64)(*(_QWORD *)&v42[1] - *(_QWORD *)&v42[0]) >> 2);
    if ( v15 >= 0x1000 )
    {
      v15 += 39LL;
      v14 = *(char **)(*(_QWORD *)&v42[0] - 8LL);
      if ( (unsigned __int64)(*(_QWORD *)&v42[0] - (_QWORD)v14 - 8LL) > 0x1F )
        sub_148AAF304(v14, v15);
    }
    result = sub_146E9F3A0(v14, v15);
    memset(v42, 0, 24);
  }
  v17 = v9;
  v16 = 0x140000000uLL;
  switch ( v2 )
  {
    case 0LL:
      if ( v12 != v13 )
      {
        result = sub_1441E0820(a1, 0, *v12);
        break;
      }
      goto LABEL_70;
    case 1LL:
      v14 = (char *)*(unsigned int *)(a1 + 13416);
      if ( (_DWORD)v14 )
      {
        if ( (_DWORD)v14 == 1 )
        {
          v18 = sub_140764510(v14);
          result = sub_1444EBC00(v18);
          v19 = *(unsigned int **)(result + 8);
          for ( i = *(unsigned int **)result; i != v19; ++i )
            result = sub_1441E0820(a1, 1, *i);
        }
        break;
      }
      v21 = v12;
      if ( v12 != v13 )
      {
        do
        {
          result = sub_1444EBAB0(*v21);
          if ( result )
          {
            v22 = *(_DWORD *)(a1 + 13416);
            if ( *(_DWORD *)(result + 12) == 3 ? v22 == 1 : v22 == 0 )
            {
              v24 = *v21;
              if ( (_DWORD)v24 )
                result = sub_1441E0820(a1, 1, v24);
            }
          }
          ++v21;
        }
        while ( v21 != v13 );
        break;
      }
      goto LABEL_70;
    case 2LL:
      if ( v12 != v13 )
      {
        result = sub_1441E0820(a1, 2, *v12);
        break;
      }
      goto LABEL_70;
    case 3LL:
      v25 = 0;
      v26 = v12;
      if ( v12 != v13 )
      {
        do
        {
          if ( v25 >= 4 )
            break;
          result = sub_1441E0820(a1, 3, *v26);
          ++v25;
          ++v26;
        }
        while ( v26 != v13 );
        break;
      }
      goto LABEL_70;
    case 4LL:
      if ( v12 != v13 )
      {
        result = sub_1441E0820(a1, 4, *v12);
        break;
      }
      goto LABEL_70;
    case 5LL:
      if ( !qword_14EF2CAA0 || (v27 = qword_14EF2CAA8, !*(_DWORD *)(qword_14EF2CAA0 + 8)) )
        v27 = 0;
      v28 = v27 - 48;
      if ( !v27 )
        v28 = 0;
      v29 = sub_145BD8B90(v28, 0x140000000uLL, v10, v11);
      result = sub_1441E0820(a1, 5, v29);
      break;
    case 6LL:
      if ( v12 == v13 )
        goto LABEL_70;
      result = sub_1441E0820(a1, 6, *v12);
      break;
    case 7LL:
      if ( v12 == v13 )
        goto LABEL_70;
      result = sub_1441E0820(a1, 7, *v12);
      break;
    case 8LL:
      if ( v12 == v13 )
        goto LABEL_70;
      result = sub_1441E0820(a1, 8, *v12);
      break;
    case 9LL:
      v30 = sub_140764510(v14);
      v31 = sub_140157CF0(v30);
      result = sub_1401550D0(v42, v31);
      *(_DWORD *)(a1 + 4072) = 0;
      v32 = (unsigned int *)*((_QWORD *)&v42[0] + 1);
      for ( j = *(unsigned int **)&v42[0]; j != v32; ++j )
      {
        sub_1441E0630(a1, *j, *(unsigned int *)(a1 + 4072));
        result = (*(_DWORD *)(a1 + 4072) + 1) & 0x80000003;
        if ( *(_DWORD *)(a1 + 4072) + 1 < 0 )
          result = ((unsigned __int8)(((*(_BYTE *)(a1 + 4072) + 1) & 3) - 1) | 0xFFFFFFFC) + 1;
        *(_DWORD *)(a1 + 4072) = result;
      }
      v14 = *(char **)&v42[0];
      if ( *(_QWORD *)&v42[0] )
      {
        v34 = 4 * ((__int64)(*(_QWORD *)&v42[1] - *(_QWORD *)&v42[0]) >> 2);
        if ( v34 >= 0x1000 )
        {
          v34 += 39LL;
          v14 = *(char **)(*(_QWORD *)&v42[0] - 8LL);
          if ( (unsigned __int64)(*(_QWORD *)&v42[0] - (_QWORD)v14 - 8LL) > 0x1F )
            sub_148AAF304(v14, v34);
        }
        result = sub_146E9F3A0(v14, v34);
        memset(v42, 0, 24);
      }
      break;
    default:
      v16 = v17;
      break;
  }
  if ( v12 != v13 )
  {
    v14 = *(char **)(a1 + 3328);
    result = (__int64)v14;
    v35 = *(char **)(a1 + 3336);
    if ( v14 != v35 )
    {
      v36 = *v12;
      while ( *(_DWORD *)result != v36 )
      {
        result += 4;
        if ( (char *)result == v35 )
          goto LABEL_70;
      }
      while ( 1 )
      {
        v37 = v14 + 4;
        if ( *(_DWORD *)v14 == v36 )
          break;
        v14 += 4;
        if ( v37 == v35 )
          goto LABEL_69;
      }
      sub_148AA1E60(v14, v37, v35 - v37);
      *(_QWORD *)(a1 + 3336) -= 4LL;
LABEL_69:
      result = sub_1441EC510(a1);
    }
  }
LABEL_70:
  if ( v12 )
  {
    v38 = (v4 - (_QWORD)v12) & 0xFFFFFFFFFFFFFFFCuLL;
    v39 = v12;
    if ( v38 >= 0x1000 )
    {
      v38 += 39LL;
      v12 = (unsigned int *)*((_QWORD *)v12 - 1);
      if ( (unsigned __int64)((char *)v39 - (char *)v12 - 8) > 0x1F )
        sub_148AAF304(v14, v16);
    }
    result = sub_146E9F3A0(v12, v38);
    v40 = 0;
    v41 = 0;
  }
  return result;
}

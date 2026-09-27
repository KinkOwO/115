__int64 __fastcall sub_1444EFA00(__int64 *a1, __int64 a2, int a3)
{
  __int64 result; // rax
  __int64 v4; // rsi
  __int64 *v5; // r12
  _DWORD *v6; // r13
  _DWORD *v7; // r15
  int v8; // edi
  __int64 *v9; // r8
  __int64 *v10; // rax
  __int64 *v11; // r8
  __int64 *v12; // rax
  char v13; // al
  char v14; // r8
  unsigned int v15; // ebx
  __int64 v16; // r14
  __int64 *v17; // r12
  __int64 *v18; // rdx
  int *v19; // rax
  __int64 *v20; // rcx
  _QWORD *v21; // rcx
  __int64 v22; // r15
  __int64 v23; // rdi
  __int64 v24; // rbx
  unsigned __int64 v25; // rdx
  __int64 v26; // rax
  __int64 v27; // rbx
  __int64 v28; // rax
  int i; // ebx
  __int64 v30; // r8
  __int64 *v31; // rsi
  __int64 v32; // rcx
  int v33; // edx
  int v34; // edi
  int *v35; // rdx
  __int64 v36; // rbx
  int j; // ebx
  unsigned __int64 v38; // r13
  __int64 v39; // rax
  int v40; // [rsp+20h] [rbp-60h] BYREF
  int v41; // [rsp+24h] [rbp-5Ch] BYREF
  int v42; // [rsp+28h] [rbp-58h] BYREF
  int v43; // [rsp+2Ch] [rbp-54h]
  __int128 v44; // [rsp+30h] [rbp-50h] BYREF
  _DWORD *v45; // [rsp+40h] [rbp-40h]
  int v46[4]; // [rsp+48h] [rbp-38h] BYREF
  int v47; // [rsp+58h] [rbp-28h] BYREF
  __int128 v48; // [rsp+60h] [rbp-20h] BYREF
  __int64 v49; // [rsp+70h] [rbp-10h]
  __int64 v50; // [rsp+78h] [rbp-8h]
  unsigned __int16 v51; // [rsp+C8h] [rbp+48h] BYREF
  unsigned int v52; // [rsp+D8h] [rbp+58h] BYREF

  v50 = -2;
  v4 = a3;
  v5 = a1;
  v43 = 0;
  v44 = 0u;
  v6 = 0;
  v45 = 0;
  v7 = 0;
  switch ( (int)a2 )
  {
    case 0:
    case 4:
    case 7:
      result = sub_146EA0BA0(&v52);
      goto LABEL_74;
    case 1:
      result = sub_146EA1920(&v51);
      v8 = 0;
      if ( !v51 )
        goto LABEL_74;
      break;
    case 2:
      sub_146EA0BA0(&v52);
      v26 = sub_143C61150();
      result = sub_1447EF3B0(v26, v52, (unsigned int)v4);
      goto LABEL_74;
    case 3:
      v27 = 4;
      do
      {
        result = sub_146EA0BA0(&v52);
        --v27;
      }
      while ( v27 );
      goto LABEL_74;
    case 6:
      sub_146EA0BA0(&v52);
      v28 = sub_143C61150();
      result = sub_1447EF380(v28, v52, (unsigned int)v4);
      goto LABEL_74;
    case 10:
      v42 = 0;
      sub_146EA1920(&v42);
      for ( i = 0; i < v42; ++i )
      {
        v40 = 0;
        sub_146EA0BA0(&v40);
      }
      v41 = 0;
      sub_146EA1920(&v41);
      v31 = &v5[3 * v4];
      v32 = v31[150];
      v31[151] = v32;
      v33 = v41;
      if ( v41 > (unsigned __int64)((v31[152] - v32) >> 2) )
      {
        if ( (unsigned __int64)v41 > 0x3FFFFFFFFFFFFFFFLL )
          sub_14014E270(v32, v41);
        sub_1402B0370(v31 + 150, v41, v30);
        v33 = v41;
      }
      v34 = 0;
      if ( v33 > 0 )
      {
        do
        {
          v40 = 0;
          sub_146EA0BA0(&v40);
          if ( v40 > 0 )
          {
            v35 = (int *)v31[151];
            if ( v35 == (int *)v31[152] )
            {
              sub_140154010(v31 + 150, v35, &v40);
            }
            else
            {
              *v35 = v40;
              v31[151] += 4;
            }
          }
          ++v34;
        }
        while ( v34 < v41 );
      }
      sub_146EA0BA0(&v52);
      v36 = 4;
      do
      {
        sub_146EA0BA0(&v52);
        --v36;
      }
      while ( v36 );
      v46[0] = 0;
      result = sub_146EA1920(v46);
      for ( j = 0; j < v46[0]; ++j )
        result = sub_146EA0BA0(&v52);
      goto LABEL_74;
    default:
LABEL_74:
      v16 = v44;
      goto LABEL_75;
  }
  do
  {
    sub_146EA0BA0(&v52);
    a2 = v52;
    if ( v52 && (unsigned int)v4 <= 3 )
    {
      v9 = (__int64 *)v5[20 * v4 + 43];
      v10 = (__int64 *)v9[1];
      a1 = v9;
      while ( !*((_BYTE *)v10 + 25) )
      {
        if ( *((_DWORD *)v10 + 7) >= (signed int)v52 )
        {
          a1 = v10;
          v10 = (__int64 *)*v10;
        }
        else
        {
          v10 = (__int64 *)v10[2];
        }
      }
      if ( !*((_BYTE *)a1 + 25) && (signed int)v52 >= *((_DWORD *)a1 + 7) && a1 != v9 )
        goto LABEL_85;
      v11 = (__int64 *)v5[19];
      v12 = (__int64 *)v11[1];
      a1 = v11;
      while ( !*((_BYTE *)v12 + 25) )
      {
        if ( *((_DWORD *)v12 + 7) >= (signed int)v52 )
        {
          a1 = v12;
          v12 = (__int64 *)*v12;
        }
        else
        {
          v12 = (__int64 *)v12[2];
        }
      }
      if ( !*((_BYTE *)a1 + 25) && (signed int)v52 >= *((_DWORD *)a1 + 7) && a1 != v11 )
      {
LABEL_85:
        a1 += 4;
        if ( a1 )
        {
          v13 = *((_BYTE *)a1 + 12);
          if ( v13 && !*((_DWORD *)a1 + 2) )
            goto LABEL_34;
          v14 = *((_BYTE *)a1 + 20);
          if ( v14 )
          {
            if ( !*((_DWORD *)a1 + 4) )
              goto LABEL_34;
          }
          v15 = 0;
          if ( v13 )
            v15 = *((_DWORD *)a1 + 2);
          if ( v14 && *((_DWORD *)a1 + 2) < *((_DWORD *)a1 + 4) )
            v15 = *((_DWORD *)a1 + 4);
          if ( v15 && (unsigned int)sub_145A11A50() <= v15 )
          {
            a2 = v52;
LABEL_34:
            v46[0] = a2;
            if ( v7 == v6 )
            {
              sub_140154010(&v44, v7, v46);
              v6 = v45;
              v7 = (_DWORD *)*((_QWORD *)&v44 + 1);
            }
            else
            {
              *v7++ = a2;
              *((_QWORD *)&v44 + 1) = v7;
            }
          }
        }
      }
    }
    ++v8;
    result = v51;
  }
  while ( v8 < v51 );
  v16 = v44;
  if ( (_DWORD *)v44 != v7 && (unsigned int)v4 <= 3 )
  {
    v17 = &v5[2 * v4];
    v18 = (__int64 *)v17[121];
    v19 = (int *)v18[1];
    v20 = v18;
    while ( !*((_BYTE *)v19 + 25) )
    {
      if ( v19[8] >= 1 )
      {
        v20 = (__int64 *)v19;
        v19 = *(int **)v19;
      }
      else
      {
        v19 = (int *)*((_QWORD *)v19 + 2);
      }
    }
    if ( *((_BYTE *)v20 + 25) || *((int *)v20 + 8) > 1 || v20 == v18 )
    {
      v47 = 1;
      v48 = 0;
      v49 = 0;
      v22 = (__int64)v7 - v44;
      *(_QWORD *)&v48 = sub_140157580(&v48, v22 >> 2);
      *((_QWORD *)&v48 + 1) = v48;
      v23 = 4 * (v22 >> 2);
      v49 = v23 + v48;
      *(_QWORD *)v46 = &v48;
      v24 = v48;
      sub_148AA1E60(v48, v16, v22);
      *((_QWORD *)&v48 + 1) = v23 + v24;
      *(_QWORD *)v46 = 0;
      result = sub_1401844D0(v17 + 121, v46, &v47);
      v43 = 0;
      a1 = (__int64 *)v48;
      if ( (_QWORD)v48 )
      {
        v25 = 4 * ((v49 - (__int64)v48) >> 2);
        if ( v25 >= 0x1000 )
        {
          v25 += 39LL;
          a1 = *(__int64 **)(v48 - 8);
          if ( (unsigned __int64)(v48 - (_QWORD)a1 - 8) > 0x1F )
            sub_148AAF304(a1, v25);
        }
        result = sub_146E9F3A0(a1, v25);
        v48 = 0;
        v49 = 0;
      }
    }
    else
    {
      v21 = v20 + 5;
      v21[1] = *v21;
      result = sub_143D855D0(v21, v16, v7);
    }
  }
LABEL_75:
  if ( v16 )
  {
    v38 = ((unsigned __int64)v6 - v16) & 0xFFFFFFFFFFFFFFFCuLL;
    v39 = v16;
    if ( v38 >= 0x1000 )
    {
      v38 += 39LL;
      v16 = *(_QWORD *)(v16 - 8);
      if ( (unsigned __int64)(v39 - v16 - 8) > 0x1F )
        sub_148AAF304(a1, a2);
    }
    result = sub_146E9F3A0(v16, v38);
    v44 = 0;
    v45 = 0;
  }
  return result;
}

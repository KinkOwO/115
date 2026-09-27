// holder_sub_1447EE8F0_0x1447ee8f0

__int64 __fastcall sub_1447EE8F0(_QWORD *a1, __int64 a2, unsigned __int64 a3, __int64 a4)
{
  char v5; // r14
  int v6; // esi
  __int64 *v7; // rbx
  __int64 *v8; // r15
  __int64 v9; // rcx
  unsigned int v10; // edx
  __int64 v11; // r11
  __int64 v12; // r10
  __int64 *v13; // rdi
  __int64 *v14; // rcx
  __int64 v15; // r8
  __int64 *v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rcx
  _QWORD *v19; // rbx
  _QWORD *v20; // rdi
  __int64 v21; // rsi
  __int64 v22; // rbx
  void (__fastcall ***v23)(_QWORD, __int64, unsigned __int64, __int64, __int64); // rcx
  __int64 i; // rdi
  void (__fastcall ***v25)(_QWORD, __int64, unsigned __int64, __int64); // rcx
  __int64 v26; // rdi
  _QWORD *v27; // rax
  _QWORD *v28; // rdi
  __int64 v29; // rcx
  __int64 v30; // r8
  _QWORD *v31; // rax
  __int64 *j; // rbx
  __int64 *v33; // rcx
  __int64 v34; // rax
  __int64 **v35; // rax
  __int64 k; // rax
  __int64 *m; // rcx
  _QWORD *v38; // rbx
  __int64 result; // rax
  _QWORD *v40; // rbx
  __int64 v41; // rdi
  int v42; // eax
  __int64 v43; // rax
  __int64 v44; // rax
  void (__fastcall ***v45)(_QWORD); // rcx
  _QWORD *v46; // rcx
  __int64 v47; // [rsp+60h] [rbp+8h] BYREF

  v5 = 1;
  v6 = 0;
  v7 = (__int64 *)a1[9];
  v8 = (__int64 *)a1[10];
  if ( v7 == v8 )
    goto LABEL_23;
  do
  {
    v9 = *v7;
    if ( *(_BYTE *)(*v7 + 64) )
    {
      ++v6;
      v10 = *(_DWORD *)(v9 + 340);
      a4 = HIBYTE(v10);
      if ( v10 != -1 )
      {
        *(_DWORD *)(v9 + 340) = -1;
        a3 = a1[7]
           & (0x100000001B3LL
            * ((unsigned int)a4
             ^ (0x100000001B3LL
              * (BYTE2(v10)
               ^ (0x100000001B3LL * (BYTE1(v10) ^ (0x100000001B3LL * ((unsigned __int8)v10 ^ 0xCBF29CE484222325uLL))))))));
        v11 = a1[4];
        v12 = 2 * a3;
        v13 = *(__int64 **)(v11 + 16 * a3 + 8);
        v14 = v13;
        a4 = a1[2];
        if ( v13 != (__int64 *)a4 )
        {
          if ( v10 == *((_DWORD *)v13 + 4) )
          {
LABEL_8:
            if ( v14 )
            {
              v15 = 2 * a3;
              v16 = *(__int64 **)(v11 + 8 * v15);
              if ( v13 == v14 )
              {
                if ( v16 == v14 )
                  *(_QWORD *)(v11 + 8 * v15) = a4;
                else
                  a4 = v14[1];
                *(_QWORD *)(v11 + 8 * v12 + 8) = a4;
              }
              else if ( v16 == v14 )
              {
                *(_QWORD *)(v11 + 8 * v15) = *v14;
              }
              v17 = *v14;
              --a1[3];
              *(_QWORD *)v14[1] = v17;
              *(_QWORD *)(v17 + 8) = v14[1];
              sub_146E9F3A0(v14, 24);
            }
          }
          else
          {
            while ( v14 != *(__int64 **)(v11 + 16 * a3) )
            {
              v14 = (__int64 *)v14[1];
              if ( v10 == *((_DWORD *)v14 + 4) )
                goto LABEL_8;
            }
          }
        }
      }
    }
    else
    {
      sub_1447EE730(v9);
      v18 = *v7;
      LODWORD(v47) = *(_DWORD *)(*v7 + 340);
      if ( (_DWORD)v47 != -1 && !*(_DWORD *)(v18 + 344) )
      {
        *(_DWORD *)(v18 + 340) = -1;
        sub_142A29E30(a1 + 1, &v47);
      }
      v5 = 0;
    }
    ++v7;
  }
  while ( v7 != v8 );
  if ( v5 )
  {
LABEL_23:
    v19 = (_QWORD *)a1[9];
    v20 = (_QWORD *)a1[10];
    if ( v19 != v20 )
    {
      do
      {
        if ( *v19 )
          (**(void (__fastcall ***)(_QWORD, __int64))*v19)(*v19, 1);
        *v19++ = 0;
      }
      while ( v19 != v20 );
      v19 = (_QWORD *)a1[9];
    }
    a1[10] = v19;
  }
  if ( v6 > 200 )
  {
    v21 = a1[9];
    v22 = a1[10];
    while ( v21 != v22 )
    {
      v23 = *(void (__fastcall ****)(_QWORD, __int64, unsigned __int64, __int64, __int64))v21;
      if ( *(_BYTE *)(*(_QWORD *)v21 + 64LL) )
      {
        if ( v23 )
          (**v23)(v23, 1, a3, a4, -2);
        if ( v21 != v22 )
        {
          for ( i = v21 + 8; i != v22; i += 8 )
          {
            v25 = *(void (__fastcall ****)(_QWORD, __int64, unsigned __int64, __int64))i;
            if ( *(_BYTE *)(*(_QWORD *)i + 64LL) )
            {
              if ( v25 )
                (**v25)(v25, 1, a3, a4);
            }
            else
            {
              *(_QWORD *)v21 = v25;
              v21 += 8;
            }
          }
          if ( v21 != v22 )
          {
            v26 = a1[10] - v22;
            sub_148AA1E60(v21, v22, v26);
            a1[10] = v26 + v21;
          }
        }
        break;
      }
      v21 += 8;
    }
  }
  if ( !a1[26] )
    goto LABEL_66;
  v27 = (_QWORD *)a1[25];
  v28 = (_QWORD *)*v27;
  if ( (_QWORD *)*v27 != v27 )
  {
    do
    {
      v29 = v28[5];
      if ( v29 && *(_BYTE *)(v29 + 64) )
      {
        (**(void (__fastcall ***)(__int64, __int64, unsigned __int64, __int64))v29)(v29, 1, a3, a4);
        v31 = v28;
        j = (__int64 *)v28[2];
        if ( *((_BYTE *)j + 25) )
        {
          for ( j = (__int64 *)v28[1]; !*((_BYTE *)j + 25); j = (__int64 *)j[1] )
          {
            if ( v31 != (_QWORD *)j[2] )
              break;
            v31 = j;
          }
        }
        else
        {
          v33 = (__int64 *)*j;
          if ( !*(_BYTE *)(*j + 25) )
          {
            do
            {
              j = v33;
              v33 = (__int64 *)*v33;
            }
            while ( !*((_BYTE *)v33 + 25) );
          }
        }
        v34 = sub_1401C5030(a1 + 25, v28, v30);
        sub_146E9F3A0(v34, 48);
        v28 = j;
      }
      else
      {
        v35 = (__int64 **)v28[2];
        if ( *((_BYTE *)v35 + 25) )
        {
          for ( k = v28[1]; !*(_BYTE *)(k + 25); k = *(_QWORD *)(k + 8) )
          {
            if ( v28 != *(_QWORD **)(k + 16) )
              break;
            v28 = (_QWORD *)k;
          }
          v28 = (_QWORD *)k;
        }
        else
        {
          v28 = (_QWORD *)v28[2];
          for ( m = *v35; !*((_BYTE *)m + 25); m = (__int64 *)*m )
            v28 = m;
        }
      }
    }
    while ( v28 != (_QWORD *)a1[25] );
    if ( !a1[26] )
    {
LABEL_66:
      v38 = (_QWORD *)a1[25];
      sub_14014EBD0(a1 + 25, a1 + 25, v38[1]);
      v38[1] = v38;
      *v38 = v38;
      v38[2] = v38;
      a1[26] = 0;
    }
  }
  result = a1[25];
  v40 = *(_QWORD **)result;
  if ( *(_QWORD *)result != result )
  {
    do
    {
      v41 = v40[5];
      if ( v41 && !*(_BYTE *)(v41 + 64) )
      {
        v42 = *(_DWORD *)(v41 + 108);
        if ( v42 <= -1 )
        {
          v43 = qword_14E63AE60;
          if ( !qword_14E63AE60 )
          {
            v44 = sub_146E8BA20(336);
            v47 = v44;
            if ( v44 )
              v45 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v44);
            else
              v45 = 0;
            qword_14E63AE60 = (__int64)v45;
            (**v45)(v45);
            v43 = qword_14E63AE60;
          }
          v42 = *(_DWORD *)(v43 + 232);
        }
        *(_DWORD *)(v41 + 104) = v42;
        *(_BYTE *)(v41 + 112) = 1;
      }
      result = v40[2];
      if ( *(_BYTE *)(result + 25) )
      {
        for ( result = v40[1]; !*(_BYTE *)(result + 25); result = *(_QWORD *)(result + 8) )
        {
          if ( v40 != *(_QWORD **)(result + 16) )
            break;
          v40 = (_QWORD *)result;
        }
        v40 = (_QWORD *)result;
      }
      else
      {
        v40 = (_QWORD *)v40[2];
        v46 = *(_QWORD **)result;
        if ( !*(_BYTE *)(*(_QWORD *)result + 25LL) )
        {
          do
          {
            v40 = v46;
            result = *v46;
            v46 = (_QWORD *)result;
          }
          while ( !*(_BYTE *)(result + 25) );
        }
      }
    }
    while ( v40 != (_QWORD *)a1[25] );
  }
  return result;
}


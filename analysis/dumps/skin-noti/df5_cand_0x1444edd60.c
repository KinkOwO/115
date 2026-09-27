__int64 sub_1444EDD60()
{
  __int64 result; // rax
  int j; // esi
  __int64 v2; // r10
  __int64 v3; // rdi
  __int64 v4; // rbx
  __int64 *v5; // rcx
  __int64 *v6; // rdx
  __int64 *v7; // rax
  __int64 *m; // r8
  __int64 v9; // rax
  __int64 k; // rcx
  __int64 v11; // rax
  int i; // ebx
  __int64 v13; // rcx
  unsigned __int16 v14; // [rsp+20h] [rbp-20h] BYREF
  __m128i si128; // [rsp+22h] [rbp-1Eh]
  int v16; // [rsp+32h] [rbp-Eh]
  char v17; // [rsp+36h] [rbp-Ah]
  int v18; // [rsp+37h] [rbp-9h]
  int v19; // [rsp+70h] [rbp+30h] BYREF
  unsigned int v20; // [rsp+78h] [rbp+38h] BYREF

  sub_146EA0BA0(&v20);
  v19 = 0;
  result = sub_146EA0BA0(&v19);
  if ( v20 < 2 )
  {
    for ( i = 0; i < v19; ++i )
    {
      v14 = 0;
      si128 = _mm_load_si128(xmmword_1491B4820);
      v16 = -1;
      v17 = 5;
      v18 = 0;
      sub_146EA0BE0(&v14, 27);
      v13 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        qword_14E638F28 = sub_1444E7F30();
        (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        v13 = qword_14E638F28;
      }
      result = sub_1444E9010(v13, &v14);
    }
  }
  else if ( v20 == 2 )
  {
    for ( j = 0; j < v19; ++j )
    {
      v14 = 0;
      si128 = _mm_load_si128(xmmword_1491B4820);
      v16 = -1;
      v17 = 5;
      v18 = 0;
      sub_146EA0BE0(&v14, 27);
      v2 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        qword_14E638F28 = sub_1444E7F30();
        (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        v2 = qword_14E638F28;
      }
      v3 = v2 + 1440;
      v4 = *(_QWORD *)(v2 + 1440);
      result = *(_QWORD *)(v4 + 8);
      while ( !*(_BYTE *)(result + 25) )
      {
        if ( *(_WORD *)(result + 28) >= v14 )
        {
          v4 = result;
          result = *(_QWORD *)result;
        }
        else
        {
          result = *(_QWORD *)(result + 16);
        }
      }
      if ( !*(_BYTE *)(v4 + 25) && v14 >= *(_WORD *)(v4 + 28) && *(_QWORD *)(v2 + 1440) != v4 )
      {
        v5 = *(__int64 **)(v2 + 1456);
        v6 = v5;
        v7 = (__int64 *)v5[1];
        if ( !*((_BYTE *)v7 + 25) )
        {
          do
          {
            if ( *((_DWORD *)v7 + 8) >= *(_DWORD *)(v4 + 32) )
            {
              v5 = v7;
              v7 = (__int64 *)*v7;
            }
            else
            {
              v7 = (__int64 *)v7[2];
            }
          }
          while ( !*((_BYTE *)v7 + 25) );
          v6 = *(__int64 **)(v2 + 1456);
        }
        if ( !*((_BYTE *)v5 + 25)
          && *(_DWORD *)(v4 + 32) >= *((_DWORD *)v5 + 8)
          && v6 != v5
          && (*((_DWORD *)v5 + 29) & 2) == 0 )
        {
          sub_146E9FBD0(v5 + 14, 500, 0);
        }
        m = *(__int64 **)(v4 + 16);
        v9 = v4;
        if ( *((_BYTE *)m + 25) )
        {
          for ( k = *(_QWORD *)(v4 + 8); !*(_BYTE *)(k + 25); k = *(_QWORD *)(k + 8) )
          {
            if ( v9 != *(_QWORD *)(k + 16) )
              break;
            v9 = k;
          }
        }
        else
        {
          for ( m = (__int64 *)*m; !*((_BYTE *)m + 25); m = (__int64 *)*m )
            ;
        }
        v11 = sub_1401C5030(v3, v4, m);
        result = sub_146E9F3A0(v11, 40);
      }
    }
  }
  return result;
}

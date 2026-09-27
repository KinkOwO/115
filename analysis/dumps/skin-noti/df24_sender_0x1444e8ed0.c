// sender_0x1444e8ed0

__int64 __fastcall sub_1444E8ED0(__int64 *a1, int a2)
{
  __int64 *v3; // rsi
  __int64 v4; // r8
  __int64 result; // rax
  __int64 v6; // rdx
  char v7; // r9
  __int64 v8; // rbx
  __int64 v9; // rax
  __int128 v10; // [rsp+30h] [rbp-38h] BYREF
  __int128 v11; // [rsp+40h] [rbp-28h]

  v3 = a1 + 39;
  v4 = a1[39];
  result = *(_QWORD *)(v4 + 8);
  v6 = v4;
  v7 = *(_BYTE *)(result + 25);
  if ( !v7 )
  {
    a1 = *(__int64 **)(v4 + 8);
    do
    {
      if ( *((_DWORD *)a1 + 7) >= a2 )
      {
        v6 = (__int64)a1;
        a1 = (__int64 *)*a1;
      }
      else
      {
        a1 = (__int64 *)a1[2];
      }
    }
    while ( !*((_BYTE *)a1 + 25) );
  }
  if ( *(_BYTE *)(v6 + 25) || a2 < *(_DWORD *)(v6 + 28) || v6 == v4 )
  {
    *(_QWORD *)&v11 = *(_QWORD *)(v4 + 8);
    DWORD2(v11) = 0;
    if ( !v7 )
    {
      do
      {
        *(_QWORD *)&v11 = result;
        if ( *(_DWORD *)(result + 28) >= a2 )
        {
          DWORD2(v11) = 1;
          v4 = result;
          result = *(_QWORD *)result;
        }
        else
        {
          DWORD2(v11) = 0;
          result = *(_QWORD *)(result + 16);
        }
      }
      while ( !*(_BYTE *)(result + 25) );
    }
    if ( *(_BYTE *)(v4 + 25) || a2 < *(_DWORD *)(v4 + 28) )
    {
      if ( v3[1] == 0x7FFFFFFFFFFFFFFLL )
        sub_14014F360(a1, v6);
      v8 = *v3;
      v9 = sub_146E8BA20(32);
      *(_DWORD *)(v9 + 28) = a2;
      *(_QWORD *)v9 = v8;
      *(_QWORD *)(v9 + 8) = v8;
      *(_QWORD *)(v9 + 16) = v8;
      *(_WORD *)(v9 + 24) = 0;
      v10 = v11;
      return sub_14014F0E0(v3, &v10, v9);
    }
  }
  return result;
}


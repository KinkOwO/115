__int64 __fastcall sub_1473A1120(__int64 a1, int a2, signed int a3)
{
  __int64 v3; // rsi
  unsigned __int64 v5; // r10
  unsigned __int64 v6; // rdi
  unsigned __int64 v7; // rax
  char v8; // r8
  char v9; // r9
  char v10; // cl
  __int64 v11; // rcx
  __int64 v12; // rax

  v3 = a1;
  if ( a1 == -17 )
    v3 = 0;
  v5 = *(_QWORD *)(v3 + 8);
  v6 = v5;
  v7 = (*(_QWORD *)(v5 + 40) & 0xFFFFFFFFFFFFFFFEuLL) - 40;
  if ( (*(_QWORD *)(v5 + 40) & 0xFFFFFFFFFFFFFFFEuLL) == 0 )
    v7 = 0;
  if ( v7 )
  {
    v8 = -1;
    v9 = 1;
    do
    {
      if ( *(_DWORD *)v7 == a2 )
      {
        if ( *(_DWORD *)(v7 + 4) == a3 )
        {
          v10 = 0;
        }
        else
        {
          v10 = -1;
          if ( *(_DWORD *)(v7 + 4) >= a3 )
            v10 = 1;
        }
      }
      else
      {
        v10 = 1;
        if ( *(_DWORD *)v7 < a2 )
          v10 = -1;
      }
      if ( v10 < 0 )
      {
        v12 = *(_QWORD *)(v7 + 56);
        if ( !v12 )
          break;
        v7 = v12 - 40;
      }
      else
      {
        v11 = *(_QWORD *)(v7 + 48);
        v5 = v7;
        v7 = v11 - 40;
        if ( !v11 )
          v7 = 0;
      }
    }
    while ( v7 );
    if ( v5 != v6 )
    {
      if ( a2 == *(_DWORD *)v5 )
      {
        if ( a3 == *(_DWORD *)(v5 + 4) )
        {
          v8 = 0;
        }
        else if ( a3 >= *(_DWORD *)(v5 + 4) )
        {
          v8 = 1;
        }
      }
      else
      {
        if ( a2 < *(_DWORD *)v5 )
          v9 = -1;
        v8 = v9;
      }
      if ( v8 >= 0 )
        return *(unsigned int *)(v5 + 8);
    }
  }
  v5 = v6;
  if ( v6 == *(_QWORD *)(v3 + 8) )
    return (unsigned int)a3;
  else
    return *(unsigned int *)(v5 + 8);
}

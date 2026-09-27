// mgr_helper_ebeb0 = sub_1444EBEB0 0x1444EBEB0

__int64 __fastcall sub_1444EBEB0(__int64 a1, __int64 a2, __int64 a3)
{
  int v4; // eax
  unsigned int *v5; // rax
  __int64 v6; // rax
  __int64 result; // rax
  bool v8; // zf

  *(_QWORD *)a2 = -1;
  if ( !a3 )
    return a2;
  v4 = *(_DWORD *)(a3 + 2048);
  if ( v4 == 169 )
  {
    v5 = *(unsigned int **)(a3 + 2056);
    if ( v5 != *(unsigned int **)(a3 + 2064) )
    {
      LOBYTE(a3) = 1;
      v6 = sub_140283D60(qword_14E683BF8, *v5, a3);
      if ( v6 )
        *(_QWORD *)a2 = *(_QWORD *)(v6 + 8);
    }
    return a2;
  }
  if ( v4 == 224 )
  {
    result = a2;
    if ( *(_QWORD *)(a3 + 2056) != *(_QWORD *)(a3 + 2064) )
    {
      *(_DWORD *)a2 = 5;
      *(_DWORD *)(a2 + 4) = 4;
    }
  }
  else
  {
    v8 = v4 == 338;
    result = a2;
    if ( v8 )
    {
      *(_DWORD *)a2 = 9;
      *(_DWORD *)(a2 + 4) = 4;
    }
  }
  return result;
}


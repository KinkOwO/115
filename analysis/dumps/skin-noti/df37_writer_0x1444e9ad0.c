// writer_0x1444e9ad0

_DWORD *__fastcall sub_1444E9AD0(__int64 a1, int a2)
{
  _DWORD *v2; // r8
  _QWORD *v3; // r9
  _DWORD *result; // rax
  _DWORD *i; // rcx
  _DWORD *v7; // rdx

  v2 = *(_DWORD **)(a1 + 1184);
  v3 = (_QWORD *)(a1 + 1176);
  result = *(_DWORD **)(a1 + 1176);
  if ( result != v2 )
  {
    do
    {
      if ( *result == a2 )
        break;
      ++result;
    }
    while ( result != v2 );
    if ( result != v2 )
    {
      for ( i = result + 1; i != v2; ++i )
      {
        if ( *i != a2 )
          *result++ = *i;
      }
    }
  }
  if ( result != *(_DWORD **)(a1 + 1184) )
    *(_QWORD *)(a1 + 1184) = result;
  v7 = (_DWORD *)v3[1];
  if ( (_DWORD *)*v3 == v7 )
  {
    if ( v7 == (_DWORD *)v3[2] )
    {
      return (_DWORD *)sub_140154010(v3, v7, &unk_14A3176C8);
    }
    else
    {
      *v7 = 100000;
      v3[1] += 4LL;
    }
  }
  return result;
}


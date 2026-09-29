// writer_0x1444ec6e0

char __fastcall sub_1444EC6E0(__int64 a1, int a2)
{
  _DWORD *v2; // r8
  _DWORD *v3; // r9

  v2 = *(_DWORD **)(a1 + 1128);
  v3 = *(_DWORD **)(a1 + 1136);
  if ( v2 == v3 )
    return 0;
  while ( *v2 != a2 )
  {
    if ( ++v2 == v3 )
      return 0;
  }
  return 1;
}


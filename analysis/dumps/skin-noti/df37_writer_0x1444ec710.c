// writer_0x1444ec710

char __fastcall sub_1444EC710(__int64 a1, int a2)
{
  _DWORD *v2; // r8
  _DWORD *v3; // rax

  v2 = *(_DWORD **)(a1 + 1184);
  v3 = *(_DWORD **)(a1 + 1176);
  if ( v3 == v2 )
    return 0;
  while ( *v3 != a2 )
  {
    if ( ++v3 == v2 )
      return 0;
  }
  return 1;
}


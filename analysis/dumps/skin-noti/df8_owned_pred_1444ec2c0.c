bool __fastcall sub_1444EC2C0(__int64 a1)
{
  char v1; // dl
  char v2; // al
  unsigned int v3; // ebx

  v1 = *(_BYTE *)(a1 + 12);
  if ( v1 && !*(_DWORD *)(a1 + 8) )
    return 0;
  v2 = *(_BYTE *)(a1 + 20);
  if ( v2 )
  {
    if ( !*(_DWORD *)(a1 + 16) )
      return 0;
  }
  v3 = 0;
  if ( v1 )
    v3 = *(_DWORD *)(a1 + 8);
  if ( v2 && *(_DWORD *)(a1 + 8) < *(_DWORD *)(a1 + 16) )
    v3 = *(_DWORD *)(a1 + 16);
  return !v3 || (unsigned int)sub_145A11A50() > v3;
}

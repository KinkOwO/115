// mgr_helper_ec1b0 = sub_1444EC1B0 0x1444EC1B0

bool __fastcall sub_1444EC1B0(__int64 a1, unsigned int a2)
{
  __int64 v3; // rax
  __int64 v4; // r8
  unsigned int *v5; // rdx
  signed int v6; // ebx
  __int64 v7; // rax
  __int64 *v8; // r8
  __int64 *v9; // rdx
  __int64 *v10; // rax
  unsigned int v11; // ebx
  __int64 *v12; // rcx
  char v13; // al
  char v14; // dl

  v3 = sub_145ABB480(a2);
  if ( !v3 )
    return 0;
  if ( *(_DWORD *)(v3 + 2048) != 169 )
    return 0;
  v5 = *(unsigned int **)(v3 + 2056);
  if ( v5 == *(unsigned int **)(v3 + 2064) )
    return 0;
  v6 = *v5;
  LOBYTE(v4) = 1;
  v7 = sub_140283D60(qword_14E683BF8, *v5, v4);
  if ( !v7 )
    return 0;
  v8 = *(__int64 **)(a1 + 16LL * *(int *)(v7 + 8) + 136);
  v9 = v8;
  v10 = (__int64 *)v8[1];
  while ( !*((_BYTE *)v10 + 25) )
  {
    if ( *((_DWORD *)v10 + 7) >= v6 )
    {
      v9 = v10;
      v10 = (__int64 *)*v10;
    }
    else
    {
      v10 = (__int64 *)v10[2];
    }
  }
  if ( *((_BYTE *)v9 + 25) || v6 < *((_DWORD *)v9 + 7) )
    v9 = v8;
  v11 = 0;
  v12 = 0;
  if ( v9 != v8 )
    v12 = v9 + 4;
  if ( !v12 )
    return 0;
  v13 = *((_BYTE *)v12 + 12);
  if ( v13 )
  {
    if ( !*((_DWORD *)v12 + 2) )
      return 1;
  }
  v14 = *((_BYTE *)v12 + 20);
  if ( v14 )
  {
    if ( !*((_DWORD *)v12 + 4) )
      return 1;
  }
  if ( v13 )
    v11 = *((_DWORD *)v12 + 2);
  if ( v14 && *((_DWORD *)v12 + 2) < *((_DWORD *)v12 + 4) )
    v11 = *((_DWORD *)v12 + 4);
  return v11 && (unsigned int)sub_145A11A50() <= v11;
}


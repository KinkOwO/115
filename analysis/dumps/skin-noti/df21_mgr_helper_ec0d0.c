// mgr_helper_ec0d0 = sub_1444EC0D0 0x1444EC0D0

bool __fastcall sub_1444EC0D0(__int64 a1, __int64 a2, __int64 a3)
{
  int v4; // ebx
  __int64 v5; // rax
  __int64 *v6; // r8
  __int64 *v7; // rdx
  __int64 *v8; // rax
  unsigned int v9; // ebx
  __int64 *v10; // rcx
  char v11; // al
  char v12; // dl

  LOBYTE(a3) = 1;
  v4 = a2;
  v5 = sub_140283D60(qword_14E683BF8, a2, a3);
  if ( !v5 )
    return 0;
  v6 = *(__int64 **)(a1 + 16LL * *(int *)(v5 + 8) + 136);
  v7 = v6;
  v8 = (__int64 *)v6[1];
  while ( !*((_BYTE *)v8 + 25) )
  {
    if ( *((_DWORD *)v8 + 7) >= v4 )
    {
      v7 = v8;
      v8 = (__int64 *)*v8;
    }
    else
    {
      v8 = (__int64 *)v8[2];
    }
  }
  if ( *((_BYTE *)v7 + 25) || v4 < *((_DWORD *)v7 + 7) )
    v7 = v6;
  v9 = 0;
  v10 = 0;
  if ( v7 != v6 )
    v10 = v7 + 4;
  if ( !v10 )
    return 0;
  v11 = *((_BYTE *)v10 + 12);
  if ( v11 )
  {
    if ( !*((_DWORD *)v10 + 2) )
      return 1;
  }
  v12 = *((_BYTE *)v10 + 20);
  if ( v12 )
  {
    if ( !*((_DWORD *)v10 + 4) )
      return 1;
  }
  if ( v11 )
    v9 = *((_DWORD *)v10 + 2);
  if ( v12 && *((_DWORD *)v10 + 2) < *((_DWORD *)v10 + 4) )
    v9 = *((_DWORD *)v10 + 4);
  return v9 && (unsigned int)sub_145A11A50() <= v9;
}


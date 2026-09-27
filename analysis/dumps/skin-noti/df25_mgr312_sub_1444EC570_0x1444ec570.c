// mgr312_sub_1444EC570_0x1444ec570

bool __fastcall sub_1444EC570(__int64 a1, int a2)
{
  __int64 *v2; // r8
  __int64 *v3; // rcx
  __int64 *v4; // rax

  v2 = *(__int64 **)(a1 + 312);
  v3 = v2;
  v4 = (__int64 *)v2[1];
  while ( !*((_BYTE *)v4 + 25) )
  {
    if ( *((_DWORD *)v4 + 7) >= a2 )
    {
      v3 = v4;
      v4 = (__int64 *)*v4;
    }
    else
    {
      v4 = (__int64 *)v4[2];
    }
  }
  if ( *((_BYTE *)v3 + 25) || a2 < *((_DWORD *)v3 + 7) )
    v3 = v2;
  return v3 != v2;
}


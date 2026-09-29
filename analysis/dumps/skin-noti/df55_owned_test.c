// owned_test

__int64 *__fastcall sub_1444EBD90(__int64 a1, int a2, int a3)
{
  __int64 *v3; // rdx
  __int64 *v4; // rcx
  __int64 *v5; // rax

  v3 = *(__int64 **)(a1 + 16LL * a2 + 136);
  v4 = v3;
  v5 = (__int64 *)v3[1];
  while ( !*((_BYTE *)v5 + 25) )
  {
    if ( *((_DWORD *)v5 + 7) >= a3 )
    {
      v4 = v5;
      v5 = (__int64 *)*v5;
    }
    else
    {
      v5 = (__int64 *)v5[2];
    }
  }
  if ( *((_BYTE *)v4 + 25) || a3 < *((_DWORD *)v4 + 7) || v4 == v3 )
    return 0;
  else
    return v4 + 4;
}


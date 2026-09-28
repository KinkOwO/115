// bus_sub_14667BB90

__int64 __fastcall sub_14667BB90(__int64 a1, int a2, unsigned int a3)
{
  __int64 v3; // rdx
  __int64 v4; // r10
  unsigned __int64 v5; // r9

  v3 = 3LL * a2;
  v4 = *(_QWORD *)(a1 + 8 * v3 + 472);
  v5 = (*(_QWORD *)(a1 + 8 * v3 + 480) - v4) >> 3;
  if ( a3 >= v5 )
    return 0;
  if ( v5 <= (int)a3 )
    sub_1401790B0(a1, v3);
  return *(_QWORD *)(v4 + 8LL * (int)a3);
}


// fn_ebb90_0x1444ebb90

__int64 __fastcall sub_1444EBB90(__int64 a1, int a2)
{
  unsigned int v2; // r8d
  __int64 v3; // rsi
  _QWORD *v4; // rdi
  __int64 v5; // rbx

  v2 = 100000;
  if ( a2 < 4 )
  {
    v3 = *(_QWORD *)(a1 + 24 * (a2 + 50LL));
    v4 = (_QWORD *)(a1 + 24 * (a2 + 50LL));
    v5 = v4[1];
    if ( v3 != v5 )
      return *(unsigned int *)(*v4 + 4LL * (int)((int)sub_146E9BA90(a1) % (unsigned __int64)((v5 - v3) >> 2)));
  }
  return v2;
}


char __fastcall sub_1444EC320(__int64 a1, int a2, __int64 a3)
{
  _QWORD *v3; // rcx
  _QWORD *v4; // rax

  if ( !a2 && (_DWORD)a3 == 5 )
    a2 = 999999;
  v3 = *(_QWORD **)(a1 + 8);
  v4 = (_QWORD *)*v3;
  if ( (_QWORD *)*v3 == v3 )
    return 0;
  while ( *((_DWORD *)v4 + 4) != (_DWORD)a3 || *(_QWORD *)((char *)v4 + 20) != __PAIR64__(a2, HIDWORD(a3)) )
  {
    v4 = (_QWORD *)*v4;
    if ( v4 == v3 )
      return 0;
  }
  return 1;
}

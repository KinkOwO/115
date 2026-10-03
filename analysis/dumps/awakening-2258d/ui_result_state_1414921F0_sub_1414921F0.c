// sub_1414921F0  va=0x1414921F0  size=160

__int64 __fastcall sub_1414921F0(__int64 a1, int a2)
{
  __int64 result; // rax

  *(_DWORD *)(a1 + 1512) = a2;
  (*(void (__fastcall **)(__int64))(*(_QWORD *)(a1 + 1536) + 56LL))(a1 + 1536);
  result = *(unsigned int *)(a1 + 1512);
  if ( (_DWORD)result == 0 )
    return sub_141494520(a1, 0);
  while ( (_DWORD)result != 2 )
  {
    if ( (_DWORD)result == 3 )
      return sub_1414928D0(a1);
    if ( (_DWORD)result != 4 )
      return result;
    sub_1414928D0(a1);
    *(_DWORD *)(a1 + 1512) = 0;
    (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)(a1 + 1536) + 56LL))(a1 + 1536, 0);
    result = *(unsigned int *)(a1 + 1512);
    if ( (_DWORD)result == 0 )
      return sub_141494520(a1, 0);
  }
  return result;
}

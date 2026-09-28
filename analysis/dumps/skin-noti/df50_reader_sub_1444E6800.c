// reader_sub_1444E6800

__int64 __fastcall sub_1444E6800(__int64 a1)
{
  __int64 result; // rax
  __int64 v3; // rcx
  __int64 v4; // rcx

  result = sub_141FB6530(a1);
  if ( (_BYTE)result )
  {
    v3 = *(_QWORD *)(a1 + 920);
    if ( v3 )
      result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v3 + 16LL))(v3);
    v4 = *(_QWORD *)(a1 + 928);
    if ( v4 )
      return (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v4 + 16LL))(v4);
  }
  return result;
}


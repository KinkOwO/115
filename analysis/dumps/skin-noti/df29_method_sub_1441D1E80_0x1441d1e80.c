// method_sub_1441D1E80_0x1441d1e80

__int64 __fastcall sub_1441D1E80(__int64 a1, __int64 a2)
{
  if ( *(_QWORD *)(a1 + 56) )
  {
    sub_146ED3030();
    sub_146F58600(*(_QWORD *)(a1 + 56));
    (*(void (__fastcall **)(_QWORD, char *))(**(_QWORD **)(a1 + 56) + 672LL))(*(_QWORD *)(a1 + 56), &byte_14BAF7F08);
    *(_BYTE *)(a1 + 104) = 0;
  }
  if ( *(_QWORD *)(a1 + 40) )
    (*(void (__fastcall **)(__int64))(*(_QWORD *)a1 + 120LL))(a1);
  LOBYTE(a2) = 1;
  return (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 32LL))(a1, a2);
}


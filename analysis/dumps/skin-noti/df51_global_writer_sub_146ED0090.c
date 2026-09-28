// global_writer_sub_146ED0090

bool __fastcall sub_146ED0090(__int64 a1)
{
  int v1; // eax
  bool result; // al

  result = 0;
  if ( *(_BYTE *)(a1 + 141) )
  {
    v1 = *(_DWORD *)(a1 + 496);
    if ( dword_14DC6B648 == v1 || dword_14DC6B63C == v1 || dword_14DC6B644 == v1 )
      return 1;
  }
  return result;
}


// nb_sub_146ECFDB0

char __fastcall sub_146ECFDB0(__int64 a1)
{
  _QWORD *v1; // rdi
  _QWORD *v2; // rbx

  if ( !*(_BYTE *)(a1 + 141) || dword_14DC6B63C != *(_DWORD *)(a1 + 496) )
  {
    v1 = *(_QWORD **)(a1 + 288);
    v2 = (_QWORD *)*v1;
    if ( (_QWORD *)*v1 == v1 )
      return 0;
    while ( (unsigned __int8)sub_146ECFDB0(v2[2]) != 1 )
    {
      v2 = (_QWORD *)*v2;
      if ( v2 == v1 )
        return 0;
    }
  }
  return 1;
}


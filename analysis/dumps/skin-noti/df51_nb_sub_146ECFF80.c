// nb_sub_146ECFF80

char __fastcall sub_146ECFF80(__int64 a1)
{
  _QWORD *v2; // rdi
  _QWORD *v3; // rbx

  if ( !*(_BYTE *)(a1 + 141) || dword_14DC6B648 != *(_DWORD *)(a1 + 496) || (unsigned int)sub_146EC45C0() != 1 )
  {
    v2 = *(_QWORD **)(a1 + 288);
    v3 = (_QWORD *)*v2;
    if ( (_QWORD *)*v2 == v2 )
      return 0;
    while ( (unsigned __int8)sub_146ECFF80(v3[2]) != 1 )
    {
      v3 = (_QWORD *)*v3;
      if ( v3 == v2 )
        return 0;
    }
  }
  return 1;
}


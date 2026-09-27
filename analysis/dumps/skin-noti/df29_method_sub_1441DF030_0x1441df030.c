// method_sub_1441DF030_0x1441df030

__int64 __fastcall sub_1441DF030(_QWORD *a1)
{
  bool v2; // di
  __int64 v3; // rcx
  _QWORD *v4; // rsi
  __int64 v5; // rbp
  __int64 result; // rax
  char v7; // cl

  v2 = 0;
  v3 = a1[17];
  if ( v3 )
    v2 = (unsigned __int8)sub_146ED0010(v3) != 0;
  v4 = a1 + 25;
  v5 = 5;
  do
  {
    result = sub_146ED0010(*v4);
    v7 = v2;
    v4 += 50;
    if ( (_BYTE)result )
      v7 = 1;
    v2 = v7;
    --v5;
  }
  while ( v5 );
  if ( v7 )
  {
    if ( (unsigned __int8)sub_146EC4990() )
    {
      sub_146B25D40(a1[14]);
    }
    else
    {
      result = sub_146EC49A0();
      if ( !(_BYTE)result )
        return result;
      sub_146B25D90(a1[14]);
    }
    return (*(__int64 (__fastcall **)(_QWORD *, _QWORD))(*a1 + 32LL))(a1, 0);
  }
  return result;
}


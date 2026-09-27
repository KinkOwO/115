__int64 __fastcall sub_1444F1EE0(__int64 a1, unsigned int a2)
{
  __int64 result; // rax
  __int64 v4; // rsi
  int i; // ebx
  unsigned int v6; // eax
  __int64 v7; // rdi
  unsigned int v8; // eax
  __int64 v9; // rbx

  result = sub_14667BB90(qword_14E683C78, 730, 0);
  v4 = result;
  if ( result )
  {
    if ( a2 == 10 )
    {
      for ( i = 0; i < 10; ++i )
      {
        sub_1441ECDB0(v4, (unsigned int)i);
        v6 = sub_1441C4010(v4, (unsigned int)i);
        result = sub_1441C4080(v4, v6);
        v7 = result;
        if ( result )
        {
          (*(void (__fastcall **)(__int64))(*(_QWORD *)result + 24LL))(result);
          result = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v7 + 32LL))(v7, 0);
        }
      }
    }
    else
    {
      sub_1441ECDB0(result, a2);
      v8 = sub_1441C4010(v4, a2);
      result = sub_1441C4080(v4, v8);
      v9 = result;
      if ( result )
      {
        (*(void (__fastcall **)(__int64))(*(_QWORD *)result + 24LL))(result);
        return (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v9 + 32LL))(v9, 0);
      }
    }
  }
  return result;
}

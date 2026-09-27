__int64 sub_1444ED400()
{
  _QWORD *v0; // rsi
  int v1; // ebx
  __int64 result; // rax
  _QWORD *v3; // rdx
  int v4; // [rsp+20h] [rbp-18h] BYREF
  __int64 v5; // [rsp+24h] [rbp-14h] BYREF
  unsigned __int8 v6; // [rsp+50h] [rbp+18h] BYREF
  unsigned __int8 v7; // [rsp+58h] [rbp+20h] BYREF

  v0 = (_QWORD *)qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    qword_14E638F28 = sub_1444E7F30();
    (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
    v0 = (_QWORD *)qword_14E638F28;
  }
  v1 = 0;
  v6 = 0;
  v4 = 0;
  v7 = 0;
  sub_146EA09F0(&v6, 1);
  result = v0[137];
  v0[138] = result;
  if ( v6 )
  {
    do
    {
      sub_146EA09F0(&v7, 1);
      sub_146EA0BA0(&v4);
      v3 = (_QWORD *)v0[138];
      LODWORD(v5) = v4;
      HIDWORD(v5) = v7;
      if ( v3 == (_QWORD *)v0[139] )
      {
        sub_140183FB0(v0 + 137, v3, &v5);
      }
      else
      {
        *v3 = v5;
        v0[138] += 8LL;
      }
      result = v6;
      ++v1;
    }
    while ( v1 < v6 );
  }
  return result;
}

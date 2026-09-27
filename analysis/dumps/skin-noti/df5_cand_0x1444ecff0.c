__int64 sub_1444ECFF0()
{
  __int64 v0; // rbx
  int v1; // ebp
  int v2; // r14d
  _QWORD *v3; // rsi
  int v4; // ebx
  __int64 v5; // rdx
  __int64 v6; // rax
  void (__fastcall ***v7)(_QWORD); // rcx
  __int64 result; // rax
  unsigned __int8 v9; // [rsp+20h] [rbp-48h] BYREF
  _BYTE v10[7]; // [rsp+21h] [rbp-47h] BYREF
  __int64 v11; // [rsp+28h] [rbp-40h]
  __int64 v12; // [rsp+30h] [rbp-38h]
  __int128 v13; // [rsp+38h] [rbp-30h] BYREF
  __int64 v14; // [rsp+48h] [rbp-20h]
  char v15; // [rsp+50h] [rbp-18h]

  v11 = -2;
  v0 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    qword_14E638F28 = sub_1444E7F30();
    (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
    v0 = qword_14E638F28;
  }
  v10[0] = 0;
  sub_146EA09F0(v10, 1);
  v1 = 0;
  v2 = v10[0] == 0;
  v3 = (_QWORD *)(v0 + 24LL * (v10[0] == 0));
  v3[171] = v3[170];
  v9 = 0;
  sub_146EA09F0(&v9, 1);
  v4 = 0;
  if ( v9 )
  {
    do
    {
      v13 = 0;
      v14 = 0;
      v15 = 0;
      sub_146EA0BE0(&v13, 25);
      v5 = v3[171];
      if ( v5 == v3[172] )
      {
        sub_141FD7160(v3 + 170, v5, &v13);
      }
      else
      {
        *(_OWORD *)v5 = v13;
        *(_QWORD *)(v5 + 16) = v14;
        *(_BYTE *)(v5 + 24) = v15;
        v3[171] += 25LL;
      }
      ++v4;
    }
    while ( v4 < v9 );
  }
  if ( !qword_14E634230 )
  {
    v6 = sub_146E8BA20(112);
    v12 = v6;
    if ( v6 )
      v7 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v6);
    else
      v7 = 0;
    qword_14E634230 = (__int64)v7;
    (**v7)(v7);
  }
  result = sub_1403F8050();
  LOBYTE(v1) = (_BYTE)result == 0;
  if ( v1 == v2 )
  {
    result = sub_14667BB90(qword_14E683C78, 730, 0);
    if ( result )
      return sub_1441E4CD0(result);
  }
  return result;
}

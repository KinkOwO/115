// singleton_user_sub_14449CDB0

__int64 sub_14449CDB0()
{
  void (__fastcall ***v0)(_QWORD); // rbx
  __int64 v1; // rcx
  __int64 v2; // rax
  void (__fastcall ***v3)(_QWORD); // rcx
  __int64 v4; // rdx
  __int64 v5; // rax
  __int64 result; // rax
  int v7; // [rsp+20h] [rbp-28h] BYREF
  __int64 v8; // [rsp+28h] [rbp-20h]
  __int64 v9; // [rsp+30h] [rbp-18h]
  unsigned __int8 v10; // [rsp+60h] [rbp+18h] BYREF
  unsigned __int8 v11; // [rsp+68h] [rbp+20h] BYREF

  v8 = -2;
  v10 = 0;
  v11 = 0;
  v0 = 0;
  v7 = 0;
  sub_146EA09F0(&v10, 1);
  sub_146EA09F0(&v11, 1);
  sub_146EA0BA0(&v7);
  v1 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v2 = sub_146E8BA20(496);
    v9 = v2;
    if ( v2 )
      v3 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v2);
    else
      v3 = 0;
    qword_14E659EA8 = (__int64)v3;
    (**v3)(v3);
    v1 = qword_14E659EA8;
  }
  *(_DWORD *)(v1 + 8LL * v10 + 68) = v7;
  v4 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v5 = sub_146E8BA20(496);
    v9 = v5;
    if ( v5 )
      v0 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v5);
    qword_14E659EA8 = (__int64)v0;
    (**v0)(v0);
    v4 = qword_14E659EA8;
  }
  result = v10;
  *(_DWORD *)(v4 + 8LL * v10 + 64) = v11;
  return result;
}


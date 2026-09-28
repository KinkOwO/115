// singleton_user_sub_14449D200

__int64 sub_14449D200()
{
  void (__fastcall ***v0)(_QWORD); // rbx
  _DWORD *v1; // r10
  __int64 v2; // rax
  int v3; // r9d
  int v4; // r8d
  int v5; // edx
  int v6; // ecx
  __int64 result; // rax
  int v8; // [rsp+20h] [rbp-28h] BYREF
  int v9; // [rsp+24h] [rbp-24h] BYREF
  unsigned int v10; // [rsp+28h] [rbp-20h] BYREF
  __int64 v11; // [rsp+30h] [rbp-18h]
  __int64 v12; // [rsp+38h] [rbp-10h]
  int v13; // [rsp+60h] [rbp+18h] BYREF
  int v14; // [rsp+68h] [rbp+20h] BYREF

  v11 = -2;
  v0 = 0;
  v10 = 0;
  v9 = 0;
  v8 = 0;
  v14 = 0;
  v13 = 0;
  sub_146EA0BA0(&v10);
  sub_146EA0BA0(&v9);
  sub_146EA0BA0(&v8);
  sub_146EA0BA0(&v14);
  sub_146EA0BA0(&v13);
  v1 = (_DWORD *)qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v2 = sub_146E8BA20(496);
    v12 = v2;
    if ( v2 )
      v0 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v2);
    qword_14E659EA8 = (__int64)v0;
    (**v0)(v0);
    v1 = (_DWORD *)qword_14E659EA8;
  }
  v3 = v13;
  v4 = v14;
  v5 = v8;
  v6 = v9;
  result = v10;
  v1[94] = v10;
  v1[95] = v6;
  v1[96] = v5;
  v1[97] = v4;
  v1[98] = v3;
  return result;
}


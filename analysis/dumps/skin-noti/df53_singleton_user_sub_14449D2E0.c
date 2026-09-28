// singleton_user_sub_14449D2E0

__int64 sub_14449D2E0()
{
  void (__fastcall ***v0)(_QWORD); // rbx
  __int64 v1; // r11
  __int64 v2; // rax
  int v3; // r10d
  int v4; // r9d
  int v5; // r8d
  int v6; // ecx
  int v7; // edx
  __int64 result; // rax
  int v9; // [rsp+20h] [rbp-28h] BYREF
  int v10; // [rsp+24h] [rbp-24h] BYREF
  int v11; // [rsp+28h] [rbp-20h] BYREF
  int v12; // [rsp+2Ch] [rbp-1Ch] BYREF
  __int64 v13; // [rsp+30h] [rbp-18h]
  __int64 v14; // [rsp+38h] [rbp-10h]
  unsigned __int8 v15; // [rsp+60h] [rbp+18h] BYREF
  char v16; // [rsp+68h] [rbp+20h] BYREF

  v13 = -2;
  v16 = 0;
  v0 = 0;
  v12 = 0;
  v11 = 0;
  v10 = 0;
  v9 = 0;
  v15 = 0;
  sub_146EA09F0(&v16);
  sub_146EA0BA0(&v12);
  sub_146EA0BA0(&v11);
  sub_146EA0BA0(&v10);
  sub_146EA0BA0(&v9);
  sub_146EA09F0(&v15);
  v1 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v2 = sub_146E8BA20(496);
    v14 = v2;
    if ( v2 )
      v0 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v2);
    qword_14E659EA8 = (__int64)v0;
    (**v0)(v0);
    v1 = qword_14E659EA8;
  }
  v3 = v15;
  v4 = v9;
  v5 = v10;
  v6 = v11;
  v7 = v12;
  *(_BYTE *)(v1 + 352) = v16 == 1;
  *(_DWORD *)(v1 + 360) = v6;
  *(_DWORD *)(v1 + 356) = v7;
  *(_DWORD *)(v1 + 364) = v5;
  *(_DWORD *)(v1 + 368) = v4;
  *(_DWORD *)(v1 + 372) = v3;
  result = sub_1459A9080(qword_14E66C090);
  if ( result )
    return sub_146E9FBC0(result + 6104);
  return result;
}


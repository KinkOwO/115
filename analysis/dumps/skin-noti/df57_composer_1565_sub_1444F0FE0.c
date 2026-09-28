// composer_1565_sub_1444F0FE0

__int64 __fastcall sub_1444F0FE0(__int64 a1, int a2, unsigned int a3, char a4)
{
  __int64 v5; // rcx
  __int64 v6; // rax
  __int64 v7; // rcx
  __int64 v8; // rax
  __int64 v9; // rdx
  __int64 v10; // rcx
  _DWORD v12[2]; // [rsp+20h] [rbp-78h] BYREF
  __int128 v13; // [rsp+28h] [rbp-70h]
  __int128 v14; // [rsp+38h] [rbp-60h]
  __int128 v15; // [rsp+48h] [rbp-50h]
  __int128 v16; // [rsp+58h] [rbp-40h]
  __int128 v17; // [rsp+68h] [rbp-30h]

  v12[1] = (a4 != 0) + 2;
  v13 = 0;
  v14 = 0;
  v15 = 0;
  v16 = 0;
  v17 = 0;
  LODWORD(v13) = sub_1473A1580(a1 + 1328, a3);
  v12[0] = a2;
  v6 = sub_146D74000(v5);
  sub_146D746E0(v6, 1565);
  v8 = sub_146D74000(v7);
  sub_146D75B10(v8, v12, 88);
  return sub_146D75AF0(v10, v9);
}


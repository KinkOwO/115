// composer_1565_sub_1444F1880

void __fastcall sub_1444F1880(__int64 a1, int a2, int a3)
{
  __int64 v3; // rax
  __int64 v4; // rcx
  __int64 v5; // rax
  __int64 v6; // rdx
  __int64 v7; // rcx
  _DWORD v8[2]; // [rsp+20h] [rbp-78h] BYREF
  __int128 v9; // [rsp+28h] [rbp-70h]
  __int128 v10; // [rsp+38h] [rbp-60h]
  __int128 v11; // [rsp+48h] [rbp-50h]
  __int128 v12; // [rsp+58h] [rbp-40h]
  __int128 v13; // [rsp+68h] [rbp-30h]

  if ( a3 != -1 )
  {
    v8[0] = a2;
    v9 = 0;
    LODWORD(v9) = a3;
    v10 = 0;
    v8[1] = 4;
    v11 = 0;
    v12 = 0;
    v13 = 0;
    v3 = sub_146D74000(a1);
    sub_146D746E0(v3, 1565);
    v5 = sub_146D74000(v4);
    sub_146D75B10(v5, v8, 88);
    sub_146D75AF0(v7, v6);
  }
}


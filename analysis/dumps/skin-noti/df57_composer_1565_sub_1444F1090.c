// composer_1565_sub_1444F1090

__int64 __fastcall sub_1444F1090(__int64 a1, int a2, _QWORD *a3)
{
  __int64 v4; // rbp
  _DWORD *v6; // rdx
  __int64 v7; // rdi
  __int64 v8; // rsi
  _DWORD *v9; // rdx
  __int64 v10; // rsi
  _DWORD *v11; // rdx
  __int64 v12; // rax
  __int64 v13; // rcx
  __int64 v14; // rsi
  __int64 v15; // rax
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // rcx
  int v21; // [rsp+20h] [rbp-B8h] BYREF
  __int64 v22; // [rsp+28h] [rbp-B0h]
  _QWORD *v23; // [rsp+30h] [rbp-A8h]
  _DWORD v24[24]; // [rsp+40h] [rbp-98h] BYREF

  v22 = -2;
  v4 = a2;
  v23 = a3;
  v6 = (_DWORD *)a3[1];
  v7 = 0;
  if ( (_DWORD *)*a3 == v6 )
  {
    switch ( v4 )
    {
      case 0LL:
        if ( v6 == (_DWORD *)a3[2] )
        {
          sub_140154010(a3, v6, &unk_14A3176C0);
        }
        else
        {
          *v6 = 20000;
          a3[1] += 4LL;
        }
        break;
      case 1LL:
        if ( v6 == (_DWORD *)a3[2] )
        {
          sub_140154010(a3, v6, &unk_14A3176C4);
        }
        else
        {
          *v6 = 30000;
          a3[1] += 4LL;
        }
        break;
      case 2LL:
        if ( v6 == (_DWORD *)a3[2] )
        {
          sub_140154010(a3, v6, &unk_14A3176CC);
        }
        else
        {
          *v6 = 1;
          a3[1] += 4LL;
        }
        break;
      case 3LL:
        v8 = 4;
        do
        {
          v21 = 0;
          v9 = (_DWORD *)a3[1];
          if ( v9 == (_DWORD *)a3[2] )
          {
            sub_140154010(a3, v9, &v21);
          }
          else
          {
            *v9 = 0;
            a3[1] += 4LL;
          }
          --v8;
        }
        while ( v8 );
        break;
      case 4LL:
        v21 = 0;
        if ( v6 == (_DWORD *)a3[2] )
        {
          sub_140154010(a3, v6, &v21);
        }
        else
        {
          *v6 = 0;
          a3[1] += 4LL;
        }
        break;
      case 6LL:
        if ( v6 == (_DWORD *)a3[2] )
        {
          sub_140154010(a3, v6, &unk_14A317CF0);
        }
        else
        {
          *v6 = 99999999;
          a3[1] += 4LL;
        }
        break;
      case 9LL:
        v10 = 4;
        do
        {
          v21 = 0;
          v11 = (_DWORD *)a3[1];
          if ( v11 == (_DWORD *)a3[2] )
          {
            sub_140154010(a3, v11, &v21);
          }
          else
          {
            *v11 = 0;
            a3[1] += 4LL;
          }
          --v10;
        }
        while ( v10 );
        break;
      default:
        break;
    }
  }
  v12 = (__int64)(a3[1] - *a3) >> 2;
  v13 = 0;
  memset(&v24[1], 0, 84);
  v24[0] = v4;
  v14 = (int)v12;
  if ( (int)v12 > 0 )
  {
    do
    {
      if ( v7 >= 20 )
        break;
      v24[v7 + 2] = sub_1473A1580(a1 + 1328, *(unsigned int *)(*a3 + 4 * v7));
      ++v7;
    }
    while ( v7 < v14 );
  }
  v15 = sub_146D74000(v13);
  sub_146D746E0(v15, 1565);
  v17 = sub_146D74000(v16);
  sub_146D75B10(v17, v24, 88);
  sub_146D75AF0(v19, v18);
  return sub_1401574A0(a3);
}


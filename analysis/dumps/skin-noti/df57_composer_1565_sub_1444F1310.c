// composer_1565_sub_1444F1310

__int64 __fastcall sub_1444F1310(unsigned __int64 a1, int a2, int a3, _QWORD *a4)
{
  unsigned __int64 v5; // rbp
  int v6; // ebx
  __int64 v7; // rdi
  __int64 v8; // rax
  __int64 v9; // rcx
  __int64 v10; // rax
  __int64 v11; // rdx
  __int64 v12; // rcx
  _DWORD v14[2]; // [rsp+30h] [rbp-98h] BYREF
  _OWORD v15[5]; // [rsp+38h] [rbp-90h] BYREF

  v5 = a1;
  memset(v15, 0, sizeof(v15));
  v14[1] = a2;
  v14[0] = a3;
  v6 = 0;
  if ( (__int64)(a4[1] - *a4) >> 2 )
  {
    v7 = 0;
    do
    {
      if ( v6 >= 20 )
        break;
      *(_DWORD *)((char *)v15 + v7) = sub_1473A1580(v5 + 1328, *(unsigned int *)(v7 + *a4));
      ++v6;
      v7 += 4;
      a1 = (__int64)(a4[1] - *a4) >> 2;
    }
    while ( v6 < a1 );
  }
  v8 = sub_146D74000(a1);
  sub_146D746E0(v8, 1565);
  v10 = sub_146D74000(v9);
  sub_146D75B10(v10, v14, 88);
  sub_146D75AF0(v12, v11);
  return sub_1401574A0(a4);
}


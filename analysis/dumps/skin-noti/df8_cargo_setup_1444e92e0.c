__int64 __fastcall sub_1444E92E0(_QWORD *a1)
{
  _QWORD *v2; // rbx
  __int64 v3; // rsi
  _QWORD *v4; // rdi
  _QWORD *v5; // rbx
  _QWORD *v6; // rbx
  _QWORD *v7; // rbx
  _QWORD *v8; // rbx
  __int64 v9; // rax
  _QWORD *v10; // rcx
  _QWORD *v11; // rbx
  __int64 result; // rax

  v2 = a1 + 17;
  v3 = 10;
  do
  {
    v4 = (_QWORD *)*v2;
    sub_1401DBB80(v2, v2, *(_QWORD *)(*v2 + 8LL));
    v4[1] = v4;
    *v4 = v4;
    v4[2] = v4;
    v2[1] = 0;
    v2 += 2;
    --v3;
  }
  while ( v3 );
  v5 = (_QWORD *)a1[129];
  sub_1401E8880(a1 + 129, a1 + 129, v5[1]);
  v5[1] = v5;
  *v5 = v5;
  v5[2] = v5;
  a1[130] = 0;
  v6 = (_QWORD *)a1[131];
  sub_14014EBD0(a1 + 131, a1 + 131, v6[1]);
  v6[1] = v6;
  *v6 = v6;
  v6[2] = v6;
  a1[132] = 0;
  v7 = (_QWORD *)a1[133];
  sub_14014EBD0(a1 + 133, a1 + 133, v7[1]);
  v7[1] = v7;
  *v7 = v7;
  v7[2] = v7;
  a1[134] = 0;
  v8 = (_QWORD *)a1[135];
  sub_1401E8880(a1 + 135, a1 + 135, v8[1]);
  v8[1] = v8;
  *v8 = v8;
  v8[2] = v8;
  a1[136] = 0;
  sub_1444F0A40(a1);
  v9 = sub_146E8C7D0(&unk_14A3186C0);
  sub_1474253B0(v9, a1 + 166);
  sub_146E9FBC0(a1 + 176);
  v10 = a1 + 150;
  a1[145] = a1[144];
  for ( a1[148] = a1[147]; v10 != a1 + 162; v10 += 3 )
    v10[1] = *v10;
  v11 = (_QWORD *)a1[39];
  result = sub_140178C60(a1 + 39, a1 + 39, v11[1]);
  v11[1] = v11;
  *v11 = v11;
  v11[2] = v11;
  a1[40] = 0;
  return result;
}

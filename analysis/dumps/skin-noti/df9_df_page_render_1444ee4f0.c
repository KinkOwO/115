__int64 __fastcall sub_1444EE4F0(__int64 a1, char a2, unsigned __int16 a3, int a4, __int64 a5, __int64 *a6)
{
  int v6; // ebx
  int v7; // r9d
  int v8; // r9d
  __int64 v9; // r8
  __int64 v10; // rsi
  __int64 v11; // r14
  __int64 v12; // rbx
  __int64 v13; // r14
  __int64 v14; // rax
  __int64 v15; // rbx
  __int64 v16; // rax
  _BYTE v18[24]; // [rsp+30h] [rbp-48h] BYREF
  unsigned int v19; // [rsp+48h] [rbp-30h] BYREF
  __int64 v20; // [rsp+50h] [rbp-28h] BYREF
  __int64 v21; // [rsp+58h] [rbp-20h]
  __int64 v22; // [rsp+60h] [rbp-18h]

  if ( a2 )
  {
    v6 = 0;
    if ( a4 )
    {
      v7 = a4 - 1;
      if ( v7 )
      {
        v8 = v7 - 1;
        if ( v8 )
        {
          if ( v8 == 1 )
            v6 = 4;
        }
        else
        {
          v6 = 2;
        }
      }
      else
      {
        v6 = 3;
      }
    }
    else
    {
      v6 = 1;
    }
    if ( qword_14E683C78 && qword_14E66C090 && (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
    {
      sub_142CBEBE0(&v19);
      v19 = v6;
      v10 = v20;
      v21 = v20;
      if ( a6 )
      {
        v11 = a6[1];
        v12 = *a6;
        if ( *a6 != v11 )
        {
          v13 = v11 - v12;
          if ( v13 >> 2 > (unsigned __int64)((v22 - v20) >> 2) )
          {
            sub_1401C4C50(&v20);
            v10 = v20;
          }
          sub_148AA1E60(v10, v12, v13);
          v21 = v13 + v10;
        }
      }
      LOBYTE(v9) = 1;
      if ( (unsigned __int8)sub_146682140(qword_14E683C78, 730, v9) )
      {
        v14 = sub_14667BB90(qword_14E683C78, 730, 0);
        v15 = v14;
        if ( v14 )
        {
          sub_1441E1C70(v14, v19, 0xFFFFFFFFLL);
          if ( a6 )
          {
            v16 = sub_1401550D0(v18, a6);
            sub_1441BEA80(v15, v16);
          }
        }
        sub_1401574A0(&v20);
      }
      else
      {
        sub_14668C520(qword_14E683C78, 730, &v19, 0);
        sub_1401574A0(&v20);
      }
    }
  }
  else
  {
    sub_1444EC790(a3);
  }
  return sub_1401574A0(a6);
}

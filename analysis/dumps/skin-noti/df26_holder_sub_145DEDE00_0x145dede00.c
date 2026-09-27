// holder_sub_145DEDE00_0x145dede00

__int64 __fastcall sub_145DEDE00(__int64 a1, int a2, unsigned __int8 a3)
{
  __int64 v4; // rbp
  void (__fastcall ***v6)(_QWORD); // rdi
  unsigned __int64 v7; // r14
  __int64 v8; // r8
  __int64 v9; // rsi
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rbx
  _QWORD *v13; // rax
  _QWORD *v14; // rax
  unsigned __int64 v15; // rsi
  __int64 v16; // rbx
  __int64 v17; // rax
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rcx
  __int64 v21; // rax
  void (__fastcall ***v22)(_QWORD); // rcx
  __int64 v23; // r9
  __int64 v24; // rcx
  __int64 v25; // rax
  __int64 v26; // rax
  __int64 v27; // rax
  __int64 v28; // rax
  __int64 v30; // [rsp+28h] [rbp-40h] BYREF
  __int64 v31; // [rsp+30h] [rbp-38h]

  v4 = a2;
  if ( a2 == 5 )
    sub_145E04300(a1);
  sub_144D03C50(&v30, a1 + 8 * (v4 + 2 * v4 + 22));
  v6 = 0;
  v7 = 0;
  v8 = v30;
  if ( (v31 - v30) / 24 )
  {
    v9 = 0;
    do
    {
      v10 = *(_QWORD *)(v9 + v8 + 8);
      if ( v10 && *(_DWORD *)(v10 + 8) )
      {
        v11 = *(_QWORD *)(v9 + v8 + 16);
        v12 = v11 - 48;
        if ( !v11 )
          v12 = 0;
        if ( v12 )
        {
          if ( *(_QWORD *)sub_145B89A50(v12) )
          {
            v13 = (_QWORD *)sub_145B89A50(v12);
            if ( (unsigned int)sub_146B33CC0(*v13) == -1 )
            {
              v14 = (_QWORD *)sub_145B89A50(v12);
              sub_146B34310(*v14);
            }
          }
          v8 = v30;
        }
      }
      ++v7;
      v9 += 24;
    }
    while ( v7 < (v31 - v8) / 24 );
  }
  if ( (_DWORD)v4 == 8 )
  {
    v15 = 0;
    if ( (v31 - v8) / 24 )
    {
      v16 = 0;
      do
      {
        v17 = *(_QWORD *)(v16 + v8 + 8);
        if ( v17 && *(_DWORD *)(v17 + 8) )
        {
          v18 = *(_QWORD *)(v16 + v8 + 16);
          v19 = v18 - 48;
          if ( !v18 )
            v19 = 0;
          if ( v19 )
          {
            (*(void (__fastcall **)(__int64))(*(_QWORD *)v19 + 120LL))(v19);
            v8 = v30;
          }
        }
        ++v15;
        v16 += 24;
      }
      while ( v15 < (v31 - v8) / 24 );
    }
  }
  else
  {
    v20 = qword_14E651768;
    if ( !qword_14E651768 )
    {
      v21 = sub_146E8BA20(312);
      if ( v21 )
        v22 = (void (__fastcall ***)(_QWORD))sub_1456BA920(v21);
      else
        v22 = 0;
      qword_14E651768 = (__int64)v22;
      (**v22)(v22);
      v20 = qword_14E651768;
    }
    if ( (unsigned __int8)sub_1456BC730(v20, (unsigned int)v4) )
    {
      v24 = qword_14E651768;
      if ( !qword_14E651768 )
      {
        v25 = sub_146E8BA20(312);
        if ( v25 )
          v6 = (void (__fastcall ***)(_QWORD))sub_1456BA920(v25);
        qword_14E651768 = (__int64)v6;
        (**v6)(v6);
        v24 = qword_14E651768;
      }
      LOBYTE(v23) = (_DWORD)v4 == 1;
      sub_1456BBE70(v24, (unsigned int)v4, &v30, v23, -2);
    }
    else
    {
      sub_145DED980(a1, &v30, (unsigned int)v4, a3, -2);
      if ( qword_14E63AE60 )
        goto LABEL_41;
      v26 = sub_146E8BA20(336);
      if ( v26 )
        v6 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v26);
      qword_14E63AE60 = (__int64)v6;
      (**v6)(v6);
      if ( qword_14E63AE60 )
      {
LABEL_41:
        v27 = sub_14319C890();
        if ( !(unsigned __int8)sub_1421BF680(v27) && !(_DWORD)v4 )
        {
          v28 = sub_143C61150();
          sub_1447E9F70(v28);
        }
      }
    }
  }
  return sub_1401F5BE0(&v30);
}


// holder_sub_1452E8370_0x1452e8370

__int64 sub_1452E8370()
{
  void (__fastcall ***v0)(_QWORD); // r14
  int v1; // ebp
  __int64 v2; // rax
  void (__fastcall ***v3)(_QWORD); // rcx
  __int64 v4; // rbx
  int v5; // edi
  int v6; // eax
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rdx
  __int64 v11; // rax
  __int64 v12; // rcx
  __int64 v13; // rax
  __int64 v14; // r8
  __int64 v15; // rax
  void (__fastcall ***v16)(_QWORD); // rcx
  __int64 v17; // rax
  void (__fastcall ***v18)(_QWORD); // rcx
  __int64 v19; // rdx
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 result; // rax
  __int64 v23; // rax
  int v24; // [rsp+28h] [rbp-60h]
  _QWORD v25[4]; // [rsp+50h] [rbp-38h] BYREF
  unsigned __int8 v26; // [rsp+A0h] [rbp+18h] BYREF
  unsigned __int8 v27; // [rsp+A8h] [rbp+20h] BYREF

  *(_BYTE *)(sub_1451C94C0() + 36) = 0;
  sub_145F0D2E0(qword_14E683C08);
  sub_146D8A260(qword_14F0EA870);
  v26 = 0;
  sub_146EA09F0(&v26, 1);
  sub_146EA09F0(&v27, 1);
  v0 = 0;
  v1 = qword_14E6343D0;
  if ( !qword_14E6343D0 )
  {
    v2 = sub_146E8BA20(72);
    if ( v2 )
      v3 = (void (__fastcall ***)(_QWORD))sub_146E93360(v2);
    else
      v3 = 0;
    qword_14E6343D0 = (__int64)v3;
    (**v3)(v3);
    v1 = qword_14E6343D0;
  }
  v4 = sub_146E8C7D0(&unk_14A7A7DB0);
  v5 = sub_146E8C7D0(&unk_14A7A7DF0);
  v6 = sub_146E8C7D0(&unk_14A7A5C80);
  sub_146E938E0(v1, 4, v6, v5, 25501, (__int64)&qword_14E66C2B8, v4);
  LODWORD(v4) = v26;
  v7 = sub_14601ACD0();
  *(_DWORD *)(sub_14014F460(v7) + 136) = v4;
  v8 = sub_14601ACD0();
  (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v8 + 32LL))(v8, v27);
  v9 = sub_1413085D0(qword_14E66C090);
  sub_146D5B010(v9);
  sub_145E26570(qword_14E682918);
  LOBYTE(v10) = 1;
  sub_145E297F0(qword_14E682918, v10);
  v11 = sub_1413085D0(qword_14E66C090);
  (*(void (__fastcall **)(__int64))(*(_QWORD *)v11 + 816LL))(v11);
  LOBYTE(v12) = 1;
  sub_146D5BFE0(v12);
  v13 = sub_14723C170(35311);
  v25[0] = 0;
  v25[2] = 0;
  v25[3] = 7;
  v14 = -1;
  do
    ++v14;
  while ( *(_WORD *)(v13 + 2 * v14) );
  sub_14014C8D0(v25, v13);
  sub_14515E840(v25);
  sub_14515D110();
  LOBYTE(v24) = 1;
  sub_144F609D0(qword_14E683D40, 25, 0, 0, 1, v24);
  if ( !qword_14E6367D0 )
  {
    v15 = sub_146E8BA20(192);
    if ( v15 )
      v16 = (void (__fastcall ***)(_QWORD))sub_146E018A0(v15);
    else
      v16 = 0;
    qword_14E6367D0 = (__int64)v16;
    (**v16)(v16);
  }
  if ( (unsigned __int8)sub_143865540() )
  {
    if ( !qword_14E6367D0 )
    {
      v17 = sub_146E8BA20(192);
      if ( v17 )
        v18 = (void (__fastcall ***)(_QWORD))sub_146E018A0(v17);
      else
        v18 = 0;
      qword_14E6367D0 = (__int64)v18;
      (**v18)(v18);
    }
    sub_146E02690();
  }
  if ( sub_14601ACC0() )
  {
    v20 = qword_14E63AE60;
    if ( !qword_14E63AE60 )
    {
      v21 = sub_146E8BA20(336);
      if ( v21 )
        v0 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v21);
      qword_14E63AE60 = (__int64)v0;
      (**v0)(v0);
      v20 = qword_14E63AE60;
    }
    sub_1447EF520(v20, v19);
  }
  result = sub_14601ACC0();
  if ( result )
  {
    v23 = sub_14601ACC0();
    return sub_14601A770(v23);
  }
  return result;
}


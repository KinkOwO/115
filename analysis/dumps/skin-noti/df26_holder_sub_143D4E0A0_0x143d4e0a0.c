// holder_sub_143D4E0A0_0x143d4e0a0

__int64 __fastcall sub_143D4E0A0(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // r8
  __int64 v4; // rax
  __int64 v5; // rdx
  __int64 v6; // rax
  __int64 v7; // rdx
  __int64 v8; // rax
  __int64 v9; // rdx
  __int64 v10; // rax
  __int64 v11; // rax
  __int64 v12; // rdx
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // rax
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // rax
  void (__fastcall ***v20)(_QWORD); // rcx

  v2 = sub_145EFAFB0();
  LOBYTE(v3) = 1;
  (*(void (__fastcall **)(__int64, _QWORD, __int64))(*(_QWORD *)v2 + 1968LL))(v2, 0, v3);
  v4 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v5) = 1;
  sub_145587710(v4, v5);
  v6 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v7) = 1;
  sub_145587720(v6, v7);
  v8 = sub_145EFAFB0();
  LOBYTE(v9) = 1;
  sub_145D36A70(v8, v9);
  v10 = sub_145EFAFB0();
  (*(void (__fastcall **)(__int64, __int64, __int64))(*(_QWORD *)v10 + 424LL))(v10, 2, 1000000);
  if ( sub_14667C540(qword_14E683C78) )
  {
    v11 = sub_14667C540(qword_14E683C78);
    LOBYTE(v12) = 1;
    sub_142D9A3E0(v11, v12);
    v13 = sub_14667C540(qword_14E683C78);
    sub_146A18E50(v13, 0);
  }
  v14 = sub_1429BDDE0(qword_14E683C78);
  sub_145586120(v14, 0);
  *(_DWORD *)(a1 + 23960) = dword_14DC6918C;
  dword_14DC6918C = 1065353216;
  v15 = sub_145EFAFB0();
  if ( (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 6048LL))(v15) )
  {
    v16 = sub_145EFAFB0();
    v17 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v16 + 6048LL))(v16);
    LOBYTE(v18) = 1;
    sub_145E4B250(v17, v18);
  }
  if ( !qword_14E63AE60 )
  {
    v19 = sub_146E8BA20(336);
    if ( v19 )
      v20 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v19);
    else
      v20 = 0;
    qword_14E63AE60 = (__int64)v20;
    (**v20)(v20);
  }
  sub_1447EF2C0(0);
  sub_146E9FBD0(a1 + 23880, 0, 0);
  return sub_144D2B6F0(a1);
}


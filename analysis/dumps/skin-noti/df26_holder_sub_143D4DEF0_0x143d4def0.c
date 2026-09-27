// holder_sub_143D4DEF0_0x143d4def0

__int64 __fastcall sub_143D4DEF0(__int64 a1)
{
  __int64 v2; // rax
  __int64 v3; // rax
  __int64 v4; // rdx
  __int64 v5; // rax
  __int64 v6; // rdx
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rdx
  __int64 v11; // rax
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // rdx
  __int64 v16; // rax
  __int64 v17; // rdx
  __int64 v18; // rax
  __int64 v19; // rcx
  __int64 v20; // rax
  __int64 v21; // rax
  __int64 v22; // rax
  void (__fastcall ***v23)(_QWORD); // rcx
  __int64 result; // rax

  v2 = sub_145EFAFB0();
  (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v2 + 1968LL))(v2, 1);
  v3 = sub_145EFAFB0();
  LOBYTE(v4) = 1;
  sub_145D30F00(v3, v4);
  v5 = sub_145EFAFB0();
  LOBYTE(v6) = 1;
  (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v5 + 3712LL))(v5, v6);
  v7 = sub_146E8C7D0(&unk_14A0E8780);
  v8 = sub_145A0F110(v7, 232);
  (*(void (__fastcall **)(__int64, _QWORD, __int64))(*(_QWORD *)v8 + 136LL))(v8, 0, 1);
  v9 = sub_145EFAFB0();
  sub_145D36A70(v9, 0);
  LOBYTE(v10) = 1;
  sub_146696C60(qword_14E683C78, v10);
  v11 = sub_1429BDDE0(qword_14E683C78);
  sub_145587710(v11, 0);
  v12 = sub_1429BDDE0(qword_14E683C78);
  sub_145587720(v12, 0);
  if ( sub_14667C540(qword_14E683C78) )
  {
    v13 = sub_14667C540(qword_14E683C78);
    sub_142D9A3E0(v13, 0);
    v14 = sub_14667C540(qword_14E683C78);
    LOBYTE(v15) = 1;
    sub_146A18E50(v14, v15);
  }
  v16 = sub_1429BDDE0(qword_14E683C78);
  LOBYTE(v17) = 1;
  sub_145586120(v16, v17);
  v18 = sub_145EFAFB0();
  if ( (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v18 + 6048LL))(v18) )
  {
    v20 = sub_145EFAFB0();
    v21 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v20 + 6048LL))(v20);
    sub_145E4B250(v21, 0);
  }
  if ( !qword_14E63AE60 )
  {
    v22 = sub_146E8BA20(336);
    if ( v22 )
      v23 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v22);
    else
      v23 = 0;
    qword_14E63AE60 = (__int64)v23;
    (**v23)(v23);
  }
  LOBYTE(v19) = 1;
  result = sub_1447EF2C0(v19);
  dword_14DC6918C = *(_DWORD *)(a1 + 23960);
  return result;
}


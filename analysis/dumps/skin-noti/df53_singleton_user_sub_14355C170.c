// singleton_user_sub_14355C170

__int64 __fastcall sub_14355C170(__int64 a1)
{
  __int64 result; // rax
  __int64 v3; // rdx
  __int64 v4; // rcx
  bool v5; // di
  __int64 v6; // rax
  __int64 v7; // rax
  __int64 v8; // rdi
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  char v11; // al
  __int64 v12; // r8
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rax
  __int64 v16; // rdx
  __int64 v17; // rcx

  result = sub_1459AA4F0(qword_14E66C090);
  if ( (_DWORD)result == 27 )
  {
    if ( sub_14667BB90(qword_14E683C78, 14, 0) )
      sub_146694510(qword_14E683C78, 14, -1, 0, 1);
    if ( (unsigned __int8)sub_145F14EE0(qword_14E683C20, 2) )
      sub_14355B230(a1);
    if ( !*(_BYTE *)(a1 + 23649) )
    {
      v4 = *(_QWORD *)(a1 + 23616);
      if ( v4 )
      {
        v5 = *(_BYTE *)(a1 + 23648) && (unsigned __int8)sub_146B34270(v4);
        v6 = sub_146E8C7D0(&unk_149E38AA0);
        v7 = sub_145A0F110(v6, 566);
        (*(void (__fastcall **)(__int64, _QWORD, bool, __int64))(*(_QWORD *)v7 + 136LL))(v7, 0, v5, 100);
      }
    }
    v8 = qword_14E659EA8;
    if ( !qword_14E659EA8 )
    {
      v9 = sub_146E8BA20(496);
      if ( v9 )
        v10 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v9);
      else
        v10 = 0;
      qword_14E659EA8 = (__int64)v10;
      (**v10)(v10);
      v8 = qword_14E659EA8;
    }
    LOBYTE(v3) = !*(_BYTE *)(a1 + 23648) || !(unsigned __int8)sub_146B34270(*(_QWORD *)(a1 + 23616));
    sub_14449EE00(v8, v3);
    if ( *(_BYTE *)(a1 + 23648) && (unsigned __int8)sub_146B34270(*(_QWORD *)(a1 + 23616)) )
    {
      v11 = sub_146EC2360(qword_14F1C0F28, 17);
      LOBYTE(v12) = 1;
      if ( v11 )
      {
        if ( !(unsigned __int8)sub_146682140(qword_14E683C78, 2996, v12) )
          sub_14668C520(qword_14E683C78, 2996, 0, 0);
      }
      else if ( (unsigned __int8)sub_146682140(qword_14E683C78, 2996, v12) )
      {
        sub_146694510(qword_14E683C78, 2996, -1, 0, 1);
      }
    }
    if ( !*(_BYTE *)(a1 + 23650) && (int)sub_146E9F840(a1 + 24128) > 7000 )
    {
      v13 = sub_145EFAFB0();
      if ( !sub_145CE4330(v13) )
      {
        v15 = sub_146D74000(v14);
        sub_146D746E0(v15, 42);
        sub_146D75AF0(v17, v16);
        sub_146E9FBC0(a1 + 24128);
      }
    }
    return sub_144D2E4B0(a1);
  }
  return result;
}


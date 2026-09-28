// sender_1551_sub_140D93550

__int64 __fastcall sub_140D93550(__int64 a1, __int64 a2, __int64 a3)
{
  __int64 result; // rax
  unsigned __int16 v5; // ax
  int v6; // ebx
  unsigned __int16 v7; // di
  __int64 v8; // rcx
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rsi
  __int64 v12; // rdx
  unsigned int v13; // r8d
  __int64 v14; // rcx
  __int64 v15; // rax
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rdx
  __int64 v21; // rcx
  void *retaddr; // [rsp+28h] [rbp+0h]
  unsigned int v23; // [rsp+48h] [rbp+20h] BYREF

  result = sub_146C82A50(a1);
  if ( result )
  {
    result = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)result + 1392LL))(result);
    if ( (_BYTE)result )
    {
      v5 = sub_147681E30(a3 + 24);
      v6 = *(_DWORD *)(a3 + 184);
      v7 = v5;
      v9 = sub_146D74000(v8);
      sub_146D746E0(v9, 713);
      v11 = sub_146D74000(v10);
      sub_146E920A0(&dword_14EF2CA00, &v23);
      v12 = v23;
      v13 = v23 + dword_14EF2CA00 + 196;
      if ( dword_14EF2CA04 && v13 && dword_14EF2CA04 != v13 )
      {
        if ( retaddr )
        {
          sub_146D89B40(retaddr, &dword_14EF2CA00);
          v12 = v23;
        }
      }
      sub_146D75CE0(v11, v12);
      v15 = sub_146D74000(v14);
      sub_146D75CE0(v15, 1);
      v17 = sub_146D74000(v16);
      sub_146D75CE0(v17, v6 != 1);
      v19 = sub_146D74000(v18);
      sub_146D75CE0(v19, v7);
      return sub_146D75AF0(v21, v20);
    }
  }
  return result;
}


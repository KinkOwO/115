// singleton_user_sub_1444A1E30

__int64 __fastcall sub_1444A1E30(__int64 a1, unsigned int a2)
{
  __int64 v4; // rcx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx

  sub_146E9FBC0(a1 + 536);
  v4 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v5 = sub_146E8BA20(496);
    if ( v5 )
      v6 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v5);
    else
      v6 = 0;
    qword_14E659EA8 = (__int64)v6;
    (**v6)(v6);
    v4 = qword_14E659EA8;
  }
  sub_14449D8B0(v4);
  sub_146E9FBD0(a1 + 536, 0, 0);
  return sub_146CE2560(a1, a2);
}


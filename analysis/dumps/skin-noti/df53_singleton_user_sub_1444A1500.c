// singleton_user_sub_1444A1500

__int64 __fastcall sub_1444A1500(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx

  v2 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v3 = sub_146E8BA20(496);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v3);
    else
      v4 = 0;
    qword_14E659EA8 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E659EA8;
  }
  sub_14449EA40(v2);
  if ( (int)sub_146E9F840(a1 + 536) > 5000 )
    sub_146E9FBD0(a1 + 536, 0, 0);
  return sub_146CE0470(a1);
}


// singleton_user_sub_1453A4D40

void sub_1453A4D40()
{
  void (__fastcall ***v0)(_QWORD); // rbx
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rcx
  __int64 v11; // rax
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rax

  sub_146694510(qword_14E683C78, 3102, -1, 0, 1);
  v0 = 0;
  if ( !qword_14E64EC28 )
  {
    v1 = sub_146E8BA20(112);
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_1430621F0(v1);
    else
      v2 = 0;
    qword_14E64EC28 = (__int64)v2;
    (**v2)(v2);
  }
  sub_143062CC0();
  if ( !qword_14E63AC10 )
  {
    v3 = sub_146E8BA20(112);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1431B2A20(v3);
    else
      v4 = 0;
    qword_14E63AC10 = (__int64)v4;
    (**v4)(v4);
  }
  sub_1431B3A40();
  if ( !qword_14E6344F8 )
  {
    v5 = sub_146E8BA20(1800);
    if ( v5 )
      v6 = (void (__fastcall ***)(_QWORD))sub_142F1DEF0(v5);
    else
      v6 = 0;
    qword_14E6344F8 = (__int64)v6;
    (**v6)(v6);
  }
  sub_142F2BEF0();
  if ( !qword_14E638CC8 )
  {
    v7 = sub_146E8BA20(2688);
    if ( v7 )
      v8 = sub_141B56770(v7);
    else
      v8 = 0;
    qword_14E638CC8 = v8;
    (**(void (__fastcall ***)(__int64))(v8 + 80))(v8 + 80);
  }
  sub_141B59830();
  if ( (unsigned __int8)sub_1459ACF00(qword_14E66C090) )
  {
    if ( !qword_14E659EA8 )
    {
      v9 = sub_146E8BA20(496);
      if ( v9 )
        v0 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v9);
      qword_14E659EA8 = (__int64)v0;
      (**v0)(v0);
    }
    nullsub_1();
  }
  else if ( sub_14014DEB0() )
  {
    v11 = sub_144F286D0();
    sub_14355D470(v11);
  }
  else
  {
    v12 = sub_142EEA7F0(v10);
    if ( !(unsigned __int8)sub_142F2AF30(v12) && !(unsigned __int8)sub_1459AB260(qword_14E66C090) )
    {
      if ( (unsigned __int8)sub_141062A00() )
      {
        v13 = sub_14667BB90(qword_14E683C78, 3693, 0);
        if ( v13 )
        {
          sub_14161E680(v13, 0);
          sub_14668C520(qword_14E683C78, 3697, 0, 0);
        }
      }
      else if ( (unsigned __int8)sub_1459AB040(qword_14E66C090) )
      {
        v14 = sub_14216B990();
        sub_142167120(v14);
      }
      else if ( (unsigned __int8)sub_1459AB010(qword_14E66C090) )
      {
        v15 = sub_1417E7EA0();
        sub_141D4C430(v15);
      }
      else if ( (unsigned __int8)sub_142ABB140(qword_14E683C40) )
      {
        sub_142ABDCA0(qword_14E683C40);
      }
      else if ( (unsigned __int8)sub_1459AAFD0(qword_14E66C090) )
      {
        v17 = sub_140BEDC40(v16);
        sub_140BEFF10(v17);
      }
      else if ( (unsigned __int8)sub_144CF4250(qword_14E683C30) == 1 )
      {
        sub_144CF8300(qword_14E683C30);
      }
      else
      {
        v18 = sub_143179A40();
        sub_1453A14F0(v18);
      }
    }
  }
}


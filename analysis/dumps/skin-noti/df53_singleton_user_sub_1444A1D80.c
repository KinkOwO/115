// singleton_user_sub_1444A1D80

void sub_1444A1D80()
{
  __int64 v0; // rax
  void (__fastcall ***v1)(_QWORD); // rcx

  nullsub_1();
  if ( !qword_14E659EA8 )
  {
    v0 = sub_146E8BA20(496);
    if ( v0 )
      v1 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v0);
    else
      v1 = 0;
    qword_14E659EA8 = (__int64)v1;
    (**v1)(v1);
  }
  nullsub_1();
}


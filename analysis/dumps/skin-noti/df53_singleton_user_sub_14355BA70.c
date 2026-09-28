// singleton_user_sub_14355BA70

__int64 sub_14355BA70()
{
  __int64 result; // rax
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx

  result = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v1 = sub_146E8BA20(496);
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v1);
    else
      v2 = 0;
    qword_14E659EA8 = (__int64)v2;
    (**v2)(v2);
    return qword_14E659EA8;
  }
  return result;
}


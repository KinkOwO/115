// fn_getmgr_0x140764510

__int64 sub_140764510()
{
  __int64 result; // rax
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx

  result = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v1 = sub_146E8BA20(1472);
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v1);
    else
      v2 = 0;
    qword_14E638F28 = (__int64)v2;
    (**v2)(v2);
    return qword_14E638F28;
  }
  return result;
}


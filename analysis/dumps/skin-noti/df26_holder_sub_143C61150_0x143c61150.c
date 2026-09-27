// holder_sub_143C61150_0x143c61150

__int64 sub_143C61150()
{
  __int64 result; // rax
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx

  result = qword_14E63AE60;
  if ( !qword_14E63AE60 )
  {
    v1 = sub_146E8BA20(336);
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_1447E41D0(v1);
    else
      v2 = 0;
    qword_14E63AE60 = (__int64)v2;
    (**v2)(v2);
    return qword_14E63AE60;
  }
  return result;
}


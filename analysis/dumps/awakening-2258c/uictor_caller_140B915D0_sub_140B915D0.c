// sub_140B915D0  va=0x140B915D0  size=66

__int64 sub_140B915D0()
{
  __int64 result; // rax
  __int64 v1; // [rsp+40h] [rbp+8h]

  result = qword_14E6399A0;
  if ( qword_14E6399A0 == 0 )
  {
    result = sub_146E8BA20(1360);
    v1 = result;
    __wind
    {
      if ( result != 0 )
        result = sub_140B8D280(result);
    }
    __unwind
    {
      j_j_scalable_free(v1, 1360);
    }
    qword_14E6399A0 = result;
  }
  return result;
}

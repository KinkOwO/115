// reader_callee_sub_1401DBB80

double __fastcall sub_1401DBB80(__int64 a1, __int64 a2, __int64 *a3)
{
  __int64 *i; // rbx
  __int64 *v6; // rcx
  double result; // xmm0_8

  for ( i = a3; !*((_BYTE *)i + 25); result = sub_146E9F3A0(v6, 56) )
  {
    sub_1401DBB80(a1, a2, i[2]);
    v6 = i;
    i = (__int64 *)*i;
  }
  return result;
}


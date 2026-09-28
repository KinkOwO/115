// reader_callee_sub_146EA1920

__int64 __fastcall sub_146EA1920(_WORD *a1)
{
  __int64 v1; // rdx

  if ( dword_14F1BF878 < 2 )
  {
    *a1 = 0;
  }
  else
  {
    v1 = qword_14F1BF870 + 2;
    *a1 = *(_WORD *)qword_14F1BF870;
    qword_14F1BF870 = v1;
  }
  return sub_146EA0340(2);
}


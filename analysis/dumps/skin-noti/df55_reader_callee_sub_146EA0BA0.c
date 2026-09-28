// reader_callee_sub_146EA0BA0

__int64 __fastcall sub_146EA0BA0(_DWORD *a1)
{
  __int64 v1; // rdx

  if ( dword_14F1BF878 < 4 )
  {
    *a1 = 0;
  }
  else
  {
    v1 = qword_14F1BF870 + 4;
    *a1 = *(_DWORD *)qword_14F1BF870;
    qword_14F1BF870 = v1;
  }
  return sub_146EA0340(4);
}


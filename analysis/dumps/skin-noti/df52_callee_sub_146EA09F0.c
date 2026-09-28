// callee_sub_146EA09F0

__int64 __fastcall sub_146EA09F0(_BYTE *a1)
{
  __int64 v1; // rdx

  if ( dword_14F1BF878 < 1 )
  {
    *a1 = 0;
  }
  else
  {
    v1 = qword_14F1BF870 + 1;
    *a1 = *(_BYTE *)qword_14F1BF870;
    qword_14F1BF870 = v1;
  }
  return sub_146EA0340(1);
}


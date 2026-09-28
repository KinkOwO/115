// reader_callee_sub_146EA0BE0

__int64 __fastcall sub_146EA0BE0(__int64 a1, int a2)
{
  __int64 v2; // rsi
  __int64 v3; // rbx

  v2 = (unsigned int)a2;
  if ( a2 > 0 )
  {
    if ( dword_14F1BF878 >= a2 )
    {
      v3 = qword_14F1BF870;
      sub_148AA1E60(a1, qword_14F1BF870, (unsigned int)a2);
      qword_14F1BF870 = v2 + v3;
      return sub_146EA0340((unsigned int)v2);
    }
    MEMORY[0] = 0;
  }
  return sub_146EA0340((unsigned int)a2);
}


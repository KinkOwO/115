// reader_callee_sub_146A19E60

unsigned __int64 __fastcall sub_146A19E60(int a1, int a2)
{
  unsigned __int64 result; // rax

  result = (unsigned __int64)((unsigned __int128)((qword_14E6A7C70 - qword_14E6A7C68) * (__int128)0x2AAAAAAAAAAAAAABLL) >> 64) >> 63;
  if ( (qword_14E6A7C70 - qword_14E6A7C68) / 96 > (unsigned __int64)a1 )
  {
    result = 96LL * a1;
    *(_DWORD *)(result + qword_14E6A7C68 + 8) = a2;
  }
  return result;
}


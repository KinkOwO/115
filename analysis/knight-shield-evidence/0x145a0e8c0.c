__int64 __fastcall sub_145A0E8C0(int a1, __int64 a2, __int64 a3)
{
  __int64 result; // rax

  if ( a1 != 0 && a1 != 28 ) /*0x145a0e8cb*/
  {
    switch ( a1 ) /*0x145a0e8d4*/
    {
      case 1: /*0x145a0e8d4*/
        return qword_14E683CB0; /*0x145a0e8dd*/
      case 2: /*0x145a0e8d4*/
        return qword_14E683C88; /*0x145a0e8ea*/
      case 45: /*0x145a0e8d4*/
        return qword_14E683CA0; /*0x145a0e8f7*/
      case 12: /*0x145a0e8d4*/
        return qword_14E683C90; /*0x145a0e904*/
      case 4: /*0x145a0e8d4*/
        return sub_145AFABE0(a1: qword_14E683D38, a2: 0, a3); /*0x145a0e913*/
      case 7: /*0x145a0e8d4*/
        return qword_14E683CB8; /*0x145a0e924*/
      case 43: /*0x145a0e8d4*/
        return qword_14E683CC0; /*0x145a0e931*/
      case 11: /*0x145a0e8d4*/
        return qword_14E683CA8; /*0x145a0e93e*/
      default:
        break; /*0x145a0e945*/
    }
    if ( (unsigned int)(a1 - 18) <= 4 ) /*0x145a0e945*/
      return qword_14E683CE0[a1 - 18]; /*0x145a0e98f*/
    if ( a1 == 30 ) /*0x145a0e94a*/
      return sub_144FFA460(); /*0x145a0e94a*/
    if ( a1 == 33 ) /*0x145a0e953*/
      return qword_14E683CC8; /*0x145a0e95c*/
    if ( a1 != 35 ) /*0x145a0e960*/
    {
      if ( a1 == 42 ) /*0x145a0e965*/
        return qword_14E683CD8; /*0x145a0e967*/
      result = 0; /*0x145a0e96f*/
      if ( a1 == 13 ) /*0x145a0e974*/
        return qword_14E683B00; /*0x145a0e974*/
      return result; /*0x145a0e8dd*/
    }
  }
  return qword_14E683C80; /*0x145a0e997*/
}

__int64 __fastcall sub_1444EC790(unsigned __int16 a1)
{
  unsigned int v1; // edx
  __int64 result; // rax
  __int64 v3; // rax

  v1 = 100002243;
  result = a1 - 1;
  switch ( a1 )
  {
    case 1u:
      v1 = 100002249;
      break;
    case 3u:
      v1 = 100002244;
      break;
    case 4u:
      v1 = 100002256;
      break;
    case 0x11u:
      v1 = 100002245;
      break;
    case 0x13u:
      v1 = 100002247;
      break;
    case 0x14u:
      v1 = 100002248;
      break;
    case 0x15u:
      v1 = 100002246;
      break;
    case 0x9Fu:
      v1 = 100086040;
      break;
    case 0xCCu:
      v1 = 100007185;
      break;
    default:
      result = (unsigned int)a1 - 1;
      break;
  }
  if ( qword_14E683C78 )
  {
    v3 = sub_14723C170(v1);
    return sub_14668C520(qword_14E683C78, 2875, v3, 0);
  }
  return result;
}

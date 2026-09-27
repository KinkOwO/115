__int64 __fastcall sub_144079890(__int64 a1, char a2, unsigned __int16 a3)
{
  int v3; // eax
  __int64 result; // rax
  __int64 v5; // rcx
  __int64 v6; // rax

  if ( a2 )
  {
    v3 = sub_14667EB40(qword_14E683C78, 3200);
    result = sub_148AA307C(v3, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DDAAC80, 0);
    if ( result )
      return sub_14407E020(result);
  }
  else
  {
    result = a3 - 1;
    switch ( a3 )
    {
      case 1u:
        if ( qword_14E683C78 )
        {
          v5 = 60818;
          goto LABEL_22;
        }
        break;
      case 3u:
      case 0x15u:
      case 0x3Cu:
      case 0x65u:
      case 0x68u:
      case 0x72u:
      case 0xD6u:
        if ( qword_14E683C78 )
        {
          v5 = 100007138;
          goto LABEL_22;
        }
        break;
      case 0x14u:
        if ( qword_14E683C78 )
        {
          v5 = 100007188;
          goto LABEL_22;
        }
        break;
      case 0x5Du:
      case 0xD4u:
        if ( qword_14E683C78 )
        {
          v5 = 100007182;
          goto LABEL_22;
        }
        break;
      case 0x6Au:
        if ( qword_14E683C78 )
        {
          v5 = 100007149;
          goto LABEL_22;
        }
        break;
      case 0x74u:
        if ( qword_14E683C78 )
        {
          v5 = 100007186;
          goto LABEL_22;
        }
        break;
      case 0xCCu:
        if ( qword_14E683C78 )
        {
          v5 = 100007187;
          goto LABEL_22;
        }
        break;
      default:
        result = sub_146ADFC80(a3, 0, 1, 1, 1, 1);
        if ( !(_BYTE)result && qword_14E683C78 )
        {
          v5 = 100007092;
LABEL_22:
          v6 = sub_14723C170(v5);
          result = sub_14668C520(qword_14E683C78, 2875, v6, 0);
        }
        break;
    }
  }
  return result;
}

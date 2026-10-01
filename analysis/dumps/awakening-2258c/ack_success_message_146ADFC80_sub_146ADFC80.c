// sub_146ADFC80  va=0x146ADFC80  size=729

char __fastcall sub_146ADFC80(__int64 a1, int a2, __int64 a3, __int64 a4, int a5, char a6)
{
  __int64 v6; // rcx
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rax
  __int64 v12; // [rsp+28h] [rbp-10h]

  switch ( (_DWORD)a1 )
  {
    case 0x72:
      v6 = 7520;
LABEL_59:
      v11 = sub_14723C170(v6);
      sub_14668C520(*(_QWORD *)&qword_14E683C78, 2875, v11, 0);
      return 1;
    case 0x73:
      v6 = 7521;
      goto LABEL_59;
    case 0xE5:
      sub_14668C520(*(_QWORD *)&qword_14E683C78, 3634, 0, 2);
      return 1;
    case 0xE6:
      if ( *(_BYTE *)(sub_1451C94C0(a1) + 16) == 1 )
      {
        sub_14668C520(*(_QWORD *)&qword_14E683C78, 3634, 0, 4);
        return 1;
      }
      v6 = 60332;
      goto LABEL_59;
    case 0x7B:
      return 1;
    case 0x7A:
      sub_14668C520(*(_QWORD *)&qword_14E683C78, 3634, 0, 0);
      return 1;
    case 0x89:
      v6 = 13006;
      goto LABEL_59;
    case 0x88:
      sub_14668C520(*(_QWORD *)&qword_14E683C78, 3634, 0, 1);
      return 1;
    case 0xD0:
      v6 = 60218;
      goto LABEL_59;
    case 0x7C:
      v6 = 60292;
      goto LABEL_59;
    case 0x8A:
      v6 = 60293;
      goto LABEL_59;
    case 0xEF:
      v6 = 60298;
      goto LABEL_59;
    case 0xED:
      v6 = 60299;
      goto LABEL_59;
    case 0xA7:
      v6 = 60257;
      goto LABEL_59;
    case 0xEA:
      v6 = 60253;
      goto LABEL_59;
    case 0x4D:
      v6 = 100007046;
      goto LABEL_59;
    case 0x78:
      if ( (unsigned int)sub_1459A90F0(qword_14E66C090) == 0 || (unsigned int)sub_1459A90F0(qword_14E66C090) == 1 )
      {
        v8 = qword_14E634428;
        if ( qword_14E634428 == 0 )
        {
          v9 = sub_146E8BA20(328);
          v12 = v9;
          __wind
          {
            if ( v9 != 0 )
              v10 = (void (__fastcall ***)(_QWORD))sub_144ECF5F0(v9);
            else
              v10 = nullptr;
          }
          __unwind
          {
            j_j_scalable_free(v12, 328);
          }
          qword_14E634428 = (__int64)v10;
          (**v10)(v10);
          v8 = qword_14E634428;
        }
        if ( (unsigned __int8)sub_144ED08F0(v8, 1) != 0 )
        {
          sub_14668C520(*(_QWORD *)&qword_14E683C78, 1222, 0, 1);
          v6 = 60307;
          goto LABEL_59;
        }
        sub_14668C520(*(_QWORD *)&qword_14E683C78, 1222, 0, 0);
      }
      v6 = 60307;
      goto LABEL_59;
    case 0xD6:
      if ( a2 == 6 )
        v6 = 42232;
      else
        v6 = 400003322;
      goto LABEL_59;
    case 0x7E:
      sub_14668C520(*(_QWORD *)&qword_14E683C78, 3634, 0, 5);
      return 1;
    case 0x36:
      if ( a6 != 0 )
      {
        if ( *(_QWORD *)&qword_14E683C78 != 0 )
        {
          v6 = 100007041;
          goto LABEL_59;
        }
      }
      else if ( *(_QWORD *)&qword_14E683C78 != 0 )
      {
        v6 = 100007056;
        goto LABEL_59;
      }
      return 1;
    case 0x7DE:
      if ( *(_QWORD *)&qword_14E683C78 == 0 )
        return 1;
      v6 = 400001683;
      goto LABEL_59;
    default:
      break;
  }
  return 0;
}

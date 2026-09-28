// loadfn_0x146d7c710

__int64 sub_146D7C710()
{
  int v0; // esi
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx
  int v3; // edi
  int i; // ebx
  __int64 v5; // rbx
  int v6; // edi
  int v7; // eax
  __int64 result; // rax
  int v9; // r14d
  __int64 v10; // rbx
  __int64 v11; // kr00_8
  int v12; // ebp
  int v13; // r15d
  __int64 v14; // rax
  void (__fastcall ***v15)(_QWORD); // rcx
  __int64 v16; // rbx
  int v17; // edi
  int v18; // eax

  v0 = qword_14E6343D0;
  if ( !qword_14E6343D0 )
  {
    v1 = sub_146E8BA20(72);
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_146E93360(v1);
    else
      v2 = 0;
    qword_14E6343D0 = (__int64)v2;
    (**v2)(v2);
    v0 = qword_14E6343D0;
  }
  v3 = 0;
  for ( i = 0; i < 65; ++i )
    v3 += sub_146D7BDC0(i, 1);
  v5 = sub_146E8C7D0(&unk_14B170B00);
  v6 = sub_146E8C7D0(&unk_14B170B40);
  v7 = sub_146E8C7D0(&unk_14B170BA0);
  result = sub_146E938E0(v0, 3, v7, v6, 841, (__int64)&qword_14F0E9EC8, v5);
  v9 = 0;
  v10 = qword_14E683B18;
  do
  {
    if ( v10 )
    {
      v11 = result;
      result = v9;
      switch ( v9 )
      {
        case 0:
          v12 = *(_DWORD *)(v10 + 552);
          goto LABEL_41;
        case 1:
          result = qword_14E683B20;
          goto LABEL_40;
        case 2:
          result = qword_14E683B28;
          goto LABEL_40;
        case 3:
          result = qword_14E683B30;
          goto LABEL_40;
        case 4:
          result = qword_14E683B38;
          goto LABEL_40;
        case 5:
          result = qword_14E683B40;
          goto LABEL_40;
        case 6:
          result = qword_14E683B48;
          goto LABEL_40;
        case 7:
          result = qword_14E683B50;
          goto LABEL_40;
        case 8:
          result = qword_14E683B58;
          goto LABEL_40;
        case 9:
          result = qword_14E683B60;
          goto LABEL_40;
        case 10:
          result = qword_14E683B68;
          goto LABEL_40;
        case 11:
          result = qword_14E683B70;
          goto LABEL_40;
        case 12:
          result = qword_14E683B78;
          goto LABEL_40;
        case 13:
          result = qword_14E683B80;
          goto LABEL_40;
        case 14:
          result = qword_14E683B88;
          goto LABEL_40;
        case 15:
          result = qword_14E683B90;
          goto LABEL_40;
        case 18:
          result = qword_14E683B98;
          goto LABEL_40;
        case 19:
          result = qword_14E683BA0;
          goto LABEL_40;
        case 20:
          result = qword_14E683BA8;
          goto LABEL_40;
        case 21:
          result = qword_14E683BB0;
          goto LABEL_40;
        case 39:
          result = qword_14E683BB8;
          goto LABEL_40;
        case 41:
          result = qword_14E683BC0;
          goto LABEL_40;
        case 42:
          result = qword_14E683BC8;
          goto LABEL_40;
        case 43:
          result = qword_14E683BD0;
          goto LABEL_40;
        case 57:
          result = qword_14E683BD8;
          goto LABEL_40;
        case 61:
          result = qword_14E683BE0;
          goto LABEL_40;
        case 62:
          result = qword_14E683BE8;
          goto LABEL_40;
        case 63:
          result = qword_14E683BF0;
          goto LABEL_40;
        case 64:
          result = qword_14E683BF8;
LABEL_40:
          v12 = *(_DWORD *)(result + 552);
LABEL_41:
          if ( v12 > 10 )
          {
            sub_146D7BDC0(v9, 1);
            v13 = qword_14E6343D0;
            if ( !qword_14E6343D0 )
            {
              v14 = sub_146E8BA20(72);
              if ( v14 )
                v15 = (void (__fastcall ***)(_QWORD))sub_146E93360(v14);
              else
                v15 = 0;
              qword_14E6343D0 = (__int64)v15;
              (**v15)(v15);
              v13 = qword_14E6343D0;
              v10 = qword_14E683B18;
            }
            if ( v10 )
            {
              result = v9;
              switch ( v9 )
              {
                case 0:
                case 1:
                case 2:
                case 3:
                case 4:
                case 5:
                case 6:
                case 7:
                case 8:
                case 9:
                case 10:
                case 11:
                case 12:
                case 13:
                case 14:
                case 15:
                case 16:
                case 17:
                case 18:
                case 19:
                case 20:
                case 21:
                case 22:
                case 23:
                case 24:
                case 25:
                case 26:
                case 27:
                case 28:
                case 29:
                case 30:
                case 31:
                case 32:
                case 33:
                case 34:
                case 35:
                case 36:
                case 37:
                case 38:
                case 39:
                case 40:
                case 41:
                case 42:
                case 43:
                case 44:
                case 45:
                case 46:
                case 47:
                case 48:
                case 49:
                case 50:
                case 51:
                case 52:
                case 53:
                case 54:
                case 55:
                case 56:
                case 57:
                case 58:
                case 59:
                case 60:
                case 61:
                case 62:
                case 63:
                case 64:
                  goto LABEL_49;
                default:
                  goto LABEL_50;
              }
            }
            else
            {
LABEL_49:
              v16 = sub_146E8C7D0(&unk_14B170C20);
              v17 = sub_146E8C7D0(&unk_14B170B40);
              v18 = sub_146E8C7D0(&unk_14B170BA0);
              result = sub_146E938E0(v13, 3, v18, v17, 850, (__int64)&qword_14F0E9EC8, v16);
              v10 = qword_14E683B18;
            }
          }
          break;
        default:
          result = v11;
          break;
      }
    }
LABEL_50:
    ++v9;
  }
  while ( v9 < 65 );
  return result;
}


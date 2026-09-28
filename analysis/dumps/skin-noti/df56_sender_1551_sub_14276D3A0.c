// sender_1551_sub_14276D3A0

__int16 *sub_14276D3A0()
{
  __int64 v0; // rcx
  __int64 v1; // rax
  void (__fastcall ***v2)(_QWORD); // rcx
  __int16 *result; // rax
  __int64 v4; // rax
  int v5; // ebx
  __int64 v6; // rax
  __int64 v7; // rcx
  __int64 v8; // rax
  _QWORD *v9; // rdi
  unsigned __int64 v10; // rsi
  __int128 *v11; // r14
  __int64 v12; // rbx
  __int128 *v13; // r8
  int v14; // eax
  unsigned __int64 v15; // rdx
  __int64 v16; // rcx
  __int64 v17; // rax
  __int64 v18; // rdx
  __int64 v19; // rcx
  _QWORD v20[3]; // [rsp+30h] [rbp-29h] BYREF
  __int128 v21; // [rsp+48h] [rbp-11h] BYREF
  unsigned __int64 v22; // [rsp+58h] [rbp-1h]
  unsigned __int64 v23; // [rsp+60h] [rbp+7h]
  _BYTE v24[8]; // [rsp+68h] [rbp+Fh] BYREF
  _QWORD v25[2]; // [rsp+70h] [rbp+17h] BYREF
  unsigned __int64 v26; // [rsp+80h] [rbp+27h]
  unsigned __int64 v27; // [rsp+88h] [rbp+2Fh]

  v20[1] = -2;
  v0 = qword_14E63B028;
  if ( !qword_14E63B028 )
  {
    v1 = sub_146E8BA20(296);
    v20[0] = v1;
    if ( v1 )
      v2 = (void (__fastcall ***)(_QWORD))sub_144AC9DD0(v1);
    else
      v2 = 0;
    qword_14E63B028 = (__int64)v2;
    (**v2)(v2);
    v0 = qword_14E63B028;
  }
  result = (__int16 *)sub_144ACF290(v0, 125);
  if ( (_BYTE)result )
  {
    result = (__int16 *)sub_145F0B890();
    if ( result )
    {
      v4 = sub_145F0B890();
      result = (__int16 *)sub_142E0A350(v4);
      v5 = *result;
      if ( v5 != -1 )
      {
        sub_144AC9C10(v20);
        LODWORD(v20[0]) = 125;
        HIDWORD(v20[0]) = v5;
        v6 = sub_141608660();
        sub_144ACDB80(v6, v24, v20);
        if ( v24[3] )
        {
          v8 = sub_146E8BA20(264);
          v9 = (_QWORD *)v8;
          v20[2] = v8;
          if ( v8 )
          {
            sub_148AA2510(v8, 0, 264);
            sub_1467CB160(v9);
            *v9 = off_149ABBA18;
          }
          else
          {
            v9 = 0;
          }
          sub_1467CEDC0(v9);
          sub_1406970A0(v9, 1);
          sub_1467CEDC0(v9);
          *(_QWORD *)&v21 = 0;
          v22 = 0;
          v23 = 0;
          v10 = v26;
          v11 = (__int128 *)v25;
          if ( v27 >= 8 )
            v11 = (__int128 *)v25[0];
          if ( v26 >= 8 )
          {
            v12 = v26 | 7;
            if ( (v26 | 7) > 0x7FFFFFFFFFFFFFFELL )
              v12 = 0x7FFFFFFFFFFFFFFELL;
            *(_QWORD *)&v21 = sub_14014CB50(&v21, v12 + 1);
            sub_148AA1E60(v21, v11, 2 * v10 + 2);
            v23 = v12;
          }
          else
          {
            v21 = *v11;
            v23 = 7;
          }
          v22 = v10;
          v13 = &v21;
          if ( v23 >= 8 )
            v13 = (__int128 *)v21;
          v14 = sub_14668C520(qword_14E683C78, 2476, v13, v9);
          sub_148AA307C(v14, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD95438, 0);
          if ( v23 >= 8 )
          {
            v15 = 2 * v23 + 2;
            v16 = v21;
            if ( v15 >= 0x1000 )
            {
              v15 = 2 * v23 + 41;
              v16 = *(_QWORD *)(v21 - 8);
              if ( (unsigned __int64)(v21 - v16 - 8) > 0x1F )
                sub_148AAF304(v16, v15);
            }
            sub_146E9F3A0(v16, v15);
          }
          v22 = 0;
          v23 = 7;
          LOWORD(v21) = 0;
        }
        else
        {
          v17 = sub_146D74000(v7);
          sub_146D746E0(v17, 2008);
          sub_146D75AF0(v19, v18);
        }
        return (__int16 *)sub_14014C710(v25);
      }
    }
  }
  return result;
}


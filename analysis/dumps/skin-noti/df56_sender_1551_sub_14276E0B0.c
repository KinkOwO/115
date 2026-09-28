// sender_1551_sub_14276E0B0

void __fastcall sub_14276E0B0(__int64 a1)
{
  __int64 v2; // rbx
  __int64 v3; // rax
  __int64 v4; // rdi
  unsigned int v5; // ebx
  __int64 v6; // rax
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // r8
  __int64 v12; // rax
  int v13; // ebx
  __int64 v14; // rax
  __int64 v15; // rcx
  __int64 v16; // rax
  _QWORD *v17; // rdi
  __int64 v18; // rax
  __int64 v19; // r8
  _QWORD *v20; // r8
  unsigned __int64 v21; // rdx
  __int64 v22; // rcx
  __int64 v23; // r8
  unsigned __int64 v24; // rsi
  __int128 *v25; // r14
  __int64 v26; // rbx
  __int128 *v27; // r8
  int v28; // eax
  unsigned __int64 v29; // rdx
  __int64 v30; // rcx
  __int64 v31; // rax
  __int64 v32; // rdx
  __int64 v33; // rcx
  __int64 v34; // rax
  __int64 v35; // rdx
  _DWORD v36[2]; // [rsp+38h] [rbp-49h] BYREF
  __int64 v37; // [rsp+40h] [rbp-41h]
  __int64 v38; // [rsp+48h] [rbp-39h]
  __int128 v39; // [rsp+50h] [rbp-31h] BYREF
  unsigned __int64 v40; // [rsp+60h] [rbp-21h]
  unsigned __int64 v41; // [rsp+68h] [rbp-19h]
  _QWORD v42[2]; // [rsp+70h] [rbp-11h] BYREF
  __int64 v43; // [rsp+80h] [rbp-1h]
  unsigned __int64 v44; // [rsp+88h] [rbp+7h]
  _BYTE v45[8]; // [rsp+90h] [rbp+Fh] BYREF
  _QWORD v46[2]; // [rsp+98h] [rbp+17h] BYREF
  unsigned __int64 v47; // [rsp+A8h] [rbp+27h]
  unsigned __int64 v48; // [rsp+B0h] [rbp+2Fh]

  v37 = -2;
  v2 = sub_141308BD0(qword_14E66C090);
  if ( !v2
    || !sub_145F0B890()
    || (unsigned int)sub_141C4A150(v2) != *(_DWORD *)(a1 + 672)
    || (unsigned int)sub_146D01AB0(v2) != *(_DWORD *)(a1 + 676) )
  {
    return;
  }
  v42[0] = 0;
  v43 = 0;
  v44 = 7;
  sub_141308BD0(qword_14E66C090);
  if ( !(unsigned __int8)sub_146D054A0(v42) )
    goto LABEL_25;
  if ( sub_145EFAFB0() )
  {
    v3 = sub_145EFAFB0();
    v4 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v3 + 304LL))(v3);
  }
  else
  {
    v4 = 0;
  }
  v5 = 0;
  v6 = sub_145F0B890();
  if ( (int)sub_1406B19B0(v6) > 0 )
  {
    while ( !(unsigned int)sub_14665B4E0(qword_14E683C68, v5) )
    {
      v7 = sub_145F13700(qword_14E683C20, v5);
      if ( v7 )
      {
        if ( v4 )
        {
          v8 = sub_145F01800(v7, 0);
          if ( !v8 || !(unsigned __int8)sub_145DFA6C0(v4, v8) )
          {
            v18 = sub_14723C170(725);
            v19 = -1;
            do
              ++v19;
            while ( *(_WORD *)(v18 + 2 * v19) );
            goto LABEL_24;
          }
        }
      }
      ++v5;
      v9 = sub_145F0B890();
      if ( (int)v5 >= (int)sub_1406B19B0(v9) )
        goto LABEL_16;
    }
    v18 = sub_14723C170(532);
    v23 = -1;
    do
      ++v23;
    while ( *(_WORD *)(v18 + 2 * v23) );
LABEL_24:
    sub_14014C8D0(v42, v18);
LABEL_25:
    if ( qword_14E683C78 )
    {
      v20 = v42;
      if ( v44 >= 8 )
        v20 = (_QWORD *)v42[0];
      sub_14668C520(qword_14E683C78, 2875, v20, 0);
    }
    goto LABEL_29;
  }
LABEL_16:
  v10 = sub_141608660();
  if ( !(unsigned __int8)sub_144ACF290(v10, 125) )
    goto LABEL_55;
  if ( sub_145F0B890() )
  {
    v12 = sub_145F0B890();
    v13 = *(__int16 *)sub_142E0A350(v12);
    if ( v13 != -1 )
    {
      sub_144AC9C10(v36);
      v36[0] = 125;
      v36[1] = v13;
      v14 = sub_141608660();
      sub_144ACDB80(v14, v45, v36);
      if ( v45[3] )
      {
        v16 = sub_146E8BA20(264);
        v17 = (_QWORD *)v16;
        v38 = v16;
        if ( v16 )
        {
          sub_148AA2510(v16, 0, 264);
          sub_1467CB160(v17);
          *v17 = off_149ABBA00;
        }
        else
        {
          v17 = 0;
        }
        sub_1467CEDC0(v17);
        sub_1406970A0(v17, 1);
        sub_1467CEDC0(v17);
        *(_QWORD *)&v39 = 0;
        v40 = 0;
        v41 = 0;
        v24 = v47;
        v25 = (__int128 *)v46;
        if ( v48 >= 8 )
          v25 = (__int128 *)v46[0];
        if ( v47 >= 8 )
        {
          v26 = v47 | 7;
          if ( (v47 | 7) > 0x7FFFFFFFFFFFFFFELL )
            v26 = 0x7FFFFFFFFFFFFFFELL;
          *(_QWORD *)&v39 = sub_14014CB50(&v39, v26 + 1);
          sub_148AA1E60(v39, v25, 2 * v24 + 2);
          v41 = v26;
        }
        else
        {
          v39 = *v25;
          v41 = 7;
        }
        v40 = v24;
        v27 = &v39;
        if ( v41 >= 8 )
          v27 = (__int128 *)v39;
        v28 = sub_14668C520(qword_14E683C78, 2476, v27, v17);
        sub_148AA307C(v28, 0, (unsigned int)&off_14DCB4760, (unsigned int)&off_14DD95438, 0);
        if ( v41 >= 8 )
        {
          v29 = 2 * v41 + 2;
          v30 = v39;
          if ( v29 >= 0x1000 )
          {
            v29 = 2 * v41 + 41;
            v30 = *(_QWORD *)(v39 - 8);
            if ( (unsigned __int64)(v39 - v30 - 8) > 0x1F )
              sub_148AAF304(v30, v29);
          }
          sub_146E9F3A0(v30, v29);
        }
        v40 = 0;
        v41 = 7;
        LOWORD(v39) = 0;
      }
      else
      {
        v31 = sub_146D74000(v15);
        sub_146D746E0(v31, 2008);
        sub_146D75AF0(v33, v32);
      }
      sub_14014C710(v46);
LABEL_55:
      LOBYTE(v11) = 1;
      if ( (unsigned __int8)sub_145F15380(qword_14E683C20, 0, v11) )
      {
        v34 = sub_141308BD0(qword_14E66C090);
        if ( v34 )
        {
          LOBYTE(v35) = 1;
          sub_1439E3F10(v34, v35);
        }
      }
    }
  }
LABEL_29:
  if ( v44 >= 8 )
  {
    v21 = 2 * v44 + 2;
    v22 = v42[0];
    if ( v21 >= 0x1000 )
    {
      v21 = 2 * v44 + 41;
      v22 = *(_QWORD *)(v42[0] - 8LL);
      if ( (unsigned __int64)(v42[0] - v22 - 8) > 0x1F )
        sub_148AAF304(v22, v21);
    }
    sub_146E9F3A0(v22, v21);
  }
  v43 = 0;
  v44 = 7;
  LOWORD(v42[0]) = 0;
}


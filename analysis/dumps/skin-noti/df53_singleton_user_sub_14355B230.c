// singleton_user_sub_14355B230

__int64 __fastcall sub_14355B230(__int64 a1)
{
  __int64 result; // rax
  int i; // edi
  __int64 v4; // rax
  __int64 v5; // rbx
  __int64 v6; // rax
  __int64 v7; // rdi
  __int64 v8; // rax
  int v9; // ebx
  __int64 v10; // rax
  int v11; // esi
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rax
  __int64 v15; // rsi
  int v16; // ebp
  unsigned int v17; // ebx
  __int64 v18; // rax
  __int64 v19; // rax
  __int64 v20; // rax
  __int64 v21; // rax
  int v22; // eax
  __int64 v23; // rax
  __int64 v24; // rax
  __int64 v25; // rbx
  __int64 v26; // rcx
  __int64 v27; // rax
  void (__fastcall ***v28)(_QWORD); // rcx
  unsigned int v29; // edi
  unsigned int v30; // edi
  unsigned int v31; // ebx
  __int64 v32; // rax
  __int64 v33; // rcx
  __int64 v34; // rax
  void (__fastcall ***v35)(_QWORD); // rcx
  __int64 v36; // rbx
  __int64 v37; // rdx
  __int64 v38; // [rsp+98h] [rbp+10h] BYREF
  __int64 *v39; // [rsp+A0h] [rbp+18h] BYREF
  unsigned int v40; // [rsp+A8h] [rbp+20h] BYREF

  result = sub_145EFAFB0();
  if ( result && !*(_BYTE *)(a1 + 23650) )
  {
    *(_BYTE *)(a1 + 23648) = 1;
    for ( i = 0; i < 8; ++i )
    {
      v4 = sub_145F13060(qword_14E683C20, (unsigned int)i);
      v5 = v4;
      if ( v4
        && !(*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)v4 + 2640LL))(v4)
        && (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v5 + 3008LL))(v5) > 0
        && !sub_145CE4330(v5) )
      {
        *(_BYTE *)(a1 + 23648) = 0;
        goto LABEL_11;
      }
    }
    if ( *(_BYTE *)(a1 + 23648) )
      *(_BYTE *)(a1 + 23650) = 1;
LABEL_11:
    v6 = sub_145EFAFB0();
    if ( sub_145CE4330(v6) )
    {
      return sub_146E9FBC0(a1 + 24128);
    }
    else
    {
      if ( (*(_DWORD *)(a1 + 24132) & 2) == 0 || (int)sub_146E9F840(a1 + 24128) <= 0 )
        sub_146E9FBD0(a1 + 24128, 0, 0);
      v7 = sub_1459A9240(qword_14E66C090);
      if ( v7 )
      {
        LODWORD(v38) = 0;
        v8 = sub_145EFAFB0();
        v9 = sub_145B8C7D0(v8);
        v10 = sub_145EFAFB0();
        v11 = sub_145B8C890(v10);
        v12 = sub_145EFAFB0();
        sub_144D26800(v7, (unsigned int)&v40, (unsigned int)&v39, (unsigned int)&v38, v12);
        if ( v9 != v40 && v11 != (_DWORD)v39 )
        {
          v13 = sub_145EFAFB0();
          (*(void (__fastcall **)(__int64, _QWORD, _QWORD, _QWORD))(*(_QWORD *)v13 + 432LL))(
            v13,
            v40,
            (unsigned int)v39,
            (unsigned int)v38);
        }
      }
      v14 = sub_145EFAFB0();
      result = sub_1450BE260(v14);
      v15 = result;
      if ( result )
      {
        v16 = 0;
        if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)result + 328LL))(result) )
        {
          if ( (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 3040LL))(v15) )
          {
            v17 = 0;
            v18 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 3040LL))(v15);
            if ( (int)sub_145F447B0(v18) > 0 )
            {
              while ( 1 )
              {
                v19 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 3040LL))(v15);
                v20 = sub_145F438A0(v19, v17);
                if ( sub_145F37660(v20) == v15 )
                {
                  v21 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 3040LL))(v15);
                  v22 = sub_145F438A0(v21, v17);
                  v23 = sub_148AA307C(v22, 0, (unsigned int)&off_14DCB4468, (unsigned int)&off_14DCC2DE0, 0);
                  if ( v23 )
                  {
                    if ( *(_BYTE *)(v23 + 292) )
                      break;
                  }
                }
                ++v17;
                v24 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 3040LL))(v15);
                if ( (int)v17 >= (int)sub_145F447B0(v24) )
                  goto LABEL_31;
              }
              v16 = 1;
            }
          }
        }
LABEL_31:
        v25 = sub_145F0BA60(qword_14E683C08);
        v26 = qword_14E659EA8;
        if ( !qword_14E659EA8 )
        {
          v27 = sub_146E8BA20(496);
          v38 = v27;
          if ( v27 )
            v28 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v27);
          else
            v28 = 0;
          qword_14E659EA8 = (__int64)v28;
          (**v28)(v28);
          v26 = qword_14E659EA8;
        }
        result = sub_14449DFB0(v26);
        v29 = result;
        if ( v25 )
        {
          v30 = sub_1406FE620(v25);
          v31 = sub_140BA9880(v25);
          v32 = sub_14355BA70();
          result = sub_14449DFC0(v32, v30, v31);
          v29 = result;
        }
        if ( !v16 )
        {
          v33 = qword_14E634220;
          if ( !qword_14E634220 )
          {
            v34 = sub_146E8BA20(64);
            v38 = v34;
            if ( v34 )
              v35 = (void (__fastcall ***)(_QWORD))sub_144B6DC50(v34);
            else
              v35 = 0;
            qword_14E634220 = (__int64)v35;
            (**v35)(v35);
            v33 = qword_14E634220;
          }
          result = sub_144B99B30(v33, 93);
          v36 = result;
          if ( result )
          {
            sub_1439A5900(result, v29, 0x7FFFFFFF);
            sub_145F3F160(v36, 0);
            sub_141EE9820(v36, 96);
            LOBYTE(v37) = 1;
            sub_14604B700(v36, v37);
            sub_145F335E0(v36);
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v36 + 392LL))(v36, 0x7FFFFFFF);
            v39 = &v38;
            v38 = 0;
            return sub_145C01600(v15, v36, v15, 0, (__int64)&v38, 272, -1, 0, 0);
          }
        }
      }
    }
  }
  return result;
}


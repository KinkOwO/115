// reader_sub_141FC0CC0

void __fastcall sub_141FC0CC0(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rax
  __int64 v6; // rbp
  int v7; // edi
  __int64 v8; // rdx
  _BYTE *v9; // rsi
  __int64 *v10; // rbx
  __int64 *v11; // rcx
  __int64 v12; // rax
  __int64 v13; // rax
  __int64 v14; // rcx
  __int64 v15; // rcx
  __int64 v16; // rcx
  __int64 v17; // rcx
  __int64 v18; // rcx
  __int64 v19; // rcx
  __int64 v20; // rcx
  __int64 v21; // rcx
  __int64 v22; // rcx
  __int64 v23; // rdx
  __int64 v24; // rcx
  __int64 v25; // rcx
  __int64 v26; // rcx
  __int64 v27; // rcx
  __int64 v28; // rcx
  __int64 v29; // rcx
  __int64 v30; // [rsp+20h] [rbp-48h]
  __int64 v31; // [rsp+28h] [rbp-40h] BYREF
  __int16 v32; // [rsp+30h] [rbp-38h]
  __int64 v33; // [rsp+38h] [rbp-30h] BYREF
  int v34; // [rsp+40h] [rbp-28h]
  __int64 v35; // [rsp+78h] [rbp+10h] BYREF

  v30 = -2;
  v2 = qword_14E634248;
  if ( !qword_14E634248 )
  {
    v3 = sub_146E8BA20(2496);
    v35 = v3;
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1456918F0(v3);
    else
      v4 = 0;
    qword_14E634248 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E634248;
  }
  v5 = sub_145693930(v2, 500);
  v6 = v5;
  if ( v5 )
  {
    v7 = 0;
    v9 = (_BYTE *)(sub_1401E65D0(v5) + 13);
    do
    {
      v10 = *(__int64 **)(a1 + 1856);
      v11 = (__int64 *)v10[1];
      while ( !*((_BYTE *)v11 + 25) )
      {
        if ( *((_DWORD *)v11 + 8) >= v7 )
        {
          v10 = v11;
          v11 = (__int64 *)*v11;
        }
        else
        {
          v11 = (__int64 *)v11[2];
        }
      }
      if ( *((_BYTE *)v10 + 25) || v7 < *((_DWORD *)v10 + 8) || v10 == *(__int64 **)(a1 + 1856) )
        goto LABEL_53;
      v31 = 0;
      v32 = 0;
      v33 = 0;
      v34 = 0;
      LODWORD(v35) = 0;
      if ( *(v9 - 1) == 1 )
      {
        sub_142AF0650(v6, (unsigned int)v7, &v31);
        v12 = sub_14074C300(v6);
        sub_1479F0550(v12, (unsigned int)v31, &v33);
        v13 = sub_14074C300(v6);
        sub_1479F0620(v13, (unsigned int)v7, (unsigned int)v31, &v35, v30);
        v14 = v10[17];
        if ( v14 )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v14 + 16LL))(v14, 0);
        v15 = v10[19];
        if ( v15 )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v15 + 16LL))(v15, 0);
        v16 = v10[7];
        if ( v16 )
          sub_14501C1F0(v16, (unsigned int)v35);
        v17 = v10[9];
        if ( v17 )
          sub_14501C1F0(v17, HIDWORD(v33));
        if ( *v9 == 1 )
        {
          v18 = v10[13];
          if ( v18 )
            (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v18 + 16LL))(v18, 0);
          v19 = v10[15];
          if ( v19 )
          {
            LOBYTE(v8) = 1;
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v19 + 16LL))(v19, v8);
          }
          v20 = v10[11];
          if ( v20 )
          {
            if ( !(unsigned __int8)sub_141FB6530(v20) )
            {
              LOBYTE(v8) = 1;
              (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v10[11] + 16LL))(v10[11], v8);
              (*(void (__fastcall **)(__int64))(*(_QWORD *)v10[11] + 432LL))(v10[11]);
            }
          }
          goto LABEL_53;
        }
        if ( (int)v31 < 1 )
        {
          v21 = v10[19];
          if ( v21 )
          {
            LOBYTE(v8) = 1;
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v21 + 16LL))(v21, v8);
          }
        }
        v22 = v10[13];
        if ( v22 )
        {
          LOBYTE(v8) = 1;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v22 + 16LL))(v22, v8);
          LOBYTE(v23) = 1;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v10[13] + 24LL))(v10[13], v23);
        }
        v24 = v10[15];
        if ( v24 )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v24 + 16LL))(v24, 0);
        v25 = v10[11];
        if ( !v25 )
          goto LABEL_53;
        v8 = 0;
      }
      else
      {
        v26 = v10[13];
        if ( v26 )
        {
          LOBYTE(v8) = 1;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v26 + 16LL))(v26, v8);
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v10[13] + 24LL))(v10[13], 0);
        }
        v27 = v10[15];
        if ( v27 )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v27 + 16LL))(v27, 0);
        v28 = v10[11];
        if ( v28 )
          (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v28 + 16LL))(v28, 0);
        v29 = v10[17];
        if ( v29 )
        {
          LOBYTE(v8) = 1;
          (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v29 + 16LL))(v29, v8);
        }
        v25 = v10[19];
        if ( !v25 )
          goto LABEL_53;
        LOBYTE(v8) = 1;
      }
      (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v25 + 16LL))(v25, v8);
LABEL_53:
      ++v7;
      v9 += 12;
    }
    while ( v7 < 3 );
  }
}


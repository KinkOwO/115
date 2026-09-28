// reader_sub_1444DF330

__int64 __fastcall sub_1444DF330(__int64 a1, __int64 a2, __int64 a3, __int64 a4, __int64 a5)
{
  unsigned int v7; // ebp
  unsigned int v8; // ebx
  __int64 v9; // rcx
  int v10; // ecx
  int v11; // ecx
  __int64 v12; // rcx
  __int64 v13; // rdx
  bool v14; // zf
  __int64 v15; // rax
  __int64 v16; // rdx
  __int64 v17; // rcx
  __int64 v18; // rax
  __int64 v19; // rdx
  __int64 v20; // rcx
  void *v21; // rcx
  int v22; // eax
  __int64 v23; // r9
  __int64 v24; // rdx
  __int64 v25; // rax
  __int64 v26; // rcx
  __int64 v27; // rax
  __int64 v28; // rdx
  __int64 v29; // rcx
  __int64 v30; // rcx
  __int64 v31; // rcx
  __int64 v32; // rsi
  int v33; // eax
  __int64 v34; // rdx
  __int64 v35; // rdx
  __int64 v36; // rcx
  __int64 v37; // rcx
  __int64 v38; // rcx
  _BYTE v40[13]; // [rsp+40h] [rbp-48h] BYREF
  int v41; // [rsp+4Dh] [rbp-3Bh]

  v7 = a3;
  v8 = a2;
  v9 = (unsigned int)(*(_DWORD *)(a1 + 180) - 1);
  if ( !(_DWORD)v9 )
  {
    if ( (_DWORD)a3 == 13 )
    {
      if ( (unsigned int)(a2 - 11) <= 2 )
      {
        v41 = a2 - 10;
        v25 = sub_146D74000(v9);
        sub_146D746E0(v25, 1612);
        v27 = sub_146D74000(v26);
        sub_146D75B10(v27, v40, 17);
        sub_146D75AF0(v29, v28);
      }
      if ( v8 == 41 )
      {
        sub_1444E4060(a1 - 1336, 0);
        (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 184) + 16LL))(*(_QWORD *)(a1 + 184), 0);
      }
    }
    goto LABEL_40;
  }
  v10 = v9 - 1;
  if ( !v10 )
  {
    if ( (_DWORD)a3 != 13 )
      goto LABEL_40;
    if ( (_DWORD)a2 == 121 )
    {
      LOBYTE(a3) = 1;
      if ( (unsigned __int8)sub_146682140(qword_14E683C78, 3333, a3) )
      {
LABEL_34:
        v21 = &unk_14A315E38;
        goto LABEL_17;
      }
      v23 = 0;
      v24 = 3333;
    }
    else
    {
      if ( (_DWORD)a2 != 122 )
        goto LABEL_40;
      LOBYTE(a3) = 1;
      if ( (unsigned __int8)sub_146682140(qword_14E683C78, 3335, a3) )
        goto LABEL_34;
      v23 = 1;
      v24 = 3335;
    }
    sub_14668C520(qword_14E683C78, v24, 0, v23);
    goto LABEL_34;
  }
  v11 = v10 - 1;
  if ( !v11 )
  {
    sub_1444DF700(a1 - 1336);
    goto LABEL_40;
  }
  v12 = (unsigned int)(v11 - 1);
  if ( (_DWORD)v12 )
  {
    if ( (_DWORD)v12 != 1 || (_DWORD)a3 != 13 )
      goto LABEL_40;
    if ( (_DWORD)a2 == 411 )
    {
      LOBYTE(a2) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 3232) + 16LL))(*(_QWORD *)(a1 + 3232), a2);
LABEL_16:
      v21 = &unk_14A3128C0;
LABEL_17:
      v22 = sub_146E8C7D0(v21);
      sub_145A31380(v22, -1, 0, 0, -1, -1, 0);
      goto LABEL_40;
    }
    if ( (_DWORD)a2 == 412 )
    {
      if ( !(unsigned __int8)sub_1444DDCF0(a1 - 1336, 3) )
        goto LABEL_16;
LABEL_11:
      LOBYTE(v13) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 4208) + 16LL))(*(_QWORD *)(a1 + 4208), v13);
      goto LABEL_16;
    }
    v14 = (_DWORD)a2 == 413;
  }
  else
  {
    if ( (_DWORD)a3 != 13 )
      goto LABEL_40;
    if ( (_DWORD)a2 == 311 )
    {
      v15 = sub_146D74000(v12);
      v16 = 1621;
      goto LABEL_15;
    }
    if ( (_DWORD)a2 == 312 )
    {
      if ( !(unsigned __int8)sub_1444DDCF0(a1 - 1336, 2) )
        goto LABEL_16;
      goto LABEL_11;
    }
    v14 = (_DWORD)a2 == 313;
  }
  if ( v14 )
  {
    v15 = sub_146D74000(v12);
    v16 = 1617;
LABEL_15:
    sub_146D746E0(v15, v16);
    v18 = sub_146D74000(v17);
    sub_146D75B10(v18, v40, 13);
    sub_146D75AF0(v20, v19);
    goto LABEL_16;
  }
LABEL_40:
  v30 = *(_QWORD *)(a1 + 3232);
  if ( v30 && (unsigned __int8)sub_141FB6530(v30) )
    sub_1444E00A0(a1 - 1336, v8, v7, a4, a5);
  v31 = *(_QWORD *)(a1 + 4208);
  if ( v31 )
  {
    if ( (unsigned __int8)sub_141FB6530(v31) )
    {
      v32 = a1 - 1336;
      if ( v7 == 13 )
      {
        if ( v8 - 811 > 7 )
        {
          if ( v8 == 880 )
          {
            v36 = *(_QWORD *)(v32 + 5544);
            v35 = 0;
            goto LABEL_56;
          }
        }
        else if ( (int)(v8 - 811 + 4 * sub_146EE4DA0(*(_QWORD *)(v32 + 6152))) < (unsigned __int64)((__int64)(*(_QWORD *)(v32 + 6176) - *(_QWORD *)(v32 + 6168)) >> 2) )
        {
          v33 = *(_DWORD *)(v32 + 6224);
          if ( v33 == 2 )
          {
            v34 = 1;
          }
          else
          {
            if ( v33 != 3 )
              goto LABEL_57;
            v34 = 2;
          }
          if ( (unsigned __int8)sub_1444DD900(a1 - 1336, v34) )
          {
            v36 = *(_QWORD *)(v32 + 6232);
            LOBYTE(v35) = 1;
LABEL_56:
            (*(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v36 + 16LL))(v36, v35);
          }
        }
      }
    }
  }
LABEL_57:
  v37 = *(_QWORD *)(a1 + 4896);
  if ( v37 && (unsigned __int8)sub_141FB6530(v37) )
    sub_1444DFB00(a1 - 1336, v8, v7, a4, a5);
  v38 = *(_QWORD *)(a1 + 5072);
  if ( v38 && (unsigned __int8)sub_141FB6530(v38) )
    sub_1444DFD30(a1 - 1336, v8, v7, a4, a5);
  return 0;
}


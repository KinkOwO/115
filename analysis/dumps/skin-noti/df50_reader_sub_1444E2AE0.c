// reader_sub_1444E2AE0

void __fastcall sub_1444E2AE0(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  __int64 v4; // rcx
  __int64 v5; // rax
  __int64 v6; // rdx
  __int64 v7; // rcx
  __int64 v8; // rcx
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rcx
  __int64 v12; // rdi
  __int64 v13; // rax
  __int64 v14; // r8
  __int64 v15; // rsi
  __int64 v16; // rdx
  __int64 v17; // rcx
  __int64 v18; // rcx
  __int64 v19; // rcx
  __int64 v20; // rcx
  __int64 v21; // rcx
  __int64 v22; // rcx
  unsigned __int64 v23; // rcx
  __int64 v24; // rax
  __int64 v25; // rcx
  __int64 v26; // rax
  __int64 v27; // rdx
  __int64 v28; // rcx
  _BYTE v29[13]; // [rsp+30h] [rbp-28h] BYREF
  _DWORD v30[2]; // [rsp+3Dh] [rbp-1Bh]

  v2 = *(_QWORD *)(a1 + 3880);
  if ( v2 && (unsigned __int8)sub_141FB6530(v2) )
  {
    if ( !(unsigned __int8)sub_146AEF960(*(_QWORD *)(a1 + 3880)) )
      return;
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 3880) + 16LL))(*(_QWORD *)(a1 + 3880), 0);
  }
  if ( *(_DWORD *)(a1 + 4288) == 3 )
  {
    if ( !*(_BYTE *)(a1 + 4293) )
    {
      *(_BYTE *)(a1 + 4293) = 1;
      v30[0] = 2;
      v3 = sub_146D74000(v2);
      sub_146D746E0(v3, 1626);
      v5 = sub_146D74000(v4);
      sub_146D75B10(v5, v29, 17);
      sub_146D75AF0(v7, v6);
    }
    return;
  }
  v8 = qword_14E664BF8;
  if ( !qword_14E664BF8 )
  {
    v9 = sub_146E8BA20(2792);
    if ( v9 )
      v10 = (void (__fastcall ***)(_QWORD))sub_1444CC370(v9);
    else
      v10 = 0;
    qword_14E664BF8 = (__int64)v10;
    (**v10)(v10);
    v8 = qword_14E664BF8;
  }
  v12 = sub_1444D2A20(v8);
  if ( v12 )
  {
    v13 = sub_1444D2BB0(v11);
    v15 = sub_1444D2CC0(v13, *(unsigned int *)(358LL * *(int *)(a1 + 4288) + v12 + 890), v14);
    if ( !v15 )
      goto LABEL_28;
    v16 = *(int *)(a1 + 4288);
    if ( !*(_WORD *)(358 * v16 + v12 + 894) )
      goto LABEL_28;
    v17 = *(_QWORD *)(a1 + 16 * v16 + 2616);
    if ( !v17 || !(unsigned __int8)sub_141FB6530(v17) )
      goto LABEL_28;
    if ( !*(_BYTE *)(a1 + 4292) )
    {
      LOWORD(v30[0]) = *(_WORD *)(a1 + 4288);
      v23 = (*(_QWORD *)(v15 + 56) - *(_QWORD *)(v15 + 48)) / 48LL;
      *(_DWORD *)((char *)v30 + 2) = (*(_DWORD *)(a1 + 4404) - 1) % v23;
      v24 = sub_146D74000(v23);
      sub_146D746E0(v24, 1625);
      v26 = sub_146D74000(v25);
      sub_146D75B10(v26, v29, 19);
      sub_146D75AF0(v28, v27);
      *(_BYTE *)(a1 + 4292) = 1;
      return;
    }
    v18 = *(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4288) + 2824);
    if ( v18 )
    {
      if ( (unsigned __int8)sub_146AEF960(v18) )
      {
        v19 = *(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4288) + 2776);
        if ( v19 )
        {
          if ( !(unsigned __int8)sub_141FB6530(v19) )
          {
            v20 = *(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4288) + 2872);
            if ( v20 )
            {
              if ( !(unsigned __int8)sub_141FB6530(v20) )
              {
                v21 = *(_QWORD *)(a1 + 3896);
                if ( v21 )
                {
                  if ( !(unsigned __int8)sub_141FB6530(v21) )
                  {
                    sub_146AF0950(*(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4288) + 2824), 0);
                    v22 = *(_QWORD *)(a1 + 16LL * *(int *)(a1 + 4288) + 2824);
                    (*(void (__fastcall **)(__int64))(*(_QWORD *)v22 + 432LL))(v22);
                    *(_BYTE *)(a1 + 4292) = 0;
LABEL_28:
                    ++*(_DWORD *)(a1 + 4288);
                  }
                }
              }
            }
          }
        }
      }
    }
  }
}


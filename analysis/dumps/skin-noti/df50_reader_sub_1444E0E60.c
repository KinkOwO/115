// reader_sub_1444E0E60

__int64 __fastcall sub_1444E0E60(__int64 a1)
{
  int v2; // eax
  unsigned int v3; // esi
  unsigned int v4; // ebp
  __int64 v5; // rcx
  __int64 v6; // rax
  __int64 v7; // rcx
  __int64 v8; // rsi
  __int64 v9; // rax
  unsigned int v10; // eax
  __int64 v11; // rcx
  __int64 v12; // rsi
  void (__fastcall *v13)(__int64, _QWORD); // rbp
  void (__fastcall ***v14)(_QWORD); // rdi
  __int64 v15; // rcx
  __int64 v16; // rax
  void (__fastcall ***v17)(_QWORD); // rcx
  unsigned __int8 v18; // al
  __int64 v19; // rbx
  __int64 (__fastcall *v20)(__int64, _QWORD); // rsi
  __int64 v21; // rcx
  __int64 v22; // rax
  unsigned __int8 v23; // al

  sub_145F70B70(a1);
  v2 = *(_DWORD *)(a1 + 1516);
  if ( v2 != 1 )
  {
    if ( v2 == 2 )
    {
      sub_1444E2120(a1);
    }
    else if ( v2 == 3 )
    {
      sub_1444E1080(a1);
    }
    else if ( v2 == 6 && (*(_DWORD *)(a1 + 4540) & 2) != 0 )
    {
      v3 = sub_146E9F840(a1 + 4536);
      v4 = sub_140193D40(a1 + 4536);
      if ( (unsigned __int8)sub_146E9FA80(a1 + 4536) )
      {
        v6 = sub_1444D2BB0(v5);
        v8 = sub_1444D2E40(v6);
        if ( v8 )
        {
          v9 = sub_1444D2BB0(v7);
          sub_1444D3750(v9, 0, v8 + 16);
        }
        v3 = v4;
        sub_146E9FBC0(a1 + 4536);
      }
      v10 = sub_146EA1750(255, 0, v3, v4);
      v11 = *(_QWORD *)(a1 + 4520);
      if ( v11 )
        (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v11 + 376LL))(v11, v10);
    }
  }
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 5176)) )
    sub_1444E2DD0(a1);
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 5544)) )
    sub_1444E1CD0(a1);
  if ( (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 6408)) )
    sub_1444E39A0(a1);
  v12 = *(_QWORD *)(a1 + 6776);
  v13 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v12 + 16LL);
  v14 = 0;
  v15 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v16 = sub_146E8BA20(112);
    if ( v16 )
      v17 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v16);
    else
      v17 = 0;
    qword_14E634230 = (__int64)v17;
    (**v17)(v17);
    v15 = qword_14E634230;
  }
  v18 = sub_1403F8790(v15);
  v13(v12, v18);
  v19 = *(_QWORD *)(a1 + 6792);
  v20 = *(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v19 + 16LL);
  v21 = qword_14E634230;
  if ( !qword_14E634230 )
  {
    v22 = sub_146E8BA20(112);
    if ( v22 )
      v14 = (void (__fastcall ***)(_QWORD))sub_1403DE110(v22);
    qword_14E634230 = (__int64)v14;
    (**v14)(v14);
    v21 = qword_14E634230;
  }
  v23 = sub_1403F8790(v21);
  return v20(v19, v23);
}


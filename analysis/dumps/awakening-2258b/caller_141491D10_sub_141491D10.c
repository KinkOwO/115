// sub_141491D10  va=0x141491D10  size=348

void __fastcall sub_141491D10(__int64 a1)
{
  __int64 v1; // rbx
  __int64 v3; // rcx
  __int64 v4; // rsi
  __int64 v5; // rax
  __int64 v6; // rcx
  __int64 v7; // rax
  __int64 v8; // rax
  int v9; // r9d
  int v10; // r8d
  _DWORD *v11; // rbx
  __int64 v12; // rax
  _QWORD v13[4]; // [rsp+30h] [rbp-38h] BYREF

  v1 = 0;
  v3 = *(_QWORD *)(a1 + 8);
  v4 = 0;
  if ( v3 != 0 && sub_14501B3E0(v3) != 0 )
  {
    v5 = sub_14501B3E0(*(_QWORD *)(a1 + 8));
    v4 = _RTDynamicCast(v5, 0, &off_14DCB4FF0, &off_14DCB64C8, 0);
  }
  v6 = *(_QWORD *)(a1 + 192);
  if ( v6 != 0 && sub_14501B3E0(v6) != 0 )
  {
    v7 = sub_14501B3E0(*(_QWORD *)(a1 + 192));
    v1 = _RTDynamicCast(v7, 0, &off_14DCB4FF0, &off_14DCB64C8, 0);
  }
  if ( v4 != 0 && v1 != 0 )
  {
    *(_DWORD *)((char *)&v13[1] + 6) = *(_DWORD *)(a1 + 996);
    BYTE2(v13[2]) = *(_BYTE *)(a1 + 56);
    *(_WORD *)((char *)&v13[2] + 3) = *(_WORD *)(a1 + 60);
    BYTE5(v13[1]) = 0;
    v8 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v1 + 152LL))(v1);
    LOBYTE(v9) = 1;
    LOBYTE(v10) = 50;
    v11 = (_DWORD *)(v8 + 24);
    sub_1480A6620(v8 + 24, 4, v10, v9, v8 + 28);
    *(_DWORD *)((char *)&v13[2] + 5) = *v11;
    v12 = sub_140B8A070();
    sub_140B8AD40(v12, (__int64)v13);
    qmemcpy((void *)(a1 + 1104), v13, 25);
  }
}

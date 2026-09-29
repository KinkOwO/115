// reader_sub_1441EC780

__int64 __fastcall sub_1441EC780(__int64 a1)
{
  unsigned int v2; // esi
  unsigned int v3; // r14d
  __int64 v4; // rbx
  _QWORD *v5; // rdi
  __int64 v6; // rdi
  void (__fastcall *v7)(__int64, _QWORD); // rbx
  __int64 v8; // rsi
  unsigned __int8 v9; // al
  __int64 v10; // rdi
  void (__fastcall *v11)(__int64, __int64); // rbx
  __int64 v12; // rax
  __int64 v13; // rdi
  __int64 (__fastcall *v14)(__int64, __int64); // rbx
  __int64 v15; // rdx
  char v17; // [rsp+50h] [rbp+8h] BYREF
  char *v18; // [rsp+58h] [rbp+10h]

  v2 = 0;
  v3 = -1;
  v4 = 0;
  v5 = (_QWORD *)(a1 + 4144);
  while ( !(unsigned __int8)sub_141FB6530(*v5) )
  {
    ++v2;
    ++v4;
    v5 += 15;
    if ( v4 >= 4 )
      goto LABEL_6;
  }
  v3 = v2;
LABEL_6:
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 4008) + 16LL))(*(_QWORD *)(a1 + 4008), 0);
  sub_146EECBB0(*(_QWORD *)(a1 + 4024), 1);
  if ( v3 <= 3 )
  {
    v6 = *(_QWORD *)(a1 + 4008);
    v7 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v6 + 16LL);
    v8 = 120LL * (int)v3;
    v9 = sub_141FB6530(*(_QWORD *)(v8 + a1 + 4144));
    v7(v6, v9);
    v10 = *(_QWORD *)(a1 + 4008);
    v11 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v10 + 664LL);
    v18 = &v17;
    v12 = sub_146EDD5A0(*(_QWORD *)(v8 + a1 + 4144), &v17);
    v11(v10, v12);
  }
  v13 = *(_QWORD *)(a1 + 4040);
  v14 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v13 + 16LL);
  LOBYTE(v15) = (unsigned __int8)sub_141FB6530(*(_QWORD *)(a1 + 4008)) == 0;
  return v14(v13, v15);
}


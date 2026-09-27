// elemmethod_sub_1441E2D70_0x1441e2d70

char __fastcall sub_1441E2D70(__int64 a1, __int64 a2, __int64 a3)
{
  int v3; // ebx
  int v4; // edi
  __int64 v6; // rdx
  __int64 v7; // rbp
  __int64 v8; // r8
  __int64 v9; // rax
  __int64 v10; // rdi
  void (__fastcall *v11)(__int64, __int64); // rbx
  __int64 v12; // rax
  __int64 v13; // rdx
  int *v14; // rcx
  _BYTE v16[8]; // [rsp+20h] [rbp-28h] BYREF
  __int64 v17; // [rsp+28h] [rbp-20h]
  _BYTE *v18; // [rsp+30h] [rbp-18h]
  int *v19; // [rsp+68h] [rbp+20h] BYREF

  v17 = -2;
  v3 = a3;
  v4 = a2;
  LOBYTE(a3) = 1;
  v7 = sub_14021BE90(qword_14E683B30, a2, a3);
  if ( !v7 )
    return 0;
  v9 = sub_1444EBAB0(0x9C40u, v6, v8);
  if ( !v9 || v3 != 10 && *(_DWORD *)(v9 + 8) != v3 )
    return 0;
  *(_DWORD *)(a1 + 40) = v4;
  if ( qword_14F1C39C8 )
  {
    (*(void (__fastcall **)(_QWORD))(**(_QWORD **)(a1 + 160) + 688LL))(*(_QWORD *)(a1 + 160));
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 112) + 16LL))(*(_QWORD *)(a1 + 112), 0);
    (*(void (__fastcall **)(__int64, int **, _QWORD))(*(_QWORD *)qword_14F1C39C8 + 16LL))(
      qword_14F1C39C8,
      &v19,
      *(_QWORD *)(v7 + 352));
    if ( v19 )
    {
      v10 = *(_QWORD *)(a1 + 112);
      v11 = *(void (__fastcall **)(__int64, __int64))(*(_QWORD *)v10 + 664LL);
      v18 = v16;
      v12 = (*(__int64 (__fastcall **)(int *, _BYTE *, _QWORD))(*(_QWORD *)v19 + 120LL))(v19, v16, *(int *)(v7 + 368));
      v11(v10, v12);
      LOBYTE(v13) = 1;
      (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 112) + 16LL))(*(_QWORD *)(a1 + 112), v13);
    }
    v14 = v19;
    if ( v19 )
    {
      --v19[2];
      if ( v14[2] <= 0 )
        (*(void (__fastcall **)(int *))(*(_QWORD *)v14 + 8LL))(v14);
    }
  }
  return 1;
}


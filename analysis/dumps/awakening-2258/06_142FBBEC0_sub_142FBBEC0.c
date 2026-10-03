// sub_142FBBEC0  size=522

void __fastcall sub_142FBBEC0(__int64 a1, int a2)
{
  __int64 v4; // rcx
  __int64 v5; // rax
  void (__fastcall ***v6)(_QWORD); // rcx
  __int64 v7; // rax
  __int64 v8; // r15
  __int64 v9; // rcx
  __int64 v10; // rcx
  int v11; // r8d
  void (__fastcall *v12)(__int64, _QWORD); // rbx
  unsigned int v13; // eax
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // rsi
  int v17; // ebx
  int v18; // edi
  int v19; // eax
  __int64 v20; // rax
  __int64 v21; // rax
  int v22; // ecx
  __int64 v23; // rax
  __int64 v24; // rax
  int v25; // r8d
  __int64 v26; // [rsp+80h] [rbp+8h] BYREF

  nullsub_1((void *)a1);
  if ( a2 == *(_DWORD *)(a1 + 144) )
  {
    v4 = qword_14E634220;
    if ( qword_14E634220 == 0 )
    {
      v5 = sub_146E8BA20(64);
      v26 = v5;
      __wind
      {
        if ( v5 != 0 )
          v6 = (void (__fastcall ***)(_QWORD))sub_144B6DC50(v5);
        else
          v6 = nullptr;
      }
      __unwind
      {
        j_j_scalable_free(v26, 64);
      }
      qword_14E634220 = (__int64)v6;
      (**v6)(v6);
      v4 = qword_14E634220;
    }
    v7 = sub_144B99B30(v4, 382);
    v8 = v7;
    v9 = *(_QWORD *)(a1 + 160);
    if ( v9 != 0 && *(_DWORD *)(v9 + 8) != 0 )
      v10 = *(_QWORD *)(a1 + 168);
    else
      v10 = 0;
    v11 = v10 - 48;
    if ( v10 == 0 )
      v11 = 0;
    sub_145201600(v7, a1 + 24, v11, *(_DWORD *)(a1 + 144), 82, *(_DWORD *)(a1 + 144));
    v12 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v8 + 392LL);
    v13 = (*(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)a1 + 1632LL))(a1, 23);
    v12(v8, v13);
    v14 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)a1 + 816LL))(a1);
    v15 = sub_145EDB1A0(v14, &v26);
    v16 = v15;
    __wind
    {
      v17 = sub_145387080(a1);
      v18 = *(_DWORD *)(a1 + 144);
      v19 = sub_1453859F0(a1);
    }
    __unwind
    {
      sub_1401DBE30(&v26);
    }
    sub_145F3E300(v8, 2, v19, v18, v17, v16);
    v20 = *(_QWORD *)(a1 + 160);
    if ( v20 != 0 && *(_DWORD *)(v20 + 8) != 0 )
      v21 = *(_QWORD *)(a1 + 168);
    else
      v21 = 0;
    v22 = v21 - 48;
    if ( v21 == 0 )
      v22 = 0;
    v26 = 0;
    __wind
    {
      v23 = *(_QWORD *)(a1 + 160);
      if ( v23 != 0 && *(_DWORD *)(v23 + 8) != 0 )
        v24 = *(_QWORD *)(a1 + 168);
      else
        v24 = 0;
      v25 = v24 - 48;
      if ( v24 == 0 )
        v25 = 0;
    }
    __unwind
    {
      sub_1401DBE30(&v26);
    }
    sub_145C01600(v22, v8, v25, 0, (__int64)&v26, 2258, -1, 0, 0);
  }
}

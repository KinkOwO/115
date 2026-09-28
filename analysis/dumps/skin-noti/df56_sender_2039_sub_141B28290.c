// sender_2039_sub_141B28290

void __fastcall sub_141B28290(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  unsigned int v4; // edi
  __int64 v5; // rax
  __int64 v6; // rax
  __int64 v7; // rax
  unsigned __int64 v8; // rdx
  __int64 v9; // rcx
  unsigned __int64 v10; // rdx
  __int64 v11; // rcx
  int v12; // esi
  __int64 v13; // rax
  void (__fastcall ***v14)(_QWORD); // rcx
  _QWORD *v15; // rdi
  int v16; // ebx
  int v17; // eax
  __int64 v18; // rcx
  __int64 v19; // rax
  __int64 v20; // rcx
  __int64 v21; // rax
  __int64 v22; // rdx
  __int64 v23; // rcx
  unsigned __int64 v24; // rdx
  __int64 v25; // rcx
  _QWORD v26[3]; // [rsp+58h] [rbp-79h] BYREF
  _QWORD v27[3]; // [rsp+70h] [rbp-61h] BYREF
  unsigned __int64 v28; // [rsp+88h] [rbp-49h]
  _QWORD v29[3]; // [rsp+90h] [rbp-41h] BYREF
  unsigned __int64 v30; // [rsp+A8h] [rbp-29h]
  _QWORD v31[3]; // [rsp+B0h] [rbp-21h] BYREF
  unsigned __int64 v32; // [rsp+C8h] [rbp-9h]
  _BYTE v33[13]; // [rsp+D0h] [rbp-1h] BYREF
  int v34; // [rsp+DDh] [rbp+Ch]
  __int64 v35; // [rsp+E1h] [rbp+10h]

  v26[2] = -2;
  if ( !*(_BYTE *)(a1 + 80) )
  {
    *(_BYTE *)(a1 + 80) = 1;
    v35 = 0;
    v34 = 0;
    sub_1475C8FD0(a1 + 88, *(unsigned int *)(a1 + 916), *(unsigned int *)(a1 + 912));
    v3 = sub_145A11970(v2);
    sub_145A07840(v31, v3);
    sub_145A07840(v29, *(int *)(a1 + 908));
    v4 = *(_DWORD *)(a1 + 904);
    v5 = sub_146E8C7D0(&unk_1497B4640);
    v6 = sub_146E8CF20(v26, v5, v4);
    v7 = sub_14014F430(v6);
    sub_14014C7B0(v27, v7);
    sub_146E8C910(v26);
    if ( v30 >= 8 )
    {
      v8 = 2 * v30 + 2;
      v9 = v29[0];
      if ( v8 >= 0x1000 )
      {
        v8 = 2 * v30 + 41;
        v9 = *(_QWORD *)(v29[0] - 8LL);
        if ( (unsigned __int64)(v29[0] - v9 - 8) > 0x1F )
          sub_148AAF304(v9, v8);
      }
      sub_146E9F3A0(v9, v8);
    }
    v29[2] = 0;
    v30 = 7;
    LOWORD(v29[0]) = 0;
    if ( v32 >= 8 )
    {
      v10 = 2 * v32 + 2;
      v11 = v31[0];
      if ( v10 >= 0x1000 )
      {
        v10 = 2 * v32 + 41;
        v11 = *(_QWORD *)(v31[0] - 8LL);
        if ( (unsigned __int64)(v31[0] - v11 - 8) > 0x1F )
          sub_148AAF304(v11, v10);
      }
      sub_146E9F3A0(v11, v10);
    }
    v31[2] = 0;
    v32 = 7;
    LOWORD(v31[0]) = 0;
    v12 = qword_14E6343D0;
    if ( !qword_14E6343D0 )
    {
      v13 = sub_146E8BA20(72);
      v26[0] = v13;
      if ( v13 )
        v14 = (void (__fastcall ***)(_QWORD))sub_146E93360(v13);
      else
        v14 = 0;
      qword_14E6343D0 = (__int64)v14;
      (**v14)(v14);
      v12 = qword_14E6343D0;
    }
    v15 = v27;
    if ( v28 >= 8 )
      v15 = (_QWORD *)v27[0];
    v16 = sub_146E8C7D0(&unk_1497B4710);
    v17 = sub_146E8C7D0(&unk_1497B4240);
    sub_146E938E0(v12, 0, v17, v16, 1127, (__int64)&qword_14E64F2D8, (__int64)v15);
    v19 = sub_146D74000(v18);
    sub_146D746E0(v19, 2241);
    v21 = sub_146D74000(v20);
    sub_146D75B10(v21, v33, 25);
    sub_146D75AF0(v23, v22);
    if ( v28 >= 8 )
    {
      v24 = 2 * v28 + 2;
      v25 = v27[0];
      if ( v24 >= 0x1000 )
      {
        v24 = 2 * v28 + 41;
        v25 = *(_QWORD *)(v27[0] - 8LL);
        if ( (unsigned __int64)(v27[0] - v25 - 8) > 0x1F )
          sub_148AAF304(v25, v24);
      }
      sub_146E9F3A0(v25, v24);
    }
    v27[2] = 0;
    v28 = 7;
    LOWORD(v27[0]) = 0;
  }
}


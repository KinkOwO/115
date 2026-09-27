// slotmap_sub_1447EAC20_0x1447eac20

__int64 __fastcall sub_1447EAC20(__int64 a1, int a2)
{
  __int64 v2; // r15
  __int64 *v3; // r8
  __int64 *v4; // rax
  __int64 *v5; // rcx
  unsigned int v7; // esi
  __int64 v8; // rax
  _QWORD *v9; // r8
  _DWORD **v10; // rax
  _DWORD *v11; // rbx
  __int64 (__fastcall *v12)(__int64, int **, __int64); // rbx
  __int64 v13; // rax
  _DWORD **v14; // rax
  int *v15; // rcx
  __int64 v16; // rdi
  __int64 v17; // rbp
  _QWORD *v18; // rax
  bool v19; // r14
  int *v20; // rcx
  _QWORD *v21; // rax
  int *v22; // rcx
  unsigned int v23; // [rsp+78h] [rbp+10h] BYREF
  int *v24; // [rsp+80h] [rbp+18h] BYREF
  int *v25; // [rsp+88h] [rbp+20h] BYREF

  v23 = a2;
  v2 = a1 + 272;
  v3 = *(__int64 **)(a1 + 272);
  v4 = (__int64 *)v3[1];
  v5 = v3;
  while ( !*((_BYTE *)v4 + 25) )
  {
    if ( *((_DWORD *)v4 + 7) >= a2 )
    {
      v5 = v4;
      v4 = (__int64 *)*v4;
    }
    else
    {
      v4 = (__int64 *)v4[2];
    }
  }
  if ( !*((_BYTE *)v5 + 25) && a2 >= *((_DWORD *)v5 + 7) && v5 != v3 )
    return *((unsigned int *)v5 + 8);
  v7 = 0;
  LOBYTE(v3) = 1;
  v8 = sub_140283D60(qword_14E683BF8, v23, v3);
  if ( v8 && (v9 = *(_QWORD **)(v8 + 656), (__int64)(*(_QWORD *)(v8 + 664) - (_QWORD)v9) >> 5) )
  {
    if ( v9[3] >= 8u )
      v9 = (_QWORD *)*v9;
    v10 = (_DWORD **)(*(__int64 (__fastcall **)(__int64, int **, _QWORD *))(*(_QWORD *)qword_14F1C39C8 + 16LL))(
                       qword_14F1C39C8,
                       &v25,
                       v9);
    v11 = *v10;
    *v10 = 0;
    v24 = 0;
  }
  else
  {
    v12 = *(__int64 (__fastcall **)(__int64, int **, __int64))(*(_QWORD *)qword_14F1C39C8 + 16LL);
    v13 = sub_146E8C7D0(&unk_14A3F4690);
    v14 = (_DWORD **)v12(qword_14F1C39C8, &v25, v13);
    v11 = *v14;
    *v14 = 0;
    v24 = 0;
  }
  v15 = v25;
  if ( v25 )
  {
    --v25[2];
    if ( v15[2] <= 0 )
      (*(void (__fastcall **)(int *))(*(_QWORD *)v15 + 8LL))(v15);
  }
  v16 = 0;
  v17 = 10;
  do
  {
    v18 = (_QWORD *)(*(__int64 (__fastcall **)(_DWORD *, int **, __int64))(*(_QWORD *)v11 + 120LL))(v11, &v24, v16);
    v19 = (*(unsigned int (__fastcall **)(_QWORD))(*(_QWORD *)*v18 + 40LL))(*v18) > v7;
    v20 = v24;
    if ( v24 )
    {
      --v24[2];
      if ( v20[2] <= 0 )
        (*(void (__fastcall **)(int *))(*(_QWORD *)v20 + 8LL))(v20);
    }
    if ( v19 )
    {
      v21 = (_QWORD *)(*(__int64 (__fastcall **)(_DWORD *, int **, __int64))(*(_QWORD *)v11 + 120LL))(v11, &v25, v16);
      v7 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)*v21 + 40LL))(*v21);
      v22 = v25;
      if ( v25 )
      {
        --v25[2];
        if ( v22[2] <= 0 )
          (*(void (__fastcall **)(int *))(*(_QWORD *)v22 + 8LL))(v22);
      }
    }
    ++v16;
    --v17;
  }
  while ( v17 );
  *(_DWORD *)sub_1401C4620(v2, &v23) = v7;
  if ( (int)--v11[2] <= 0 )
    (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)v11 + 8LL))(v11);
  return v7;
}


// singleton_user_sub_1444AA790

__int64 __fastcall sub_1444AA790(__int64 a1, __int64 *a2)
{
  __int64 v4; // rsi
  __int64 v5; // rax
  __int64 (__fastcall ***v6)(_QWORD); // rcx
  __int64 result; // rax
  __int64 v8; // rax
  __int64 v9; // rax
  __int64 v10; // rax
  __int64 v11; // rax
  unsigned int v12; // esi
  int v13; // ebx
  int v14; // eax
  int v15; // edx
  int v16; // r8d
  int *v17; // rcx
  int v18; // edx
  int v19; // r8d
  _BYTE v20[72]; // [rsp+58h] [rbp-50h] BYREF
  int *v21; // [rsp+B0h] [rbp+8h] BYREF

  sub_1467A6790(a1, a2);
  v4 = qword_14E659EA8;
  if ( qword_14E659EA8
    || ((v5 = sub_146E8BA20(496), (v21 = (int *)v5) == 0)
      ? (v6 = 0)
      : (v6 = (__int64 (__fastcall ***)(_QWORD))sub_14449CAF0(v5)),
        qword_14E659EA8 = (__int64)v6,
        result = (**v6)(v6),
        (v4 = qword_14E659EA8) != 0) )
  {
    result = *a2;
    if ( *a2 == *(_QWORD *)(a1 + 1576) )
    {
      v8 = sub_14449E0B0(v4, v20);
      sub_1444A9AB0(a1 - 24, a2, *(unsigned int *)(v8 + 4));
      result = *a2;
    }
    if ( result == *(_QWORD *)(a1 + 1640) )
    {
      v9 = sub_14449E0B0(v4, v20);
      sub_1444A9AB0(a1 - 24, a2, *(unsigned int *)(v9 + 8));
      result = *a2;
    }
    if ( result == *(_QWORD *)(a1 + 1672) )
    {
      v10 = sub_14449E0B0(v4, v20);
      sub_1444A9AB0(a1 - 24, a2, *(unsigned int *)(v10 + 12));
      result = *a2;
    }
    if ( result == *(_QWORD *)(a1 + 1704) )
    {
      v11 = sub_14449E0B0(v4, v20);
      sub_1444A9B80(a1 - 24, a2, *(unsigned int *)(v11 + 16));
      result = *a2;
    }
    if ( result == *(_QWORD *)(a1 + 1608) )
    {
      v12 = *(_DWORD *)(sub_14449E0B0(v4, v20) + 20);
      sub_146ECA0B0(*a2);
      sub_146ECA0C0(*a2);
      sub_142757420(*a2);
      v13 = *(_DWORD *)(*(_QWORD *)(a1 + 8) + 4LL);
      v14 = (*(__int64 (__fastcall **)(_QWORD, int **, __int64))(**(_QWORD **)(a1 + 1488) + 120LL))(
              *(_QWORD *)(a1 + 1488),
              &v21,
              7);
      sub_146EBBA60(v13 + a1 + 8, v15, v16, v14, 1065353216, 1065353216, 0, -1, 2123789977, 2123789977);
      v17 = v21;
      if ( v21 )
      {
        --v21[2];
        if ( v17[2] <= 0 )
          (*(void (__fastcall **)(int *))(*(_QWORD *)v17 + 8LL))(v17);
      }
      sub_146F175E0(*(_QWORD *)(a1 + 1720), v12);
      return sub_146F15BB0(*(_QWORD *)(a1 + 1720), v18, v19, v12, -1, 0, 1065353216, 1065353216);
    }
  }
  return result;
}


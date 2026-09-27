// slotmap_sub_1444E6F30_0x1444e6f30

char __fastcall sub_1444E6F30(__int64 a1, unsigned int a2, __int64 a3)
{
  _DWORD *v6; // rbx
  int v7; // edx
  int v8; // r8d
  int v9; // eax
  __int64 v10; // rax
  __int64 v11; // rcx
  __int64 v12; // rax
  __int64 *v13; // r8
  __int64 *v14; // rax
  __int64 *v15; // rcx
  _DWORD *v16; // rax
  __int64 v17; // rax
  __int64 v18; // rcx
  _DWORD *v19; // rax
  void *retaddr; // [rsp+48h] [rbp+0h]
  int v22; // [rsp+50h] [rbp+8h] BYREF
  __int64 v23; // [rsp+68h] [rbp+20h] BYREF

  if ( *(_QWORD *)(a1 + 920) || a2 && a2 - 1 > 1 )
    return 0;
  v6 = (_DWORD *)sub_140E54E40(a3);
  sub_146E920A0(v6, &v23);
  v7 = v23;
  v8 = *v6 + v23 + 196;
  v9 = v6[1];
  if ( v9 && v8 && v9 != v8 && retaddr )
  {
    sub_146D89B40(retaddr, v6);
    v7 = v23;
  }
  v22 = v7;
  v10 = *(_QWORD *)(qword_14E664C38 + 8);
  v11 = qword_14E664C38;
  while ( !*(_BYTE *)(v10 + 25) )
  {
    if ( *(_DWORD *)(v10 + 28) >= v7 )
    {
      v11 = v10;
      v10 = *(_QWORD *)v10;
    }
    else
    {
      v10 = *(_QWORD *)(v10 + 16);
    }
  }
  if ( !*(_BYTE *)(v11 + 25)
    && v7 >= *(_DWORD *)(v11 + 28)
    && v11 != qword_14E664C38
    && *(int *)sub_1401C4620(&qword_14E664C38, &v22) >= 5 )
  {
    return 0;
  }
  v12 = sub_146E8BA20(272);
  v23 = v12;
  if ( v12 )
    v12 = sub_1444E7230(v12, a3, a2, a1 + 856, -2);
  *(_QWORD *)(a1 + 920) = v12;
  v13 = *(__int64 **)(a1 + 904);
  v14 = (__int64 *)v13[1];
  v15 = v13;
  while ( !*((_BYTE *)v14 + 25) )
  {
    if ( *((_DWORD *)v14 + 7) >= v22 )
    {
      v15 = v14;
      v14 = (__int64 *)*v14;
    }
    else
    {
      v14 = (__int64 *)v14[2];
    }
  }
  if ( *((_BYTE *)v15 + 25) || v22 < *((_DWORD *)v15 + 7) || v15 == v13 )
  {
    *(_DWORD *)sub_1401C4620(a1 + 904, &v22) = 1;
  }
  else
  {
    v16 = (_DWORD *)sub_1401C4620(a1 + 904, &v22);
    ++*v16;
  }
  v17 = *(_QWORD *)(qword_14E664C38 + 8);
  v18 = qword_14E664C38;
  while ( !*(_BYTE *)(v17 + 25) )
  {
    if ( *(_DWORD *)(v17 + 28) >= v22 )
    {
      v18 = v17;
      v17 = *(_QWORD *)v17;
    }
    else
    {
      v17 = *(_QWORD *)(v17 + 16);
    }
  }
  if ( *(_BYTE *)(v18 + 25) || v22 < *(_DWORD *)(v18 + 28) || v18 == qword_14E664C38 )
  {
    *(_DWORD *)sub_1401C4620(&qword_14E664C38, &v22) = 1;
    return 1;
  }
  else
  {
    v19 = (_DWORD *)sub_1401C4620(&qword_14E664C38, &v22);
    ++*v19;
    return 1;
  }
}


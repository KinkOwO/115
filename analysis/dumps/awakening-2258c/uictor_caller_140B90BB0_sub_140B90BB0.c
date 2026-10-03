// sub_140B90BB0  va=0x140B90BB0  size=996

__int64 __fastcall sub_140B90BB0(__int64 a1, unsigned int a2, unsigned int a3)
{
  int v5; // esi
  __int64 v6; // rbp
  __int64 v7; // rax
  __int64 v8; // r15
  __int64 v9; // rax
  void (__fastcall ***v10)(_QWORD); // rcx
  __int64 v11; // rax
  __int64 v12; // rdx
  unsigned int v13; // ebx
  __int64 v14; // rax
  __int64 v15; // rax
  __int64 v16; // r14
  __int64 v17; // rax
  unsigned int *v18; // rbx
  unsigned int v19; // ebx
  unsigned int v20; // eax
  unsigned int v22; // eax
  __int64 *v23; // r8
  __int64 *v24; // rax
  __int64 *v25; // rcx
  int v26; // ebx
  int v27; // eax
  int v28; // edx
  __int64 *v29; // r8
  __int64 *v30; // rax
  __int64 *v31; // rcx
  __int64 v32; // rdx
  __int64 v33; // rax
  __int64 *v34; // rax
  __int64 *v35; // rcx
  __int64 *v36; // r9
  int v37; // eax
  __int64 v38; // [rsp+88h] [rbp+20h]
  __int64 v39; // [rsp+88h] [rbp+20h]
  __int64 v40; // [rsp+88h] [rbp+20h]

  v5 = 0;
  v6 = qword_14E6399A0;
  if ( qword_14E6399A0 == 0 )
  {
    v7 = sub_146E8BA20(1360);
    v38 = v7;
    __wind
    {
      if ( v7 != 0 )
        v6 = sub_140B8D280(v7);
      else
        v6 = 0;
    }
    __unwind
    {
      j_j_scalable_free(v38, 1360);
    }
    qword_14E6399A0 = v6;
  }
  v8 = qword_14E634628;
  if ( qword_14E634628 == 0 )
  {
    v9 = sub_146E8BA20(168);
    v39 = v9;
    __wind
    {
      if ( v9 != 0 )
        v10 = (void (__fastcall ***)(_QWORD))sub_1403B7230(v9);
      else
        v10 = nullptr;
    }
    __unwind
    {
      j_j_scalable_free(v39, 168);
    }
    qword_14E634628 = (__int64)v10;
    (**v10)(v10);
    v8 = qword_14E634628;
  }
  v11 = sub_145A70830(a2);
  if ( v11 == 0 )
    goto LABEL_31;
  v13 = *(_DWORD *)(v11 + 2120);
  if ( v13 == 50 )
  {
    if ( *(_QWORD *)(v11 + 7368) != 0 )
      v13 = *(_DWORD *)(**(_QWORD **)(v11 + 7360) + 28LL);
  }
  else if ( v13 == 36 )
  {
    return 0;
  }
  v14 = sub_145EFAFB0();
  if ( v14 != 0
    && (v15 = (*(__int64 (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v14 + 7944LL))(v14, v13), v16 = v15, v15 != 0) )
  {
    v17 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v15 + 152LL))(v15);
    v18 = (unsigned int *)(v17 + 24);
    sub_1480A6620(v17 + 24, 4, 50, 1, (_DWORD *)(v17 + 28));
    v19 = *v18;
    if ( v19 == a2 || (*(unsigned int (__fastcall **)(__int64))(*(_QWORD *)v16 + 936LL))(v16) == a2 )
      return 1;
    if ( a3 == -1 )
    {
      if ( ((unsigned __int8)sub_145A86890(190, v19) != 0 || (unsigned __int8)sub_145A86890(238, v19) != 0)
        && ((unsigned __int8)sub_145A86890(190, a2) != 0 || (unsigned __int8)sub_145A86890(238, a2) != 0) )
      {
        v20 = sub_1403B8590(v8, a2);
        if ( (int)sub_140B90210(v6, v20) > 0 )
          return 2;
      }
      goto LABEL_47;
    }
  }
  else
  {
LABEL_31:
    if ( a3 == -1 )
      goto LABEL_47;
  }
  if ( ((unsigned __int8)sub_145A86890(190, a3) != 0 || (unsigned __int8)sub_145A86890(238, a3) != 0)
    && ((unsigned __int8)sub_145A86890(190, a2) != 0 || (unsigned __int8)sub_145A86890(238, a2) != 0) )
  {
    v22 = sub_1403B8590(v8, a2);
    v12 = v22;
    if ( v22 != -1 && v22 != 0 )
    {
      v23 = *(__int64 **)(v6 + 632);
      v24 = (__int64 *)v23[1];
      v25 = v23;
      while ( *((_BYTE *)v24 + 25) == 0 )
      {
        if ( *((_DWORD *)v24 + 7) >= (int)v12 )
        {
          v25 = v24;
          v24 = (__int64 *)*v24;
        }
        else
        {
          v24 = (__int64 *)v24[2];
        }
      }
      if ( *((_BYTE *)v25 + 25) == 0 && (int)v12 >= *((_DWORD *)v25 + 7) && v25 != v23 && *((int *)v25 + 8) > 0 )
        return 2;
    }
  }
LABEL_47:
  v26 = sub_1403B80A0(v8, v12);
  if ( v26 == (unsigned int)sub_1403B8580(v8)
    && ((unsigned __int8)sub_145A86890(187, a2) != 0 || (unsigned __int8)sub_145A86890(189, a2) != 0) )
  {
    v27 = sub_1403B8590(v8, a2);
    v28 = v27;
    if ( v27 != -1 && v27 != 0 )
    {
      v29 = *(__int64 **)(v6 + 632);
      v30 = (__int64 *)v29[1];
      v31 = v29;
      while ( *((_BYTE *)v30 + 25) == 0 )
      {
        if ( *((_DWORD *)v30 + 7) >= v28 )
        {
          v31 = v30;
          v30 = (__int64 *)*v30;
        }
        else
        {
          v30 = (__int64 *)v30[2];
        }
      }
      if ( *((_BYTE *)v31 + 25) == 0 && v28 >= *((_DWORD *)v31 + 7) && v31 != v29 && *((int *)v31 + 8) > 0 )
        return 2;
    }
    if ( v28 == a3 )
      return 2;
  }
  v32 = qword_14E6399A0;
  if ( qword_14E6399A0 == 0 )
  {
    v33 = sub_146E8BA20(1360);
    v40 = v33;
    __wind
    {
      if ( v33 != 0 )
        v32 = sub_140B8D280(v33);
      else
        v32 = 0;
    }
    __unwind
    {
      j_j_scalable_free(v40, 1360);
    }
    qword_14E6399A0 = v32;
  }
  if ( a2 == -1 || a2 == 0 )
    goto LABEL_81;
  v34 = *(__int64 **)(*(_QWORD *)(v32 + 632) + 8LL);
  v35 = *(__int64 **)(v32 + 632);
  v36 = v35;
  while ( *((_BYTE *)v34 + 25) == 0 )
  {
    if ( *((_DWORD *)v34 + 7) >= (signed int)a2 )
    {
      v35 = v34;
      v34 = (__int64 *)*v34;
    }
    else
    {
      v34 = (__int64 *)v34[2];
    }
  }
  if ( *((_BYTE *)v35 + 25) != 0 || (signed int)a2 < *((_DWORD *)v35 + 7) )
  {
    v35 = *(__int64 **)(v32 + 632);
    v36 = v35;
  }
  if ( v35 == v36 )
LABEL_81:
    v37 = 0;
  else
    v37 = *((_DWORD *)v35 + 8);
  LOBYTE(v5) = v37 <= 0;
  return (unsigned int)(v5 + 2);
}

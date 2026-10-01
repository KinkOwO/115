// sub_1414928D0  va=0x1414928D0  size=405

__int64 __fastcall sub_1414928D0(__int64 a1)
{
  __int64 v2; // rsi
  unsigned int *v3; // rbx
  unsigned int *v4; // rdi
  __int64 v5; // rcx
  unsigned __int64 v6; // rdx
  __int64 v7; // rdx
  __int64 v8; // rcx
  unsigned __int64 v9; // rdx
  unsigned __int8 v10; // al
  __int64 v11; // rdx
  __int128 v13; // [rsp+28h] [rbp-20h] BYREF
  __int64 v14; // [rsp+38h] [rbp-10h]
  int v15; // [rsp+50h] [rbp+8h] BYREF

  v2 = a1 + 1536;
  if ( (*(unsigned __int8 (__fastcall **)(__int64))(*(_QWORD *)(a1 + 1536) + 96LL))(a1 + 1536) == 0 )
    goto LABEL_16;
  v13 = 0;
  v14 = 0;
  __wind
  {
    (*(void (__fastcall **)(__int64, __int128 *))(*(_QWORD *)v2 + 120LL))(v2, &v13);
  }
  __unwind
  {
    sub_14017AC10(&v13);
  }
  v4 = *((unsigned int **)&v13 + 1);
  v3 = (unsigned int *)v13;
  if ( (_QWORD)v13 != *((_QWORD *)&v13 + 1) )
  {
    while ( 1 )
    {
      v15 = 1;
      if ( (int)sub_145AD8BB0(*v3, &v15) < (int)v3[1] )
        break;
      v3 += 2;
      if ( v3 == v4 )
        goto LABEL_6;
    }
    v8 = v13;
    if ( (_QWORD)v13 == 0 )
      goto LABEL_16;
    v9 = (v14 - v13) & 0xFFFFFFFFFFFFFFF8uLL;
    if ( v9 < 0x1000 || (v9 += 39LL, v8 = *(_QWORD *)(v13 - 8), (unsigned __int64)(v13 - v8 - 8) <= 0x1F) )
    {
      j_j_scalable_free(v8, v9);
      v13 = 0;
      v14 = 0;
      goto LABEL_16;
    }
LABEL_18:
    invalid_parameter_noinfo_noreturn();
  }
LABEL_6:
  v5 = v13;
  if ( (_QWORD)v13 != 0 )
  {
    v6 = (v14 - v13) & 0xFFFFFFFFFFFFFFF8uLL;
    if ( v6 >= 0x1000 )
    {
      v6 += 39LL;
      v5 = *(_QWORD *)(v13 - 8);
      if ( (unsigned __int64)(v13 - v5 - 8) > 0x1F )
        goto LABEL_18;
    }
    j_j_scalable_free(v5, v6);
    v13 = 0;
    v14 = 0;
  }
  if ( (unsigned __int8)sub_141489760(v2) == 0 )
  {
LABEL_16:
    v7 = 0;
    goto LABEL_17;
  }
  LOBYTE(v7) = 1;
LABEL_17:
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2784) + 24LL))(*(_QWORD *)(a1 + 2784), v7);
  v10 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v2 + 104LL))(v2);
  (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 2800) + 24LL))(*(_QWORD *)(a1 + 2800), v10);
  LOBYTE(v11) = 1;
  return (*(__int64 (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2896) + 24LL))(*(_QWORD *)(a1 + 2896), v11);
}

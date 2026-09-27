// ui_1441E3F40

__int64 __fastcall sub_1441E3F40(__int64 a1)
{
  __int64 v2; // rcx
  __int64 v3; // rax
  void (__fastcall ***v4)(_QWORD); // rcx
  __int64 v5; // rax
  __int64 v6; // rdx
  _OWORD *v7; // r8
  __int64 *v8; // rbx
  __int64 v9; // rax
  int v10; // ecx
  int v11; // ecx
  int v12; // ecx
  bool v13; // zf
  __int64 **v14; // rax
  __int64 *i; // rax
  __int64 *j; // rcx
  __int64 v17; // rdx
  __int64 v18; // rcx
  __int64 *v19; // rbx
  __int64 *v20; // rcx
  _QWORD v22[4]; // [rsp+28h] [rbp-20h] BYREF

  *(_QWORD *)(a1 + 24) = *(_QWORD *)(a1 + 16);
  v2 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    v3 = sub_146E8BA20(1472);
    if ( v3 )
      v4 = (void (__fastcall ***)(_QWORD))sub_1444E81C0(v3);
    else
      v4 = 0;
    qword_14E638F28 = (__int64)v4;
    (**v4)(v4);
    v2 = qword_14E638F28;
  }
  v5 = sub_1444EBDF0(v2, 0);
  sub_1441B8B20(v22, v5);
  v8 = *(__int64 **)v22[0];
  while ( v8 != (__int64 *)v22[0] )
  {
    v9 = sub_1444EBAB0(*((_DWORD *)v8 + 7), v6, (__int64)v7);
    if ( !v9 )
      goto LABEL_25;
    if ( *(_DWORD *)(v9 + 8) )
      goto LABEL_25;
    v10 = *((_DWORD *)v8 + 7);
    if ( v10 == 20000 || v10 == 60000 || v10 == 80000 || v10 == 50000 )
      goto LABEL_25;
    v6 = *(unsigned int *)(a1 + 2344);
    v11 = *(_DWORD *)(v9 + 12);
    if ( v11 )
    {
      v12 = v11 - 1;
      if ( v12 )
      {
        if ( v12 != 1 )
        {
          if ( (unsigned int)(v6 - 1) > 2 )
            goto LABEL_22;
          goto LABEL_25;
        }
        v13 = (_DWORD)v6 == 3;
      }
      else
      {
        v13 = (_DWORD)v6 == 2;
      }
    }
    else
    {
      v13 = (_DWORD)v6 == 1;
    }
    if ( v13 )
    {
LABEL_22:
      v7 = v8 + 4;
      v6 = *(_QWORD *)(a1 + 24);
      if ( v6 == *(_QWORD *)(a1 + 32) )
      {
        sub_1405EA310(a1 + 16, v6, v7);
      }
      else
      {
        *(_OWORD *)v6 = *v7;
        *(_QWORD *)(v6 + 16) = v8[6];
        *(_QWORD *)(a1 + 24) += 24LL;
      }
    }
LABEL_25:
    v14 = (__int64 **)v8[2];
    if ( *((_BYTE *)v14 + 25) )
    {
      for ( i = (__int64 *)v8[1]; !*((_BYTE *)i + 25); i = (__int64 *)i[1] )
      {
        if ( v8 != (__int64 *)i[2] )
          break;
        v8 = i;
      }
      v8 = i;
    }
    else
    {
      v8 = (__int64 *)v8[2];
      for ( j = *v14; !*((_BYTE *)j + 25); j = (__int64 *)*j )
        v8 = j;
    }
  }
  sub_141FD8CD0(*(_QWORD *)(a1 + 16), *(_QWORD *)(a1 + 24), (*(_QWORD *)(a1 + 24) - *(_QWORD *)(a1 + 16)) / 24LL, 0);
  LOBYTE(v17) = *(_QWORD *)(a1 + 16) == *(_QWORD *)(a1 + 24);
  (*(void (__fastcall **)(_QWORD, __int64))(**(_QWORD **)(a1 + 2328) + 16LL))(*(_QWORD *)(a1 + 2328), v17);
  v18 = v22[0];
  v19 = *(__int64 **)(v22[0] + 8LL);
  if ( !*((_BYTE *)v19 + 25) )
  {
    do
    {
      sub_1401DBB80(v22, v22, v19[2]);
      v20 = v19;
      v19 = (__int64 *)*v19;
      sub_146E9F3A0(v20, 56);
    }
    while ( !*((_BYTE *)v19 + 25) );
    v18 = v22[0];
  }
  return sub_146E9F3A0(v18, 56);
}


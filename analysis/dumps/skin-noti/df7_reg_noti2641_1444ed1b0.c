__int64 sub_1444ED1B0()
{
  __int64 v0; // rdi
  _QWORD **v1; // rcx
  _QWORD *v2; // rcx
  _QWORD *v3; // rbx
  int v4; // esi
  unsigned int v5; // eax
  int i; // edi
  __int64 v7; // rcx
  __int64 v8; // rcx
  __int64 v9; // rbx
  __int64 v10; // rax
  int *v11; // rax
  __int64 v12; // rcx
  int j; // esi
  __int64 result; // rax
  int k; // edi
  __int64 v16; // rcx
  __int64 v17; // rcx
  __int64 v18; // rbx
  __int64 v19; // rax
  int *v20; // rax
  __int64 v21; // rcx
  int v22; // [rsp+20h] [rbp-10h] BYREF
  int v23; // [rsp+24h] [rbp-Ch]
  signed int v24; // [rsp+28h] [rbp-8h] BYREF
  unsigned int v25; // [rsp+2Ch] [rbp-4h]
  unsigned int v26; // [rsp+60h] [rbp+30h] BYREF

  v0 = qword_14E638F28;
  if ( !qword_14E638F28 )
  {
    qword_14E638F28 = sub_1444E7F30();
    (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
    v0 = qword_14E638F28;
  }
  v1 = *(_QWORD ***)(v0 + 8);
  *v1[1] = 0;
  v2 = *v1;
  if ( v2 )
  {
    do
    {
      v3 = (_QWORD *)*v2;
      sub_146E9F3A0(v2, 32);
      v2 = v3;
    }
    while ( v3 );
  }
  v4 = 0;
  **(_QWORD **)(v0 + 8) = *(_QWORD *)(v0 + 8);
  *(_QWORD *)(*(_QWORD *)(v0 + 8) + 8LL) = *(_QWORD *)(v0 + 8);
  *(_QWORD *)(v0 + 16) = 0;
  do
  {
    v26 = 0;
    sub_146EA0BA0(&v26);
    v5 = v26;
    for ( i = 0; i < (int)v26; ++i )
    {
      v23 = -1;
      v24 = 0;
      v22 = v4;
      v25 = v5;
      sub_146EA0BA0(&v24);
      if ( sub_145EFAFB0(v7) )
      {
        v9 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v9 = qword_14E638F28;
        }
        v10 = sub_145EFAFB0(v8);
        v11 = (int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v10 + 7656LL))(v10);
        v24 = sub_1473A1120(v9 + 1328, *v11, v24);
      }
      v12 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        qword_14E638F28 = sub_1444E7F30();
        (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        v12 = qword_14E638F28;
      }
      sub_1444E8DF0(v12, &v22);
      v5 = v26;
    }
    ++v4;
  }
  while ( v4 < 10 );
  for ( j = 0; j < 4; ++j )
  {
    v26 = 0;
    sub_146EA0BA0(&v26);
    result = v26;
    for ( k = 0; k < (int)v26; ++k )
    {
      v22 = -1;
      v24 = 0;
      v23 = j;
      v25 = result;
      sub_146EA0BA0(&v24);
      if ( sub_145EFAFB0(v16) )
      {
        v18 = qword_14E638F28;
        if ( !qword_14E638F28 )
        {
          qword_14E638F28 = sub_1444E7F30();
          (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
          v18 = qword_14E638F28;
        }
        v19 = sub_145EFAFB0(v17);
        v20 = (int *)(*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v19 + 7656LL))(v19);
        v24 = sub_1473A1120(v18 + 1328, *v20, v24);
      }
      v21 = qword_14E638F28;
      if ( !qword_14E638F28 )
      {
        qword_14E638F28 = sub_1444E7F30();
        (**(void (__fastcall ***)(__int64))qword_14E638F28)(qword_14E638F28);
        v21 = qword_14E638F28;
      }
      sub_1444E8DF0(v21, &v22);
      result = v26;
    }
  }
  return result;
}

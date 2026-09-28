// bus_sub_146694510

void __fastcall sub_146694510(__int64 a1, unsigned int a2, int a3, unsigned __int8 a4, char a5)
{
  unsigned int v7; // edi
  _QWORD *v9; // rax
  __int64 *v10; // rdi
  __int64 *i; // rbx
  __int64 v12; // rax
  _QWORD *v13; // rbp
  _QWORD *j; // rsi
  __int64 v15; // r14
  _QWORD **v16; // rcx
  _QWORD *v17; // rcx
  _QWORD *v18; // rbx
  _QWORD *v19; // rbx
  _QWORD *k; // rdi
  __int128 v21; // [rsp+28h] [rbp-40h] BYREF

  v7 = a2;
  if ( a5 )
  {
    v21 = 0;
    v9 = (_QWORD *)sub_146E8BA20(24);
    *v9 = v9;
    v9[1] = v9;
    *(_QWORD *)&v21 = v9;
    if ( (unsigned __int8)sub_14667E500(a1, v7, &v21) )
    {
      v10 = (__int64 *)v21;
      for ( i = *(__int64 **)v21; i != v10; i = (__int64 *)*i )
      {
        v12 = *((int *)i + 4);
        v13 = *(_QWORD **)(a1 + 24 * v12 + 480);
        for ( j = *(_QWORD **)(a1 + 24 * v12 + 472); j != v13; ++j )
        {
          v15 = *j;
          if ( *j && (*(unsigned __int8 (__fastcall **)(_QWORD))(*(_QWORD *)v15 + 840LL))(*j) )
            sub_146694390(a1, v15, a4);
        }
      }
      v7 = a2;
    }
    v16 = (_QWORD **)v21;
    **(_QWORD **)(v21 + 8) = 0;
    v17 = *v16;
    if ( v17 )
    {
      do
      {
        v18 = (_QWORD *)*v17;
        sub_146E9F3A0((__int64)v17, 24);
        v17 = v18;
      }
      while ( v18 );
    }
    sub_146E9F3A0(v21, 24);
  }
  if ( v7 != 4176 )
  {
    v19 = *(_QWORD **)(a1 + 24LL * (int)v7 + 472);
    if ( v19 != *(_QWORD **)(a1 + 24LL * (int)v7 + 480) )
    {
      for ( k = (_QWORD *)(a1 + 24 * ((int)v7 + 20LL)); v19 != (_QWORD *)*k; ++v19 )
      {
        if ( a3 < 0 || (unsigned int)sub_1467A29B0(*v19) == a3 )
          sub_146694390(a1, *v19, a4);
      }
    }
  }
}


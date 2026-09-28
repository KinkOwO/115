// singleton_user_sub_1444ADF40

__int64 __fastcall sub_1444ADF40(__int64 a1, int a2, _QWORD *a3, unsigned int a4, unsigned int a5, unsigned int a6)
{
  int v10; // ebx
  __int64 v11; // rcx
  __int64 v12; // rax
  void (__fastcall ***v13)(_QWORD); // rcx
  unsigned int v14; // eax
  __int64 v15; // r8
  __int64 v16; // rdi
  __int64 v17; // rbp
  __int64 v18; // rax
  __int64 v19; // rdx
  __int64 v20; // rcx
  _BYTE *i; // rax
  __int64 v22; // rbp
  _QWORD *v23; // rax
  __int64 v24; // rdx
  volatile signed __int32 *v25; // rdi
  __int64 v26; // rbx
  __int64 v27; // rax
  _QWORD *v28; // rdx

  v10 = 0;
  v11 = qword_14E659EA8;
  if ( !qword_14E659EA8 )
  {
    v12 = sub_146E8BA20(496);
    if ( v12 )
      v13 = (void (__fastcall ***)(_QWORD))sub_14449CAF0(v12);
    else
      v13 = 0;
    qword_14E659EA8 = (__int64)v13;
    (**v13)(v13);
    v11 = qword_14E659EA8;
  }
  v14 = sub_14449DFC0(v11, a4, a5);
  LOBYTE(v15) = 1;
  v16 = sub_140283D60(qword_14E683B20, v14, v15);
  v17 = *(_QWORD *)(v16 + 4752);
  v18 = sub_146E8BA20(2800);
  if ( v18 )
    v19 = sub_146B6BC70(v18, v17, *(_QWORD *)(v16 + 3816), 0, 0);
  else
    v19 = 0;
  if ( v19 )
  {
    v20 = 0;
    for ( i = (_BYTE *)(a1 + 1728); *i; i += 88 )
    {
      ++v10;
      if ( ++v20 >= 8 )
        return sub_14014C710(a3);
    }
    v22 = a1 + 88LL * v10;
    *(_BYTE *)(v22 + 1728) = 1;
    *(_DWORD *)(v22 + 1732) = a2;
    v23 = (_QWORD *)sub_140251BC0(v19);
    v24 = v23[1];
    if ( v24 )
    {
      _InterlockedIncrement((volatile signed __int32 *)(v24 + 8));
      v24 = v23[1];
    }
    *(_QWORD *)(v22 + 1712) = *v23;
    v25 = *(volatile signed __int32 **)(v22 + 1720);
    *(_QWORD *)(v22 + 1720) = v24;
    v26 = -1;
    if ( v25 )
    {
      if ( _InterlockedExchangeAdd(v25 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v25)(v25);
        if ( _InterlockedExchangeAdd(v25 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v25 + 8LL))(v25);
      }
    }
    v27 = sub_145CDE180(a4, a5, a6, 0);
    do
      ++v26;
    while ( *(_WORD *)(v27 + 2 * v26) );
    sub_14014C8D0(v22 + 1680, v27);
    if ( (_QWORD *)(v22 + 1648) != a3 )
    {
      v28 = a3;
      if ( a3[3] >= 8u )
        v28 = (_QWORD *)*a3;
      sub_14014C8D0(v22 + 1648, v28);
    }
  }
  return sub_14014C710(a3);
}


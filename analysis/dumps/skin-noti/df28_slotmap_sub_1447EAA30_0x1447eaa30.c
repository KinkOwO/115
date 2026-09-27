// slotmap_sub_1447EAA30_0x1447eaa30

__int64 __fastcall sub_1447EAA30(__int64 a1, signed int a2, unsigned __int8 a3)
{
  __int64 v3; // r14
  __int64 *v4; // rax
  __int64 *v5; // r9
  signed int v6; // ebp
  int i; // r15d
  __int64 v8; // rsi
  __int64 v9; // rdi
  _QWORD *v10; // rax
  int v11; // ebx
  int v12; // ebx
  signed int v13; // ecx
  volatile signed __int32 *v14; // rbx
  volatile signed __int32 *v15; // rbx
  __int128 v17; // [rsp+28h] [rbp-50h] BYREF
  __int128 v18; // [rsp+38h] [rbp-40h]
  __int128 v19; // [rsp+48h] [rbp-30h]
  int v20; // [rsp+88h] [rbp+10h] BYREF

  v20 = a2;
  v3 = a1 + 16 * (a3 + 18LL);
  v4 = *(__int64 **)(*(_QWORD *)v3 + 8LL);
  v5 = *(__int64 **)v3;
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
  if ( *((_BYTE *)v5 + 25) || a2 < *((_DWORD *)v5 + 7) || v5 == *(__int64 **)v3 )
  {
    sub_1447EE080(a1, &v17, (unsigned int)a2, a3);
    v6 = 0x80000000;
    for ( i = 0; i < (int)sub_146B33E50(v17); ++i )
    {
      v8 = sub_146B33E00(v17, (unsigned int)i);
      v9 = *(_QWORD *)sub_1472E7960(v8);
      v10 = (_QWORD *)sub_1472E7960(v8);
      v11 = (*(__int64 (__fastcall **)(_QWORD))(*(_QWORD *)*v10 + 56LL))(*v10);
      v12 = (*(__int64 (__fastcall **)(__int64))(*(_QWORD *)v9 + 72LL))(v9) + v11;
      v13 = sub_1472E7A40(v8) + v12;
      if ( v13 < v6 )
        v13 = v6;
      v6 = v13;
    }
    *(_DWORD *)sub_1401C4620(v3, &v20) = v6;
    sub_1401F4EE0(&v17);
    v19 = 0;
    v18 = v17;
    v14 = (volatile signed __int32 *)*((_QWORD *)&v17 + 1);
    v17 = 0u;
    if ( *((_QWORD *)&v18 + 1) )
    {
      if ( _InterlockedExchangeAdd(v14 + 2, 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v14)(v14);
        if ( _InterlockedExchangeAdd(v14 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v14 + 8LL))(v14);
      }
    }
    v15 = (volatile signed __int32 *)*((_QWORD *)&v17 + 1);
    if ( *((_QWORD *)&v17 + 1) )
    {
      if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v17 + 1) + 8LL), 0xFFFFFFFF) == 1 )
      {
        (**(void (__fastcall ***)(volatile signed __int32 *))v15)(v15);
        if ( _InterlockedExchangeAdd(v15 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v15 + 8LL))(v15);
      }
    }
  }
  return *(unsigned int *)sub_1401C4620(v3, &v20);
}


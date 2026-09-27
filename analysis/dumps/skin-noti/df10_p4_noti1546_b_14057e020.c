_QWORD *__fastcall sub_14057E020(_QWORD *a1, __int64 a2)
{
  __int64 v4; // rdi
  volatile signed __int32 *v5; // rcx
  __int64 v6; // rax
  __int64 v7; // r8
  volatile signed __int32 *v8; // rcx

  v4 = 0;
  *a1 = &off_149254B60;
  a1[1] = 0;
  a1[2] = 0;
  if ( a2 )
  {
    v6 = *(_QWORD *)(a2 + 16);
    v7 = 0;
    if ( v6 )
    {
      v4 = *(_QWORD *)(a2 + 8);
      _InterlockedIncrement((volatile signed __int32 *)(v6 + 12));
      v7 = v6;
    }
    a1[1] = v4;
    v8 = (volatile signed __int32 *)a1[2];
    a1[2] = v7;
    if ( v8 && _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
    v4 = a2;
  }
  else
  {
    a1[1] = 0;
    v5 = (volatile signed __int32 *)a1[2];
    a1[2] = 0;
    if ( v5 && _InterlockedExchangeAdd(v5 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v5 + 8LL))(v5);
  }
  a1[3] = v4;
  return a1;
}

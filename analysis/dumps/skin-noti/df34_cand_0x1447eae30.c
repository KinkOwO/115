// cand_0x1447eae30

__int64 __fastcall sub_1447EAE30(unsigned int a1, __int64 a2, __int64 a3, __int64 a4)
{
  __int64 v4; // rsi
  __int64 v5; // rax
  volatile signed __int32 *v6; // rbx
  int *v7; // rcx
  volatile signed __int32 *v8; // rbx
  __int128 v10; // [rsp+28h] [rbp-40h] BYREF
  __int128 v11; // [rsp+38h] [rbp-30h]
  __int128 v12; // [rsp+48h] [rbp-20h]

  v4 = 9999999;
  LOBYTE(a4) = 1;
  sub_14085BC60(qword_14E683BF8, &v10, a1, a4);
  v5 = v10;
  if ( (_QWORD)v10 )
  {
    if ( *(_DWORD *)(v10 + 8) != 2 )
    {
      v12 = 0;
      v11 = v10;
      v6 = (volatile signed __int32 *)*((_QWORD *)&v10 + 1);
      v10 = 0u;
      if ( *((_QWORD *)&v11 + 1) )
      {
        if ( _InterlockedExchangeAdd(v6 + 2, 0xFFFFFFFF) == 1 )
        {
          (**(void (__fastcall ***)(volatile signed __int32 *))v6)(v6);
          if ( _InterlockedExchangeAdd(v6 + 3, 0xFFFFFFFF) == 1 )
            (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v6 + 8LL))(v6);
        }
      }
      v5 = v10;
    }
    if ( v5 )
    {
      v7 = *(int **)(v5 + 632);
      if ( (__int64)(*(_QWORD *)(v5 + 640) - (_QWORD)v7) >> 2 )
      {
        if ( *v7 != -1 )
          v4 = *v7;
      }
    }
  }
  v8 = (volatile signed __int32 *)*((_QWORD *)&v10 + 1);
  if ( *((_QWORD *)&v10 + 1) )
  {
    if ( _InterlockedExchangeAdd((volatile signed __int32 *)(*((_QWORD *)&v10 + 1) + 8LL), 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
      if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
    }
  }
  return v4;
}


// reg_sub_14076EC30

__int64 __fastcall sub_14076EC30(__int64 a1, __int64 a2, __int64 a3)
{
  int v3; // ebp
  __int64 result; // rax
  __int64 v6; // rax
  volatile signed __int32 *v7; // rbx
  volatile signed __int32 *v8; // rbx
  volatile signed __int32 *v9; // [rsp+30h] [rbp-48h] BYREF
  volatile signed __int32 *v10; // [rsp+38h] [rbp-40h]
  __int16 v11; // [rsp+40h] [rbp-38h] BYREF
  int v12; // [rsp+42h] [rbp-36h]
  __m128i si128; // [rsp+46h] [rbp-32h]
  char v14; // [rsp+56h] [rbp-22h]
  int v15; // [rsp+57h] [rbp-21h]

  v3 = a2;
  result = sub_1444EBAB0(a2, a2, a3);
  if ( result )
  {
    if ( !*(_QWORD *)(a1 + 8016) )
    {
      v6 = sub_146E8BA20(128);
      v7 = (volatile signed __int32 *)v6;
      if ( v6 )
      {
        *(_OWORD *)v6 = 0;
        *(_DWORD *)(v6 + 8) = 1;
        *(_DWORD *)(v6 + 12) = 1;
        *(_QWORD *)v6 = off_14931B778;
        sub_1444E8670((_QWORD *)(v6 + 16));
      }
      else
      {
        v7 = 0;
      }
      v9 = v7 + 4;
      v10 = v7;
      sub_1401E5080(a1 + 8016, &v9);
      if ( v10 && _InterlockedExchangeAdd(v10 + 2, 0xFFFFFFFF) == 1 )
      {
        v8 = v10;
        (**(void (__fastcall ***)(volatile signed __int32 *))v10)(v10);
        if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
          (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v10 + 8LL))(v10);
      }
    }
    v11 = 0;
    si128 = _mm_load_si128(xmmword_1491B4820);
    v15 = 0;
    v12 = v3;
    v14 = 1;
    return sub_1444F1920(*(_QWORD *)(a1 + 8016), (__int64)&v11);
  }
  return result;
}


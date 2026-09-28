// reader_sub_141FB0AE0

__int64 __fastcall sub_141FB0AE0(__int64 a1, __int64 *a2)
{
  __int64 result; // rax
  __int64 v5; // rbp
  __int64 v6; // r14
  volatile signed __int32 *v7; // rbx
  __int64 *v8; // rbx
  __int64 v9; // rdi
  _BYTE v10[8]; // [rsp+28h] [rbp-20h] BYREF
  volatile signed __int32 *v11; // [rsp+30h] [rbp-18h]

  sub_1467A6790(a1, a2);
  result = (*(__int64 (__fastcall **)(__int64, _BYTE *))(*(_QWORD *)(a1 - 24) + 272LL))(a1 - 24, v10);
  v5 = *(_QWORD *)result;
  v6 = *a2;
  v7 = v11;
  if ( v11 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v11 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v7)(v7);
      if ( _InterlockedExchangeAdd(v7 + 3, 0xFFFFFFFF) == 1 )
        result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v7 + 8LL))(v7);
    }
  }
  if ( v5 == v6 )
  {
    v8 = (__int64 *)(a1 + 1584);
    v9 = 5;
    do
    {
      if ( *v8 )
      {
        result = sub_141FB6530(*v8);
        if ( (_BYTE)result )
          result = sub_146ECA5A0(*v8);
      }
      v8 += 15;
      --v9;
    }
    while ( v9 );
  }
  return result;
}


// reader_sub_141FD5C20

void __fastcall sub_141FD5C20(__int64 a1)
{
  __int64 v2; // rdi
  void (__fastcall *v3)(__int64, _QWORD); // rbx
  unsigned __int8 v4; // al
  __int64 v5; // rdi
  void (__fastcall *v6)(__int64, _QWORD); // rbx
  unsigned __int8 v7; // al
  volatile signed __int32 *v8; // rbx
  volatile signed __int32 *v9; // rbx
  __int64 v10; // [rsp+28h] [rbp-30h] BYREF
  volatile signed __int32 *v11; // [rsp+30h] [rbp-28h]
  __int64 v12; // [rsp+38h] [rbp-20h] BYREF
  volatile signed __int32 *v13; // [rsp+40h] [rbp-18h]

  sub_141FC7B00(a1, &v12, 14);
  sub_141FC7B00(a1, &v10, 15);
  v2 = v10;
  if ( v10 && v12 )
  {
    v3 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v10 + 16LL);
    v4 = sub_141FB6530(v12);
    v3(v2, v4);
    v5 = v10;
    v6 = *(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v10 + 24LL);
    v7 = sub_141FB6530(v12);
    v6(v5, v7);
  }
  v8 = v11;
  if ( v11 )
  {
    if ( _InterlockedExchangeAdd(v11 + 2, 0xFFFFFFFF) == 1 )
    {
      (**(void (__fastcall ***)(volatile signed __int32 *))v8)(v8);
      if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
        (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
    }
  }
  v9 = v13;
  if ( v13 && _InterlockedExchangeAdd(v13 + 2, 0xFFFFFFFF) == 1 )
  {
    (**(void (__fastcall ***)(volatile signed __int32 *))v9)(v9);
    if ( _InterlockedExchangeAdd(v9 + 3, 0xFFFFFFFF) == 1 )
      (*(void (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v9 + 8LL))(v9);
  }
}


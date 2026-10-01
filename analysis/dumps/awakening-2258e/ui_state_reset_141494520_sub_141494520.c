// sub_141494520  va=0x141494520  size=254

__int64 __fastcall sub_141494520(__int64 a1, char a2)
{
  __int64 v4; // rax
  unsigned __int8 v5; // bl
  __int64 v6; // rcx
  __int64 result; // rax
  volatile signed __int32 *v8; // rbx
  __int64 v9; // [rsp+28h] [rbp-20h] BYREF
  volatile signed __int32 *v10; // [rsp+30h] [rbp-18h]

  v4 = sub_146E8C7D0(&unk_149272650);
  sub_145F6E4E0(a1, &v9, v4);
  __wind
  {
    v5 = a2 ^ 1;
    if ( v9 != 0 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v9 + 24LL))(v9, v5);
    (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 2928) + 24LL))(*(_QWORD *)(a1 + 2928), v5);
    v6 = *(_QWORD *)(a1 + 1520);
    if ( v6 != 0 )
      (*(void (__fastcall **)(__int64, _QWORD))(*(_QWORD *)v6 + 24LL))(v6, v5);
    if ( a2 != 0 )
    {
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 2784) + 24LL))(*(_QWORD *)(a1 + 2784), 0);
      (*(void (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 2800) + 24LL))(*(_QWORD *)(a1 + 2800), 0);
      result = (*(__int64 (__fastcall **)(_QWORD, _QWORD))(**(_QWORD **)(a1 + 2896) + 24LL))(*(_QWORD *)(a1 + 2896), 0);
    }
    else
    {
      result = sub_1414928D0(a1);
    }
  }
  __unwind
  {
    sub_1401566D0(&v9);
  }
  v8 = v10;
  if ( v10 != nullptr )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v10 + 2, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
    {
      result = (**(__int64 (__fastcall ***)(volatile signed __int32 *))v8)(v8);
      if ( _InterlockedExchangeAdd(v8 + 3, 0xFFFFFFFF) == 1 )
        return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v8 + 8LL))(v8);
    }
  }
  return result;
}

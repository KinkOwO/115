// reader_sub_1441DCCA0

__int64 __fastcall sub_1441DCCA0(_QWORD *a1)
{
  _QWORD *v2; // rbx
  __int64 v3; // rsi
  __int64 v4; // rdi
  __int64 (__fastcall *v5)(__int64, __int64); // rbp
  __int64 v6; // rdx
  __int64 result; // rax

  if ( a1[7] && (unsigned __int8)sub_146F593F0() )
    (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 80LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 96LL))(a1);
  v2 = a1 + 45;
  v3 = 7;
  do
  {
    v4 = *(v2 - 2);
    v5 = *(__int64 (__fastcall **)(__int64, __int64))(*(_QWORD *)v4 + 16LL);
    if ( (unsigned __int8)sub_141FB6530(*v2) || !(unsigned __int8)sub_146ED0030(*(v2 - 20)) )
      v6 = 0;
    else
      LOBYTE(v6) = 1;
    result = v5(v4, v6);
    v2 += 28;
    --v3;
  }
  while ( v3 );
  return result;
}


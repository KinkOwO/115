// method_sub_1441DC6C0_0x1441dc6c0

__int64 __fastcall sub_1441DC6C0(_QWORD *a1)
{
  _QWORD *v2; // rbx
  __int64 v3; // rsi
  _QWORD *v4; // rdi
  __int64 result; // rax

  if ( a1[7] && (unsigned __int8)sub_146F593F0() )
    (*(void (__fastcall **)(_QWORD *))(*a1 + 112LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 80LL))(a1);
  (*(void (__fastcall **)(_QWORD *))(*a1 + 96LL))(a1);
  v2 = a1 + 19;
  v3 = 5;
  v4 = v2;
  do
  {
    result = (*(__int64 (__fastcall **)(_QWORD *))(*v2 + 8LL))(v4);
    v4 += 50;
    v2 += 50;
    --v3;
  }
  while ( v3 );
  return result;
}


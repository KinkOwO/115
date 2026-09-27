// method_sub_1441D20E0_0x1441d20e0

__int64 __fastcall sub_1441D20E0(__int64 a1)
{
  __int64 v2; // rdi
  unsigned __int64 v3; // rdx
  unsigned __int64 v4; // rdx
  __int64 v5; // rcx
  __int64 result; // rax
  _QWORD v7[3]; // [rsp+28h] [rbp-30h] BYREF
  unsigned __int64 v8; // [rsp+40h] [rbp-18h]

  v2 = *(_QWORD *)((*(__int64 (__fastcall **)(_QWORD, _QWORD *))(**(_QWORD **)(a1 + 56) + 688LL))(
                     *(_QWORD *)(a1 + 56),
                     v7)
                 + 16);
  v3 = v8;
  if ( v8 >= 8 )
  {
    v4 = 2 * v8 + 2;
    v5 = v7[0];
    if ( v4 >= 0x1000 )
    {
      v4 = 2 * v8 + 41;
      v5 = *(_QWORD *)(v7[0] - 8LL);
      if ( (unsigned __int64)(v7[0] - v5 - 8) > 0x1F )
        sub_148AAF304(v5, v4);
    }
    sub_146E9F3A0(v5, v4);
  }
  v7[2] = 0;
  v8 = 7;
  LOWORD(v7[0]) = 0;
  if ( v2 )
    *(_BYTE *)(a1 + 104) = 1;
  LOBYTE(v3) = 1;
  result = (*(__int64 (__fastcall **)(__int64, unsigned __int64))(*(_QWORD *)a1 + 32LL))(a1, v3);
  *(_BYTE *)(a1 + 104) = 0;
  return result;
}


// method_sub_1441D9070_0x1441d9070

__int64 __fastcall sub_1441D9070(_DWORD *a1)
{
  __int64 v1; // rax
  __int64 v3; // rdx

  v1 = *(_QWORD *)a1;
  a1[32] = -1;
  (*(void (**)(void))(v1 + 104))();
  (*(void (__fastcall **)(_DWORD *))(*(_QWORD *)a1 + 24LL))(a1);
  LOBYTE(v3) = 1;
  return (*(__int64 (__fastcall **)(_DWORD *, __int64))(*(_QWORD *)a1 + 32LL))(a1, v3);
}


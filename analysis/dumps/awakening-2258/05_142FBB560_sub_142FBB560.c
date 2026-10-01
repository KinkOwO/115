// sub_142FBB560  size=148

__int64 __fastcall sub_142FBB560(_QWORD *a1, __int64 a2, __int64 a3)
{
  __int64 result; // rax
  __int64 v7; // rax
  __int64 v8; // rax
  __int64 v9; // rcx

  nullsub_1(a1);
  result = (*(__int64 (__fastcall **)(_QWORD *, _QWORD))(*a1 + 808LL))(a1, *(unsigned int *)(a2 + 180));
  if ( result != 0 )
  {
    v7 = a1[20];
    if ( v7 != 0 && *(_DWORD *)(v7 + 8) != 0 )
      v8 = a1[21];
    else
      v8 = 0;
    v9 = v8 - 48;
    if ( v8 == 0 )
      v9 = 0;
    result = (*(__int64 (__fastcall **)(__int64, __int64, _QWORD, __int64))(*(_QWORD *)v9 + 3088LL))(
               v9,
               2258,
               0,
               0xFFFFFFFFLL);
    if ( (_BYTE)result != 0 )
      return sub_142FC0770(a1, a3);
  }
  return result;
}

__int64 __fastcall sub_140422B30(_QWORD *a1)
{
  __int64 result; // rax
  volatile signed __int32 *v3; // rcx
  volatile signed __int32 *v4; // rcx

  *a1 = &off_149254B60;
  result = 0;
  a1[1] = 0;
  v3 = (volatile signed __int32 *)a1[2];
  a1[2] = 0;
  if ( v3 )
  {
    result = (unsigned int)_InterlockedExchangeAdd(v3 + 3, 0xFFFFFFFF);
    if ( (_DWORD)result == 1 )
      result = (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v3 + 8LL))(v3);
  }
  v4 = (volatile signed __int32 *)a1[2];
  if ( v4 )
  {
    if ( _InterlockedExchangeAdd(v4 + 3, 0xFFFFFFFF) == 1 )
      return (*(__int64 (__fastcall **)(volatile signed __int32 *))(*(_QWORD *)v4 + 8LL))(v4);
  }
  return result;
}

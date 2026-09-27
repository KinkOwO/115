// mgr_helper_e9630 = sub_1444E9630 0x1444E9630

__int64 __fastcall sub_1444E9630(__int64 a1)
{
  __int64 v1; // rsi
  _QWORD *v2; // rdi
  _QWORD *j; // rbx
  void (__fastcall ***v4)(_QWORD, __int64); // rcx
  __int64 **v5; // rax
  __int64 i; // rax
  __int64 *v7; // rcx
  __int64 result; // rax

  v1 = a1 + 1048;
  v2 = *(_QWORD **)(a1 + 1048);
  j = (_QWORD *)*v2;
  if ( (_QWORD *)*v2 != v2 )
  {
    do
    {
      v4 = (void (__fastcall ***)(_QWORD, __int64))j[5];
      if ( v4 )
        (**v4)(v4, 1);
      v5 = (__int64 **)j[2];
      j[5] = 0;
      if ( *((_BYTE *)v5 + 25) )
      {
        for ( i = j[1]; !*(_BYTE *)(i + 25); i = *(_QWORD *)(i + 8) )
        {
          if ( j != *(_QWORD **)(i + 16) )
            break;
          j = (_QWORD *)i;
        }
        j = (_QWORD *)i;
      }
      else
      {
        v7 = *v5;
        for ( j = v5; !*((_BYTE *)v7 + 25); v7 = (__int64 *)*v7 )
          j = v7;
      }
      v2 = *(_QWORD **)v1;
    }
    while ( j != *(_QWORD **)v1 );
  }
  result = sub_14014EBD0(v1, v1, v2[1]);
  v2[1] = v2;
  *v2 = v2;
  v2[2] = v2;
  *(_QWORD *)(v1 + 8) = 0;
  return result;
}


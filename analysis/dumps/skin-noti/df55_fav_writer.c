// fav_writer

__int64 __fastcall sub_1444E8DF0(__int64 a1, int *a2)
{
  __int64 v3; // rsi
  __int64 *v4; // rdi
  __int64 result; // rax
  int v6; // edx
  __int64 *v7; // rcx

  v3 = a1 + 8;
  v4 = *(__int64 **)(a1 + 8);
  result = *v4;
  if ( (__int64 *)*v4 == v4 )
  {
LABEL_7:
    if ( *(_QWORD *)(a1 + 16) == 0x7FFFFFFFFFFFFFFLL )
      sub_14883BB54("list too long");
    result = sub_146E8BA20(32);
    *(_OWORD *)(result + 16) = *(_OWORD *)a2;
    ++*(_QWORD *)(v3 + 8);
    v7 = (__int64 *)v4[1];
    *(_QWORD *)result = v4;
    *(_QWORD *)(result + 8) = v7;
    v4[1] = result;
    *v7 = result;
  }
  else
  {
    v6 = *a2;
    while ( *(_DWORD *)(result + 16) != v6 || *(_DWORD *)(result + 20) != a2[1] || *(_DWORD *)(result + 24) != a2[2] )
    {
      result = *(_QWORD *)result;
      if ( (__int64 *)result == v4 )
        goto LABEL_7;
    }
    *(_OWORD *)(result + 16) = *(_OWORD *)a2;
  }
  return result;
}


// builder_sub_1444EAF10_0x1444eaf10

__int64 __fastcall sub_1444EAF10(__int64 a1, __int64 a2, __int64 a3)
{
  unsigned int v3; // ebx
  __int64 v5; // rax
  __int64 **v6; // rdi
  __int64 *v7; // rax
  __int64 *v8; // rcx
  __int64 result; // rax
  __int64 v10; // rax
  __int64 v11; // rbx
  _QWORD *v12; // rax
  _QWORD *v13; // rax
  int v14; // [rsp+88h] [rbp+10h] BYREF
  __int64 v15; // [rsp+98h] [rbp+20h]

  v14 = a2;
  v3 = a3;
  LOBYTE(a3) = 1;
  v5 = sub_140283D60(qword_14E683BF8, a2, a3);
  if ( !v5 || *(_DWORD *)(v5 + 8) != 2 )
    return 0;
  v6 = (__int64 **)(a1 + 1048);
  v7 = (__int64 *)(*v6)[1];
  v8 = *v6;
  while ( !*((_BYTE *)v7 + 25) )
  {
    if ( *((_DWORD *)v7 + 8) >= v14 )
    {
      v8 = v7;
      v7 = (__int64 *)*v7;
    }
    else
    {
      v7 = (__int64 *)v7[2];
    }
  }
  if ( *((_BYTE *)v8 + 25) || v14 < *((_DWORD *)v8 + 8) || v8 == *v6 || (result = v8[5]) == 0 )
  {
    if ( v3 <= 5 )
    {
      v10 = sub_146E8BA20(408);
      v15 = v10;
      if ( v10 )
        v11 = sub_1447E40D0(v10);
      else
        v11 = 0;
      *(_QWORD *)sub_140738030(v6, &v14) = v11;
      v12 = (_QWORD *)sub_140738030(v6, &v14);
      sub_1447EBED0(*v12, 0, 0, 0, 99999, 1, 100, 100, 0, v14, 0);
      v13 = (_QWORD *)sub_140738030(v6, &v14);
      sub_1447EEE10(*v13);
      return *(_QWORD *)sub_140738030(v6, &v14);
    }
    return 0;
  }
  return result;
}

